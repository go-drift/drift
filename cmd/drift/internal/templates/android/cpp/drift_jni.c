/**
 * @file drift_jni.c
 * @brief JNI bridge between Android Java/Kotlin code and the Go Drift engine.
 *
 * This file provides the native implementation for the NativeBridge Kotlin object.
 * It dynamically loads the Go shared library (libdrift.so) at runtime and resolves
 * the exported Go functions (DriftRenderFrame and DriftPointerEvent).
 *
 * Architecture:
 *
 *     Kotlin (NativeBridge.kt)
 *            │
 *            ▼ JNI call
 *     This C file (drift_jni.c)
 *            │
 *            ▼ Dynamic linking (dlopen/dlsym)
 *     Go library (libdrift.so)
 *
 * The dynamic linking approach is used because:
 *   1. Go's cgo produces a shared library that exports C-compatible symbols
 *   2. JNI requires a separate shared library that follows JNI naming conventions
 *   3. We link them together at runtime using dlopen/dlsym
 *
 * Thread Safety:
 *   The function pointer caching is not thread-safe for the initial resolution,
 *   but this is acceptable because:
 *     - The first call typically happens during app startup on the main thread
 *     - Subsequent calls just read the cached pointer (safe for concurrent reads)
 *     - The worst case is redundant resolution, which is harmless
 */

#include <jni.h>       /* JNI types and macros */
#include <stdint.h>    /* Standard integer types */
#include <string.h>    /* strdup, memcpy */
#include <dlfcn.h>     /* Dynamic linking: dlopen, dlsym, dlerror */
#include <android/log.h> /* Android logging: __android_log_print */
#include <stdlib.h>    /* malloc, free */
#include <stdio.h>     /* snprintf */

/**
 * Function pointer type for DriftRenderFrame.
 * Matches the signature exported by Go:
 *   func DriftRenderFrame(width C.int, height C.int, buffer unsafe.Pointer, bufferLen C.int) C.int
 *
 * @param width      Width of the render target in pixels
 * @param height     Height of the render target in pixels
 * @param buffer     Pointer to the RGBA buffer (4 bytes per pixel)
 * @param bufferLen  Total size of the buffer in bytes
 * @return           0 on success, non-zero on error
 */
typedef int (*DriftRenderFn)(int width, int height, void *buffer, int bufferLen);

/**
 * Function pointer type for DriftPointerEvent.
 * Matches the signature exported by Go:
 *   func DriftPointerEvent(pointerID C.int64_t, phase C.int, x C.double, y C.double)
 *
 * @param pointerID  Unique identifier for this pointer/touch (enables multi-touch)
 * @param phase      Touch phase: 0=Down, 1=Move, 2=Up, 3=Cancel
 * @param x          X coordinate in pixels
 * @param y          Y coordinate in pixels
 */
typedef void (*DriftPointerFn)(int64_t pointerID, int phase, double x, double y);

/**
 * Function pointer type for DriftSetDeviceScale.
 * Matches the signature exported by Go:
 *   func DriftSetDeviceScale(scale C.double)
 *
 * @param scale Device pixel scale factor (e.g., 2.0 or 3.0 on high-DPI screens)
 */
typedef void (*DriftSetScaleFn)(double scale);

/**
 * Function pointer type for DriftPlatformHandleEvent.
 * Matches the signature exported by Go.
 */
typedef void (*DriftPlatformHandleEventFn)(const char *channel, const void *data, int dataLen);

/**
 * Function pointer type for DriftPlatformHandleEventError.
 * Matches the signature exported by Go.
 */
typedef void (*DriftPlatformHandleEventErrorFn)(const char *channel, const char *code, const char *message);

/**
 * Function pointer type for DriftPlatformHandleEventDone.
 * Matches the signature exported by Go.
 */
typedef void (*DriftPlatformHandleEventDoneFn)(const char *channel);

/**
 * Function pointer type for DriftPlatformIsStreamActive.
 * Matches the signature exported by Go.
 */
typedef int (*DriftPlatformIsStreamActiveFn)(const char *channel);

/**
 * Function pointer type for DriftPlatformSetNativeHandler.
 * Used to register a callback that Go can use to invoke native methods.
 */
typedef void (*DriftPlatformSetNativeHandlerFn)(void *handler);

/**
 * Native method handler signature that Go expects.
 */
typedef int (*DriftNativeMethodHandler)(
    const char *channel,
    const char *method,
    const void *argsData,
    int argsLen,
    void **resultData,
    int *resultLen,
    char **errorMsg
);

/**
 * Function pointer type for DriftSkiaInitGL.
 * Matches the signature exported by Go:
 *   func DriftSkiaInitGL() C.int
 *
 * @return 0 on success, non-zero on failure
 */
typedef int (*DriftSkiaInitFn)(void);

typedef int (*DriftAppInitFn)(void);

/**
 * Function pointer type for DriftSkiaRenderGL.
 * Matches the signature exported by Go:
 *   func DriftSkiaRenderGL(width C.int, height C.int) C.int
 *
 * @param width  Width of the render target in pixels
 * @param height Height of the render target in pixels
 * @return 0 on success, non-zero on failure
 */
typedef int (*DriftSkiaRenderFn)(int width, int height);
typedef const char* (*DriftSkiaErrorFn)(void);

/**
 * Function pointer type for DriftBackButtonPressed.
 * Matches the signature exported by Go:
 *   func DriftBackButtonPressed() C.int
 *
 * @return 1 if back was handled (route popped), 0 if not handled (at root)
 */
typedef int (*DriftBackButtonFn)(void);

typedef void (*DriftRequestFrameFn)(void);

/* Cached function pointers. NULL until resolved. */
static DriftRenderFn drift_render_frame = NULL;
static DriftPointerFn drift_pointer_event = NULL;
static DriftSetScaleFn drift_set_scale = NULL;
static DriftAppInitFn drift_app_init = NULL;
static DriftSkiaInitFn drift_skia_init = NULL;
static DriftSkiaRenderFn drift_skia_render = NULL;
static DriftSkiaErrorFn drift_skia_error = NULL;
static DriftPlatformHandleEventFn drift_platform_event = NULL;
static DriftPlatformHandleEventErrorFn drift_platform_event_error = NULL;
static DriftPlatformHandleEventDoneFn drift_platform_event_done = NULL;
static DriftPlatformIsStreamActiveFn drift_platform_stream_active = NULL;
static DriftPlatformSetNativeHandlerFn drift_platform_set_handler = NULL;
static DriftBackButtonFn drift_back_button = NULL;
static DriftRequestFrameFn drift_request_frame = NULL;

/* Handle to the loaded Go shared library. NULL until loaded. */
static void *drift_handle = NULL;

/* Global JVM reference for callbacks from Go to Kotlin */
static JavaVM *g_jvm = NULL;
static jclass g_platform_channel_class = NULL;
static jmethodID g_handle_method_call = NULL;
static jmethodID g_consume_last_error = NULL;
static int g_native_handler_registered = 0;

static char *json_error(const char *code, const char *message) {
    const char *safe_code = code ? code : "native_error";
    const char *safe_message = message ? message : "";
    size_t len = (size_t)snprintf(NULL, 0, "{\"code\":\"%s\",\"message\":\"%s\"}", safe_code, safe_message);
    char *buffer = (char *)malloc(len + 1);
    if (!buffer) {
        return NULL;
    }
    snprintf(buffer, len + 1, "{\"code\":\"%s\",\"message\":\"%s\"}", safe_code, safe_message);
    return buffer;
}

/**
 * Native method handler called by Go to invoke Kotlin methods.
 * This is the C callback that bridges Go -> Kotlin.
 */
static int native_method_handler(
    const char *channel,
    const char *method,
    const void *argsData,
    int argsLen,
    void **resultData,
    int *resultLen,
    char **errorMsg
) {
    if (!g_jvm || !g_platform_channel_class || !g_handle_method_call) {
        if (errorMsg) {
            char *payload = json_error("jni_error", "JNI not initialized");
            *errorMsg = payload ? payload : strdup("JNI not initialized");
        }
        return -1;
    }

    JNIEnv *env = NULL;
    int needs_detach = 0;

    /* Get JNIEnv for current thread */
    jint result = (*g_jvm)->GetEnv(g_jvm, (void **)&env, JNI_VERSION_1_6);
    if (result == JNI_EDETACHED) {
        if ((*g_jvm)->AttachCurrentThread(g_jvm, &env, NULL) != 0) {
            if (errorMsg) {
                char *payload = json_error("jni_error", "Failed to attach thread");
                *errorMsg = payload ? payload : strdup("Failed to attach thread");
            }
            return -1;
        }
        needs_detach = 1;
    } else if (result != JNI_OK) {
        if (errorMsg) {
            char *payload = json_error("jni_error", "Failed to get JNI env");
            *errorMsg = payload ? payload : strdup("Failed to get JNI env");
        }
        return -1;
    }

    /* Create Java strings */
    jstring jchannel = (*env)->NewStringUTF(env, channel);
    jstring jmethod = (*env)->NewStringUTF(env, method);

    /* Create byte array for args */
    jbyteArray jargsData = NULL;
    if (argsData && argsLen > 0) {
        jargsData = (*env)->NewByteArray(env, argsLen);
        (*env)->SetByteArrayRegion(env, jargsData, 0, argsLen, (const jbyte *)argsData);
    }

    /* Call Kotlin: PlatformChannelManager.handleMethodCallNative(channel, method, argsData) */
    jobject jresult = (*env)->CallStaticObjectMethod(
        env, g_platform_channel_class, g_handle_method_call,
        jchannel, jmethod, jargsData
    );

    int ret = 0;

    /* Check for exceptions */
    if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        if (errorMsg) {
            char *payload = json_error("kotlin_exception", "Kotlin exception");
            *errorMsg = payload ? payload : strdup("Kotlin exception");
        }
        ret = -1;
    } else if (jresult != NULL) {
        /* Extract result byte array */
        jsize len = (*env)->GetArrayLength(env, (jbyteArray)jresult);
        if (len > 0) {
            *resultLen = len;
            *resultData = malloc(len);
            (*env)->GetByteArrayRegion(env, (jbyteArray)jresult, 0, len, (jbyte *)*resultData);
        }
        (*env)->DeleteLocalRef(env, jresult);
    } else if (g_consume_last_error) {
        jstring jerror = (jstring)(*env)->CallStaticObjectMethod(env, g_platform_channel_class, g_consume_last_error);
        if ((*env)->ExceptionCheck(env)) {
            (*env)->ExceptionClear(env);
            if (errorMsg) {
                char *payload = json_error("kotlin_exception", "Kotlin exception");
                *errorMsg = payload ? payload : strdup("Kotlin exception");
            }
            ret = -1;
        } else if (jerror != NULL) {
            const char *errStr = (*env)->GetStringUTFChars(env, jerror, NULL);
            if (errStr && errorMsg) {
                *errorMsg = strdup(errStr);
                ret = -1;
            }
            if (errStr) {
                (*env)->ReleaseStringUTFChars(env, jerror, errStr);
            }
            (*env)->DeleteLocalRef(env, jerror);
        }
    }

    /* Cleanup */
    if (jargsData) (*env)->DeleteLocalRef(env, jargsData);
    (*env)->DeleteLocalRef(env, jmethod);
    (*env)->DeleteLocalRef(env, jchannel);

    if (needs_detach) {
        (*g_jvm)->DetachCurrentThread(g_jvm);
    }

    return ret;
}

/**
 * Resolves the DriftPlatformSetNativeHandler function and registers our handler.
 */
static int resolve_and_register_native_handler(void) {
    if (g_native_handler_registered) {
        return 0;
    }

    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen failed: %s", dlerror());
            return -1;
        }
    }

    if (!drift_platform_set_handler) {
        drift_platform_set_handler = (DriftPlatformSetNativeHandlerFn)dlsym(
            drift_handle, "DriftPlatformSetNativeHandler"
        );
        if (!drift_platform_set_handler) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "DriftPlatformSetNativeHandler not found");
            return -1;
        }
    }

    /* Register our native method handler with Go */
    drift_platform_set_handler((void *)native_method_handler);
    g_native_handler_registered = 1;
    __android_log_print(ANDROID_LOG_INFO, "DriftJNI", "Native method handler registered");

    return 0;
}

/**
 * Resolves the DriftRenderFrame function from the Go shared library.
 *
 * This function uses lazy loading: the library is loaded on first call,
 * and the function pointer is cached for subsequent calls.
 *
 * Loading Strategy:
 *   1. First try dlopen("libdrift.so") with RTLD_NOW | RTLD_GLOBAL
 *      - RTLD_NOW: Resolve all symbols immediately (fail fast if missing)
 *      - RTLD_GLOBAL: Make symbols available for other libraries
 *   2. If that succeeds, use dlsym on that handle
 *   3. If dlopen fails, try RTLD_DEFAULT (search already-loaded libraries)
 *      This can work if the library was loaded differently
 *
 * @return 0 if the function was successfully resolved, 1 on failure.
 */
static int resolve_drift_render(void) {
    /* Return immediately if already resolved */
    if (drift_render_frame) {
        return 0;
    }

    /* Try to load the Go shared library if not already loaded */
    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            /* Log the error from dlopen for debugging */
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libdrift.so failed: %s", dlerror());
        }
    }

    /* Look up the DriftRenderFrame symbol */
    if (drift_handle) {
        /* Library loaded successfully, look up in that library */
        drift_render_frame = (DriftRenderFn)dlsym(drift_handle, "DriftRenderFrame");
    } else {
        /* Fallback: search in already-loaded libraries */
        drift_render_frame = (DriftRenderFn)dlsym(RTLD_DEFAULT, "DriftRenderFrame");
    }

    /* Log if the symbol wasn't found */
    if (!drift_render_frame) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "DriftRenderFrame not found: %s", dlerror());
    }

    return drift_render_frame ? 0 : 1;
}

/**
 * Resolves the DriftPointerEvent function from the Go shared library.
 *
 * Uses the same lazy loading strategy as resolve_drift_render().
 * See that function's documentation for details on the loading strategy.
 *
 * @return 0 if the function was successfully resolved, 1 on failure.
 */
static int resolve_drift_pointer(void) {
    /* Return immediately if already resolved */
    if (drift_pointer_event) {
        return 0;
    }

    /* Try to load the Go shared library if not already loaded */
    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            /* Log the error from dlopen for debugging */
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libdrift.so failed: %s", dlerror());
        }
    }

    /* Look up the DriftPointerEvent symbol */
    if (drift_handle) {
        /* Library loaded successfully, look up in that library */
        drift_pointer_event = (DriftPointerFn)dlsym(drift_handle, "DriftPointerEvent");
    } else {
        /* Fallback: search in already-loaded libraries */
        drift_pointer_event = (DriftPointerFn)dlsym(RTLD_DEFAULT, "DriftPointerEvent");
    }

    /* Log if the symbol wasn't found */
    if (!drift_pointer_event) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "DriftPointerEvent not found: %s", dlerror());
    }

    return drift_pointer_event ? 0 : 1;
}

/**
 * Resolves the DriftSetDeviceScale function from the Go shared library.
 *
 * Uses the same lazy loading strategy as resolve_drift_render().
 *
 * @return 0 if the function was successfully resolved, 1 on failure.
 */
static int resolve_drift_scale(void) {
    /* Return immediately if already resolved */
    if (drift_set_scale) {
        return 0;
    }

    /* Try to load the Go shared library if not already loaded */
    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libdrift.so failed: %s", dlerror());
        }
    }

    /* Look up the DriftSetDeviceScale symbol */
    if (drift_handle) {
        drift_set_scale = (DriftSetScaleFn)dlsym(drift_handle, "DriftSetDeviceScale");
    } else {
        drift_set_scale = (DriftSetScaleFn)dlsym(RTLD_DEFAULT, "DriftSetDeviceScale");
    }

    /* Log if the symbol wasn't found */
    if (!drift_set_scale) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "DriftSetDeviceScale not found: %s", dlerror());
    }

    return drift_set_scale ? 0 : 1;
}

/**
 * Resolves the DriftAppInit function from the Go shared library.
 *
 * @return 0 if the function was successfully resolved, 1 on failure.
 */
static int resolve_drift_app_init(void) {
    if (drift_app_init) {
        return 0;
    }

    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libdrift.so failed: %s", dlerror());
        }
    }

    if (drift_handle) {
        drift_app_init = (DriftAppInitFn)dlsym(drift_handle, "DriftAppInit");
    } else {
        drift_app_init = (DriftAppInitFn)dlsym(RTLD_DEFAULT, "DriftAppInit");
    }

    if (!drift_app_init) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "DriftAppInit not found: %s", dlerror());
    }

    return drift_app_init ? 0 : 1;
}

/**
 * Resolves the DriftSkiaInitGL function from the Go shared library.
 *
 * @return 0 if the function was successfully resolved, 1 on failure.
 */
static int resolve_drift_skia_init(void) {
    if (drift_skia_init) {
        return 0;
    }

    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libdrift.so failed: %s", dlerror());
        }
    }

    if (drift_handle) {
        drift_skia_init = (DriftSkiaInitFn)dlsym(drift_handle, "DriftSkiaInitGL");
    } else {
        drift_skia_init = (DriftSkiaInitFn)dlsym(RTLD_DEFAULT, "DriftSkiaInitGL");
    }

    if (!drift_skia_init) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "DriftSkiaInitGL not found: %s", dlerror());
    }

    return drift_skia_init ? 0 : 1;
}

/**
 * Resolves the DriftSkiaRenderGL function from the Go shared library.
 *
 * @return 0 if the function was successfully resolved, 1 on failure.
 */
static int resolve_drift_skia_render(void) {
    if (drift_skia_render) {
        return 0;
    }

    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libdrift.so failed: %s", dlerror());
        }
    }

    if (drift_handle) {
        drift_skia_render = (DriftSkiaRenderFn)dlsym(drift_handle, "DriftSkiaRenderGL");
    } else {
        drift_skia_render = (DriftSkiaRenderFn)dlsym(RTLD_DEFAULT, "DriftSkiaRenderGL");
    }

    if (!drift_skia_render) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "DriftSkiaRenderGL not found: %s", dlerror());
    }

    return drift_skia_render ? 0 : 1;
}

static int resolve_drift_skia_error(void) {
    if (drift_skia_error) {
        return 0;
    }

    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libdrift.so failed: %s", dlerror());
        }
    }

    if (drift_handle) {
        drift_skia_error = (DriftSkiaErrorFn)dlsym(drift_handle, "DriftSkiaLastError");
    } else {
        drift_skia_error = (DriftSkiaErrorFn)dlsym(RTLD_DEFAULT, "DriftSkiaLastError");
    }

    if (!drift_skia_error) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "DriftSkiaLastError not found: %s", dlerror());
    }

    return drift_skia_error ? 0 : 1;
}

/**
 * JNI implementation for NativeBridge.renderFrame().
 *
 * Called from DriftRenderer.onDrawFrame() each frame to render the scene.
 * This function:
 *   1. Resolves the Go function if not already cached
 *   2. Extracts the direct buffer address from the Java ByteBuffer
 *   3. Calls the Go render function to fill the buffer with RGBA pixels
 *
 * @param env       JNI environment pointer (provides JNI functions)
 * @param clazz     Reference to the NativeBridge class (unused, static method)
 * @param width     Width of the render target in pixels
 * @param height    Height of the render target in pixels
 * @param buffer    Direct ByteBuffer allocated by Java (must be direct!)
 * @param bufferLen Size of the buffer in bytes (should be width * height * 4)
 * @return          0 on success, 1 on failure
 *
 * Note: The buffer MUST be a direct ByteBuffer (allocated with allocateDirect).
 *       Regular heap ByteBuffers do not have a stable native address.
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_renderFrame(
    JNIEnv *env,
    jclass clazz,
    jint width,
    jint height,
    jobject buffer,
    jint bufferLen
) {
    /* Ensure the Go render function is available */
    if (resolve_drift_render() != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Failed to resolve DriftRenderFrame");
        return 1;
    }

    /*
     * Get the native pointer from the direct ByteBuffer.
     * This only works for direct buffers (allocateDirect).
     * For heap buffers, this returns NULL.
     */
    void *ptr = (*env)->GetDirectBufferAddress(env, buffer);
    if (!ptr) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Direct buffer address is null");
        return 1;
    }

    /* Call the Go render function to fill the buffer with pixels */
    int result = drift_render_frame(width, height, ptr, bufferLen);

    /* Log any render failures for debugging */
    if (result != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Render failed width=%d height=%d len=%d", width, height, bufferLen);
    }

    return (jint)result;
}

/**
 * JNI implementation for NativeBridge.appInit().
 *
 * Calls the Go application entrypoint once.
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_appInit(
    JNIEnv *env,
    jclass clazz
) {
    (void)env;
    (void)clazz;

    if (resolve_drift_app_init() != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Failed to resolve DriftAppInit");
        return 1;
    }

    return (jint)drift_app_init();
}

/**
 * JNI implementation for NativeBridge.initSkiaGL().
 *
 * Initializes the Skia GL context using the current GL context.
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_initSkiaGL(
    JNIEnv *env,
    jclass clazz
) {
    (void)env;
    (void)clazz;

    if (resolve_drift_skia_init() != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Failed to resolve DriftSkiaInitGL");
        return 1;
    }

    return (jint)drift_skia_init();
}

/**
 * JNI implementation for NativeBridge.renderFrameSkia().
 *
 * Renders a frame directly to the current framebuffer using Skia.
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_renderFrameSkia(
    JNIEnv *env,
    jclass clazz,
    jint width,
    jint height
) {
    (void)env;
    (void)clazz;

    if (resolve_drift_skia_render() != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Failed to resolve DriftSkiaRenderGL");
        return 1;
    }

    int result = drift_skia_render(width, height);
    if (result != 0 && resolve_drift_skia_error() == 0) {
        const char *err = drift_skia_error();
        if (err && err[0]) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Skia render error: %s", err);
        }
        if (err) {
            free((void *)err);
        }
    }
    return (jint)result;
}

/**
 * JNI implementation for NativeBridge.pointerEvent().
 *
 * Called from DriftSurfaceView.onTouchEvent() when the user touches the screen.
 * This function forwards touch events to the Go engine for processing.
 *
 * @param env       JNI environment pointer (provides JNI functions)
 * @param clazz     Reference to the NativeBridge class (unused, static method)
 * @param pointerID Unique identifier for this pointer/touch (from MotionEvent.getPointerId())
 * @param phase     Touch phase: 0=Down, 1=Move, 2=Up, 3=Cancel
 *                  Maps from Android MotionEvent actions in DriftSurfaceView
 * @param x         X coordinate of the touch in pixels (from MotionEvent.getX())
 * @param y         Y coordinate of the touch in pixels (from MotionEvent.getY())
 *
 * Note: Coordinates are in view pixels, not density-independent pixels (dp).
 *       The Go engine works in raw pixels, matching the render buffer dimensions.
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_pointerEvent(
    JNIEnv *env,
    jclass clazz,
    jlong pointerID,
    jint phase,
    jdouble x,
    jdouble y
) {
    /* Ensure the Go pointer function is available */
    if (resolve_drift_pointer() != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Failed to resolve DriftPointerEvent");
        return;
    }

    /* Forward the event to the Go engine */
    drift_pointer_event((int64_t)pointerID, phase, x, y);
}

/**
 * JNI implementation for NativeBridge.setDeviceScale().
 *
 * Called when the view is created or configuration changes, ensuring the
 * Go engine uses the correct scale factor for logical sizing.
 *
 * @param env    JNI environment pointer (provides JNI functions)
 * @param clazz  Reference to the NativeBridge class (unused, static method)
 * @param scale  Device scale factor from Android DisplayMetrics.density
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_setDeviceScale(
    JNIEnv *env,
    jclass clazz,
    jdouble scale
) {
    /* Ensure the Go scale function is available */
    if (resolve_drift_scale() != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Failed to resolve DriftSetDeviceScale");
        return;
    }

    /* Forward the scale to the Go engine */
    drift_set_scale(scale);
}

/**
 * Resolves the DriftPlatformHandleEvent function from the Go shared library.
 *
 * @return 0 if the function was successfully resolved, 1 on failure.
 */
static int resolve_drift_platform_event(void) {
    if (drift_platform_event) {
        return 0;
    }

    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libdrift.so failed: %s", dlerror());
        }
    }

    if (drift_handle) {
        drift_platform_event = (DriftPlatformHandleEventFn)dlsym(drift_handle, "DriftPlatformHandleEvent");
    } else {
        drift_platform_event = (DriftPlatformHandleEventFn)dlsym(RTLD_DEFAULT, "DriftPlatformHandleEvent");
    }

    if (!drift_platform_event) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "DriftPlatformHandleEvent not found: %s", dlerror());
    }

    return drift_platform_event ? 0 : 1;
}

/**
 * Resolves the DriftPlatformHandleEventError function from the Go shared library.
 */
static int resolve_drift_platform_event_error(void) {
    if (drift_platform_event_error) {
        return 0;
    }

    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
    }

    if (drift_handle) {
        drift_platform_event_error = (DriftPlatformHandleEventErrorFn)dlsym(drift_handle, "DriftPlatformHandleEventError");
    } else {
        drift_platform_event_error = (DriftPlatformHandleEventErrorFn)dlsym(RTLD_DEFAULT, "DriftPlatformHandleEventError");
    }

    return drift_platform_event_error ? 0 : 1;
}

/**
 * Resolves the DriftPlatformHandleEventDone function from the Go shared library.
 */
static int resolve_drift_platform_event_done(void) {
    if (drift_platform_event_done) {
        return 0;
    }

    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
    }

    if (drift_handle) {
        drift_platform_event_done = (DriftPlatformHandleEventDoneFn)dlsym(drift_handle, "DriftPlatformHandleEventDone");
    } else {
        drift_platform_event_done = (DriftPlatformHandleEventDoneFn)dlsym(RTLD_DEFAULT, "DriftPlatformHandleEventDone");
    }

    return drift_platform_event_done ? 0 : 1;
}

/**
 * Resolves the DriftPlatformIsStreamActive function from the Go shared library.
 */
static int resolve_drift_platform_stream_active(void) {
    if (drift_platform_stream_active) {
        return 0;
    }

    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
    }

    if (drift_handle) {
        drift_platform_stream_active = (DriftPlatformIsStreamActiveFn)dlsym(drift_handle, "DriftPlatformIsStreamActive");
    } else {
        drift_platform_stream_active = (DriftPlatformIsStreamActiveFn)dlsym(RTLD_DEFAULT, "DriftPlatformIsStreamActive");
    }

    return drift_platform_stream_active ? 0 : 1;
}

/**
 * Resolves the DriftBackButtonPressed function from the Go shared library.
 *
 * @return 0 if the function was successfully resolved, 1 on failure.
 */
static int resolve_drift_back_button(void) {
    if (drift_back_button) {
        return 0;
    }

    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libdrift.so failed: %s", dlerror());
        }
    }

    if (drift_handle) {
        drift_back_button = (DriftBackButtonFn)dlsym(drift_handle, "DriftBackButtonPressed");
    } else {
        drift_back_button = (DriftBackButtonFn)dlsym(RTLD_DEFAULT, "DriftBackButtonPressed");
    }

    if (!drift_back_button) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "DriftBackButtonPressed not found: %s", dlerror());
    }

    return drift_back_button ? 0 : 1;
}

/**
 * Resolves the DriftRequestFrame function from the Go shared library.
 *
 * @return 0 if the function was successfully resolved, 1 on failure.
 */
static int resolve_drift_request_frame(void) {
    if (drift_request_frame) {
        return 0;
    }

    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libdrift.so failed: %s", dlerror());
        }
    }

    if (drift_handle) {
        drift_request_frame = (DriftRequestFrameFn)dlsym(drift_handle, "DriftRequestFrame");
    } else {
        drift_request_frame = (DriftRequestFrameFn)dlsym(RTLD_DEFAULT, "DriftRequestFrame");
    }

    if (!drift_request_frame) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "DriftRequestFrame not found: %s", dlerror());
    }

    return drift_request_frame ? 0 : 1;
}

/**
 * JNI implementation for NativeBridge.platformHandleEvent().
 *
 * Sends an event to Go event listeners.
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_platformHandleEvent(
    JNIEnv *env,
    jclass clazz,
    jstring channel,
    jbyteArray data,
    jint dataLen
) {
    (void)clazz;

    if (resolve_drift_platform_event() != 0) {
        return;
    }

    const char *channelStr = (*env)->GetStringUTFChars(env, channel, NULL);
    if (!channelStr) return;

    jbyte *dataBytes = NULL;
    if (data != NULL && dataLen > 0) {
        dataBytes = (*env)->GetByteArrayElements(env, data, NULL);
    }

    drift_platform_event(channelStr, dataBytes, dataLen);

    if (dataBytes) {
        (*env)->ReleaseByteArrayElements(env, data, dataBytes, JNI_ABORT);
    }
    (*env)->ReleaseStringUTFChars(env, channel, channelStr);
}

/**
 * JNI implementation for NativeBridge.platformHandleEventError().
 *
 * Sends an error to Go event listeners.
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_platformHandleEventError(
    JNIEnv *env,
    jclass clazz,
    jstring channel,
    jstring code,
    jstring message
) {
    (void)clazz;

    if (resolve_drift_platform_event_error() != 0) {
        return;
    }

    const char *channelStr = (*env)->GetStringUTFChars(env, channel, NULL);
    const char *codeStr = (*env)->GetStringUTFChars(env, code, NULL);
    const char *messageStr = (*env)->GetStringUTFChars(env, message, NULL);

    if (channelStr && codeStr && messageStr) {
        drift_platform_event_error(channelStr, codeStr, messageStr);
    }

    if (messageStr) (*env)->ReleaseStringUTFChars(env, message, messageStr);
    if (codeStr) (*env)->ReleaseStringUTFChars(env, code, codeStr);
    if (channelStr) (*env)->ReleaseStringUTFChars(env, channel, channelStr);
}

/**
 * JNI implementation for NativeBridge.platformHandleEventDone().
 *
 * Notifies Go that an event stream has ended.
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_platformHandleEventDone(
    JNIEnv *env,
    jclass clazz,
    jstring channel
) {
    (void)clazz;

    if (resolve_drift_platform_event_done() != 0) {
        return;
    }

    const char *channelStr = (*env)->GetStringUTFChars(env, channel, NULL);
    if (channelStr) {
        drift_platform_event_done(channelStr);
        (*env)->ReleaseStringUTFChars(env, channel, channelStr);
    }
}

/**
 * JNI implementation for NativeBridge.platformIsStreamActive().
 *
 * Checks if Go is listening to events on the given channel.
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_platformIsStreamActive(
    JNIEnv *env,
    jclass clazz,
    jstring channel
) {
    (void)clazz;

    if (resolve_drift_platform_stream_active() != 0) {
        return 0;
    }

    const char *channelStr = (*env)->GetStringUTFChars(env, channel, NULL);
    if (!channelStr) return 0;

    int result = drift_platform_stream_active(channelStr);
    (*env)->ReleaseStringUTFChars(env, channel, channelStr);

    return (jint)result;
}

/**
 * JNI implementation for NativeBridge.backButtonPressed().
 *
 * Called from MainActivity when the Android back button is pressed.
 * Returns 1 if the Go engine handled the back (popped a route),
 * 0 if not handled (at root route, app should exit).
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_backButtonPressed(
    JNIEnv *env,
    jclass clazz
) {
    (void)env;
    (void)clazz;

    if (resolve_drift_back_button() != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Failed to resolve DriftBackButtonPressed");
        return 0;
    }

    return (jint)drift_back_button();
}

/**
 * JNI implementation for NativeBridge.requestFrame().
 *
 * Signals the Go engine to mark the current frame dirty.
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_requestFrame(
    JNIEnv *env,
    jclass clazz
) {
    (void)env;
    (void)clazz;

    if (resolve_drift_request_frame() != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Failed to resolve DriftRequestFrame");
        return;
    }

    drift_request_frame();
}

/**
 * JNI_OnLoad is called when the native library is loaded.
 * We save the JavaVM reference for later use in callbacks.
 */
JNIEXPORT jint JNICALL JNI_OnLoad(JavaVM *vm, void *reserved) {
    (void)reserved;
    g_jvm = vm;
    __android_log_print(ANDROID_LOG_INFO, "DriftJNI", "JNI_OnLoad: JVM saved");
    return JNI_VERSION_1_6;
}

/**
 * JNI implementation for NativeBridge.platformInit().
 *
 * Initializes platform channels by finding the Kotlin handler method
 * and registering our native callback with Go.
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_platformInit(
    JNIEnv *env,
    jclass clazz
) {
    (void)clazz;

    /* Find the PlatformChannelManager class and method */
    jclass localClass = (*env)->FindClass(env, "{{.PackagePath}}/PlatformChannelManager");
    if (!localClass) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "PlatformChannelManager class not found");
        return -1;
    }

    /* Create a global reference so it survives across JNI calls */
    g_platform_channel_class = (*env)->NewGlobalRef(env, localClass);
    (*env)->DeleteLocalRef(env, localClass);

    /* Find the static method: handleMethodCallNative(String, String, ByteArray) -> ByteArray */
    g_handle_method_call = (*env)->GetStaticMethodID(
        env, g_platform_channel_class,
        "handleMethodCallNative",
        "(Ljava/lang/String;Ljava/lang/String;[B)[B"
    );

    if (!g_handle_method_call) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "handleMethodCallNative method not found");
        return -1;
    }

    /* Find the static method: consumeLastError() -> String */
    g_consume_last_error = (*env)->GetStaticMethodID(
        env, g_platform_channel_class,
        "consumeLastError",
        "()Ljava/lang/String;"
    );

    if (!g_consume_last_error) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "consumeLastError method not found");
        return -1;
    }

    /* Register our native handler with Go */
    if (resolve_and_register_native_handler() != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Failed to register native handler");
        return -1;
    }

    __android_log_print(ANDROID_LOG_INFO, "DriftJNI", "Platform channels initialized");
    return 0;
}
