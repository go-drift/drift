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

/* AHardwareBuffer / EGL for synchronized rendering */
#include <android/hardware_buffer.h>
#include <android/native_window.h>
#include <android/native_window_jni.h>
#include <EGL/egl.h>
#include <EGL/eglext.h>
#include <GLES2/gl2.h>
#include <GLES2/gl2ext.h>
#include <unistd.h>    /* close() for fence FDs */

/*
 * SurfaceControl types and function pointers.
 *
 * The NDK header <android/surface_control.h> uses C++ syntax (default arguments,
 * references) and cannot be included from a C file. Instead, we forward-declare
 * the opaque types and resolve functions at runtime via dlsym from libandroid.so.
 * All functions used here are available from API 29.
 */
typedef struct ASurfaceControl ASurfaceControl;
typedef struct ASurfaceTransaction ASurfaceTransaction;

typedef ASurfaceControl* (*pf_ASurfaceControl_createFromWindow)(ANativeWindow*, const char*);
typedef void (*pf_ASurfaceControl_release)(ASurfaceControl*);
typedef ASurfaceTransaction* (*pf_ASurfaceTransaction_create)(void);
typedef void (*pf_ASurfaceTransaction_delete)(ASurfaceTransaction*);
typedef void (*pf_ASurfaceTransaction_setBuffer)(ASurfaceTransaction*, ASurfaceControl*, AHardwareBuffer*, int);
typedef void (*pf_ASurfaceTransaction_setVisibility)(ASurfaceTransaction*, ASurfaceControl*, int8_t);
typedef void (*pf_ASurfaceTransaction_apply)(ASurfaceTransaction*);

static pf_ASurfaceControl_createFromWindow sc_createFromWindow = NULL;
static pf_ASurfaceControl_release sc_release = NULL;
static pf_ASurfaceTransaction_create sc_createTransaction = NULL;
static pf_ASurfaceTransaction_delete sc_deleteTransaction = NULL;
static pf_ASurfaceTransaction_setBuffer sc_setBuffer = NULL;
static pf_ASurfaceTransaction_setVisibility sc_setVisibility = NULL;
static pf_ASurfaceTransaction_apply sc_apply = NULL;

static int resolve_surface_control(void) {
    if (sc_createFromWindow) return 0; /* already resolved */

    void *lib = dlopen("libandroid.so", RTLD_NOW);
    if (!lib) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libandroid.so failed: %s", dlerror());
        return -1;
    }

    sc_createFromWindow = (pf_ASurfaceControl_createFromWindow)dlsym(lib, "ASurfaceControl_createFromWindow");
    sc_release = (pf_ASurfaceControl_release)dlsym(lib, "ASurfaceControl_release");
    sc_createTransaction = (pf_ASurfaceTransaction_create)dlsym(lib, "ASurfaceTransaction_create");
    sc_deleteTransaction = (pf_ASurfaceTransaction_delete)dlsym(lib, "ASurfaceTransaction_delete");
    sc_setBuffer = (pf_ASurfaceTransaction_setBuffer)dlsym(lib, "ASurfaceTransaction_setBuffer");
    sc_setVisibility = (pf_ASurfaceTransaction_setVisibility)dlsym(lib, "ASurfaceTransaction_setVisibility");
    sc_apply = (pf_ASurfaceTransaction_apply)dlsym(lib, "ASurfaceTransaction_apply");

    if (!sc_createFromWindow || !sc_release || !sc_createTransaction ||
        !sc_deleteTransaction || !sc_setBuffer || !sc_setVisibility || !sc_apply) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "SurfaceControl NDK functions unavailable");
        sc_createFromWindow = NULL; /* mark as unresolved */
        return -1;
    }

    /* Keep lib open (functions remain valid) */
    return 0;
}

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
typedef int (*DriftNeedsFrameFn)(void);
typedef uint64_t (*DriftPlatformCurrentFrameSeqFn)(void);
typedef int (*DriftPlatformGeometryPendingFn)(void);

/**
 * Function pointer type for DriftSetScheduleFrameHandler.
 * Registers a C callback that Go invokes when it needs a new frame.
 */
typedef void (*DriftSetScheduleFrameHandlerFn)(void (*handler)(void));

/**
 * Function pointer type for DriftHitTestPlatformView.
 * Matches the signature exported by Go:
 *   func DriftHitTestPlatformView(viewID C.int64_t, x C.double, y C.double) C.int
 *
 * @param viewID Platform view ID to check
 * @param x      X coordinate in pixels
 * @param y      Y coordinate in pixels
 * @return 1 if topmost (allow touch), 0 if obscured (block touch)
 */
typedef int (*DriftHitTestPlatformViewFn)(int64_t viewID, double x, double y);

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
static DriftNeedsFrameFn drift_needs_frame = NULL;
static int drift_needs_frame_resolved = 0;
static DriftPlatformCurrentFrameSeqFn drift_platform_current_frame_seq = NULL;
static DriftPlatformGeometryPendingFn drift_platform_geometry_pending = NULL;
static DriftHitTestPlatformViewFn drift_hit_test_platform_view = NULL;
static int drift_hit_test_platform_view_resolved = 0;
static DriftSetScheduleFrameHandlerFn drift_set_schedule_frame_handler = NULL;

/* Handle to the loaded Go shared library. NULL until loaded. */
static void *drift_handle = NULL;

/* Global JVM reference for callbacks from Go to Kotlin */
static JavaVM *g_jvm = NULL;
static jclass g_platform_channel_class = NULL;
static jmethodID g_handle_method_call = NULL;
static jmethodID g_consume_last_error = NULL;
static jmethodID g_native_schedule_frame = NULL;
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
 * Schedule-frame callback invoked by Go when it needs a new frame.
 * Attaches to the JVM, then calls PlatformChannelManager.nativeScheduleFrame()
 * which posts a one-shot Choreographer callback on the main thread.
 *
 * Attach/detach cost is acceptable here: this fires once per state change
 * (user tap, Dispatch callback), not per frame. Animation continuity is
 * handled by DriftRenderer's post-render NeedsFrame() check on the
 * already-attached GL thread.
 */
static void schedule_frame_handler(void) {
    if (!g_jvm || !g_platform_channel_class || !g_native_schedule_frame) {
        return;
    }

    JNIEnv *env = NULL;
    int needs_detach = 0;

    jint result = (*g_jvm)->GetEnv(g_jvm, (void **)&env, JNI_VERSION_1_6);
    if (result == JNI_EDETACHED) {
        if ((*g_jvm)->AttachCurrentThread(g_jvm, &env, NULL) != 0) {
            return;
        }
        needs_detach = 1;
    } else if (result != JNI_OK) {
        return;
    }

    (*env)->CallStaticVoidMethod(env, g_platform_channel_class, g_native_schedule_frame);

    if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
    }

    if (needs_detach) {
        (*g_jvm)->DetachCurrentThread(g_jvm);
    }
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
 * Resolves the DriftNeedsFrame function from the Go shared library.
 *
 * Uses a "resolved" flag to ensure we only attempt resolution once,
 * avoiding log spam and repeated dlsym calls if the symbol is missing.
 *
 * @return 0 if the function was successfully resolved, 1 on failure.
 */
static int resolve_drift_needs_frame(void) {
    if (drift_needs_frame) {
        return 0;
    }

    /* Only attempt resolution once to avoid log spam on every frame */
    if (drift_needs_frame_resolved) {
        return 1;
    }
    drift_needs_frame_resolved = 1;

    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libdrift.so failed: %s", dlerror());
        }
    }

    if (drift_handle) {
        drift_needs_frame = (DriftNeedsFrameFn)dlsym(drift_handle, "DriftNeedsFrame");
    } else {
        drift_needs_frame = (DriftNeedsFrameFn)dlsym(RTLD_DEFAULT, "DriftNeedsFrame");
    }

    if (!drift_needs_frame) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "DriftNeedsFrame not found: %s", dlerror());
    }

    return drift_needs_frame ? 0 : 1;
}

static int resolve_drift_platform_current_frame_seq(void) {
    if (drift_platform_current_frame_seq) {
        return 0;
    }

    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libdrift.so failed: %s", dlerror());
        }
    }

    if (drift_handle) {
        drift_platform_current_frame_seq = (DriftPlatformCurrentFrameSeqFn)dlsym(drift_handle, "DriftPlatformCurrentFrameSeq");
    } else {
        drift_platform_current_frame_seq = (DriftPlatformCurrentFrameSeqFn)dlsym(RTLD_DEFAULT, "DriftPlatformCurrentFrameSeq");
    }

    if (!drift_platform_current_frame_seq) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "DriftPlatformCurrentFrameSeq not found: %s", dlerror());
    }

    return drift_platform_current_frame_seq ? 0 : 1;
}

static int resolve_drift_platform_geometry_pending(void) {
    if (drift_platform_geometry_pending) {
        return 0;
    }

    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libdrift.so failed: %s", dlerror());
        }
    }

    if (drift_handle) {
        drift_platform_geometry_pending = (DriftPlatformGeometryPendingFn)dlsym(drift_handle, "DriftPlatformGeometryPending");
    } else {
        drift_platform_geometry_pending = (DriftPlatformGeometryPendingFn)dlsym(RTLD_DEFAULT, "DriftPlatformGeometryPending");
    }

    if (!drift_platform_geometry_pending) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "DriftPlatformGeometryPending not found: %s", dlerror());
    }

    return drift_platform_geometry_pending ? 0 : 1;
}

JNIEXPORT jlong JNICALL
Java_{{.JNIPackage}}_NativeBridge_currentFrameSeq(
    JNIEnv *env,
    jclass clazz
) {
    (void)env;
    (void)clazz;

    if (resolve_drift_platform_current_frame_seq() != 0) {
        return (jlong)0;
    }
    return (jlong)drift_platform_current_frame_seq();
}

JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_geometryPending(
    JNIEnv *env,
    jclass clazz
) {
    (void)env;
    (void)clazz;

    if (resolve_drift_platform_geometry_pending() != 0) {
        return (jint)0;
    }
    return (jint)drift_platform_geometry_pending();
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
 * JNI implementation for NativeBridge.needsFrame().
 *
 * Checks if the Go engine has any pending work that requires a new frame.
 * Returns 1 if a frame should be rendered, 0 if it can be skipped.
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_needsFrame(
    JNIEnv *env,
    jclass clazz
) {
    (void)env;
    (void)clazz;

    if (resolve_drift_needs_frame() != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Failed to resolve DriftNeedsFrame");
        return 1;  /* Fail-safe: render if we can't check */
    }

    return (jint)drift_needs_frame();
}

/**
 * Resolves the DriftHitTestPlatformView function from the Go shared library.
 *
 * @return 0 if the function was successfully resolved, 1 on failure.
 */
static int resolve_drift_hit_test_platform_view(void) {
    if (drift_hit_test_platform_view) {
        return 0;
    }

    /* Only attempt resolution once to avoid log spam */
    if (drift_hit_test_platform_view_resolved) {
        return 1;
    }
    drift_hit_test_platform_view_resolved = 1;

    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libdrift.so failed: %s", dlerror());
        }
    }

    if (drift_handle) {
        drift_hit_test_platform_view = (DriftHitTestPlatformViewFn)dlsym(drift_handle, "DriftHitTestPlatformView");
    } else {
        drift_hit_test_platform_view = (DriftHitTestPlatformViewFn)dlsym(RTLD_DEFAULT, "DriftHitTestPlatformView");
    }

    if (!drift_hit_test_platform_view) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "DriftHitTestPlatformView not found: %s", dlerror());
    }

    return drift_hit_test_platform_view ? 0 : 1;
}

/**
 * JNI implementation for NativeBridge.hitTestPlatformView().
 *
 * Queries the Go engine's hit test to determine if a platform view is the
 * topmost target at the given pixel coordinates.
 *
 * @param viewID Platform view ID to check
 * @param x      X coordinate in pixels
 * @param y      Y coordinate in pixels
 * @return 1 if topmost (allow touch), 0 if obscured (block touch)
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_hitTestPlatformView(
    JNIEnv *env,
    jclass clazz,
    jlong viewID,
    jdouble x,
    jdouble y
) {
    (void)env;
    (void)clazz;

    if (resolve_drift_hit_test_platform_view() != 0) {
        return 1; /* Fail-safe: allow touch if we can't check */
    }

    return (jint)drift_hit_test_platform_view((int64_t)viewID, x, y);
}

/* =========================================================================
 * AHardwareBuffer pool for SurfaceControl-based rendering.
 *
 * Each slot holds an AHardwareBuffer, an EGL image wrapping it, a GL texture
 * backed by that image, and an FBO targeting the texture. acquireBuffer()
 * round-robins through the pool and binds the next FBO so that subsequent
 * GL rendering (Skia) targets the AHardwareBuffer.
 * ========================================================================= */

#define MAX_BUFFER_COUNT 4

typedef struct {
    AHardwareBuffer *buffer;
    EGLImageKHR image;
    GLuint texture;
    GLuint fbo;
    GLuint depthStencil;
} BufferSlot;

typedef struct {
    BufferSlot slots[MAX_BUFFER_COUNT];
    int count;
    int width;
    int height;
    int current; /* index of most recently acquired slot */
    int hasFenceSupport; /* 1 if EGL_ANDROID_native_fence_sync is available */
} BufferPool;

/* EGL function pointers resolved at pool creation. */
static PFNEGLCREATEIMAGEKHRPROC egl_createImageKHR = NULL;
static PFNEGLDESTROYIMAGEKHRPROC egl_destroyImageKHR = NULL;
static PFNGLEGLIMAGETARGETTEXTURE2DOESPROC gl_imageTargetTexture2DOES = NULL;
static PFNEGLCREATESYNCKHRPROC egl_createSyncKHR = NULL;
static PFNEGLDESTROYSYNCKHRPROC egl_destroySyncKHR = NULL;
static PFNEGLDUPNATIVEFENCEFDANDROIDPROC egl_dupNativeFenceFD = NULL;

/* eglGetNativeClientBufferANDROID: wraps AHardwareBuffer as EGLClientBuffer */
typedef EGLClientBuffer (EGLAPIENTRYP PFNEGLGETNATIVECLIENTBUFFERANDROIDPROC)(const AHardwareBuffer *);
static PFNEGLGETNATIVECLIENTBUFFERANDROIDPROC egl_getNativeClientBuffer = NULL;

static int resolve_egl_extensions(void) {
    if (egl_createImageKHR) return 0; /* already resolved */

    egl_createImageKHR = (PFNEGLCREATEIMAGEKHRPROC)eglGetProcAddress("eglCreateImageKHR");
    egl_destroyImageKHR = (PFNEGLDESTROYIMAGEKHRPROC)eglGetProcAddress("eglDestroyImageKHR");
    gl_imageTargetTexture2DOES = (PFNGLEGLIMAGETARGETTEXTURE2DOESPROC)eglGetProcAddress("glEGLImageTargetTexture2DOES");
    egl_getNativeClientBuffer = (PFNEGLGETNATIVECLIENTBUFFERANDROIDPROC)eglGetProcAddress("eglGetNativeClientBufferANDROID");

    if (!egl_createImageKHR || !egl_destroyImageKHR || !gl_imageTargetTexture2DOES || !egl_getNativeClientBuffer) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Required EGL image extensions unavailable");
        return -1;
    }

    /* Fence extensions are optional; fall back to glFinish(). */
    egl_createSyncKHR = (PFNEGLCREATESYNCKHRPROC)eglGetProcAddress("eglCreateSyncKHR");
    egl_destroySyncKHR = (PFNEGLDESTROYSYNCKHRPROC)eglGetProcAddress("eglDestroySyncKHR");
    egl_dupNativeFenceFD = (PFNEGLDUPNATIVEFENCEFDANDROIDPROC)eglGetProcAddress("eglDupNativeFenceFDANDROID");

    return 0;
}

static void destroy_slot(BufferSlot *slot) {
    EGLDisplay display = eglGetCurrentDisplay();

    if (slot->fbo) { glDeleteFramebuffers(1, &slot->fbo); slot->fbo = 0; }
    if (slot->depthStencil) { glDeleteRenderbuffers(1, &slot->depthStencil); slot->depthStencil = 0; }
    if (slot->texture) { glDeleteTextures(1, &slot->texture); slot->texture = 0; }
    if (slot->image != EGL_NO_IMAGE_KHR && egl_destroyImageKHR) {
        egl_destroyImageKHR(display, slot->image);
        slot->image = EGL_NO_IMAGE_KHR;
    }
    if (slot->buffer) { AHardwareBuffer_release(slot->buffer); slot->buffer = NULL; }
}

static int init_slot(BufferSlot *slot, int width, int height) {
    EGLDisplay display = eglGetCurrentDisplay();

    /* Allocate AHardwareBuffer */
    AHardwareBuffer_Desc desc = {
        .width = (uint32_t)width,
        .height = (uint32_t)height,
        .layers = 1,
        .format = AHARDWAREBUFFER_FORMAT_R8G8B8A8_UNORM,
        .usage = AHARDWAREBUFFER_USAGE_GPU_COLOR_OUTPUT |
                 AHARDWAREBUFFER_USAGE_GPU_SAMPLED_IMAGE |
                 AHARDWAREBUFFER_USAGE_COMPOSER_OVERLAY,
        .stride = 0,
        .rfu0 = 0,
        .rfu1 = 0,
    };
    if (AHardwareBuffer_allocate(&desc, &slot->buffer) != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "AHardwareBuffer_allocate failed");
        return -1;
    }

    /* Wrap in EGL image */
    EGLClientBuffer clientBuffer = egl_getNativeClientBuffer(slot->buffer);
    if (!clientBuffer) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "eglGetNativeClientBufferANDROID failed");
        destroy_slot(slot);
        return -1;
    }
    EGLint imageAttribs[] = { EGL_NONE };
    slot->image = egl_createImageKHR(display, EGL_NO_CONTEXT,
        EGL_NATIVE_BUFFER_ANDROID, clientBuffer, imageAttribs);
    if (slot->image == EGL_NO_IMAGE_KHR) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "eglCreateImageKHR failed");
        destroy_slot(slot);
        return -1;
    }

    /* Create GL texture backed by the EGL image */
    glGenTextures(1, &slot->texture);
    glBindTexture(GL_TEXTURE_2D, slot->texture);
    gl_imageTargetTexture2DOES(GL_TEXTURE_2D, (GLeglImageOES)slot->image);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
    glBindTexture(GL_TEXTURE_2D, 0);

    /* Create FBO */
    glGenFramebuffers(1, &slot->fbo);
    glBindFramebuffer(GL_FRAMEBUFFER, slot->fbo);
    glFramebufferTexture2D(GL_FRAMEBUFFER, GL_COLOR_ATTACHMENT0, GL_TEXTURE_2D, slot->texture, 0);

    /* Create depth/stencil renderbuffer (Skia needs stencil for complex paths) */
    glGenRenderbuffers(1, &slot->depthStencil);
    glBindRenderbuffer(GL_RENDERBUFFER, slot->depthStencil);
    glRenderbufferStorage(GL_RENDERBUFFER, GL_DEPTH24_STENCIL8_OES, width, height);
    glFramebufferRenderbuffer(GL_FRAMEBUFFER, GL_DEPTH_ATTACHMENT, GL_RENDERBUFFER, slot->depthStencil);
    glFramebufferRenderbuffer(GL_FRAMEBUFFER, GL_STENCIL_ATTACHMENT, GL_RENDERBUFFER, slot->depthStencil);

    GLenum status = glCheckFramebufferStatus(GL_FRAMEBUFFER);
    glBindFramebuffer(GL_FRAMEBUFFER, 0);
    glBindRenderbuffer(GL_RENDERBUFFER, 0);

    if (status != GL_FRAMEBUFFER_COMPLETE) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "FBO incomplete: 0x%x", status);
        destroy_slot(slot);
        return -1;
    }

    return 0;
}

/**
 * JNI: createBufferPool(width, height, count) -> pool pointer (as jlong)
 */
JNIEXPORT jlong JNICALL
Java_{{.JNIPackage}}_NativeBridge_createBufferPool(
    JNIEnv *env, jclass clazz,
    jint width, jint height, jint count
) {
    (void)env; (void)clazz;

    if (resolve_egl_extensions() != 0) return 0;
    if (count < 1 || count > MAX_BUFFER_COUNT) count = 3;

    BufferPool *pool = (BufferPool *)calloc(1, sizeof(BufferPool));
    if (!pool) return 0;

    pool->width = width;
    pool->height = height;
    pool->count = count;
    pool->current = -1;

    /* Check fence support */
    pool->hasFenceSupport = (egl_createSyncKHR && egl_destroySyncKHR && egl_dupNativeFenceFD) ? 1 : 0;
    if (!pool->hasFenceSupport) {
        __android_log_print(ANDROID_LOG_WARN, "DriftJNI",
            "EGL_ANDROID_native_fence_sync unavailable; using glFinish fallback");
    }

    for (int i = 0; i < count; i++) {
        if (init_slot(&pool->slots[i], width, height) != 0) {
            /* Clean up already-initialized slots */
            for (int j = 0; j < i; j++) destroy_slot(&pool->slots[j]);
            free(pool);
            return 0;
        }
    }

    __android_log_print(ANDROID_LOG_INFO, "DriftJNI",
        "Buffer pool created: %dx%d, %d buffers, fence=%d", width, height, count, pool->hasFenceSupport);
    return (jlong)(uintptr_t)pool;
}

/**
 * JNI: destroyBufferPool(pool)
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_destroyBufferPool(
    JNIEnv *env, jclass clazz, jlong poolPtr
) {
    (void)env; (void)clazz;
    BufferPool *pool = (BufferPool *)(uintptr_t)poolPtr;
    if (!pool) return;

    for (int i = 0; i < pool->count; i++) {
        destroy_slot(&pool->slots[i]);
    }
    free(pool);
}

/**
 * JNI: acquireBuffer(pool) -> buffer index
 *
 * Round-robins through the pool and binds the FBO for the next slot.
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_acquireBuffer(
    JNIEnv *env, jclass clazz, jlong poolPtr
) {
    (void)env; (void)clazz;
    BufferPool *pool = (BufferPool *)(uintptr_t)poolPtr;
    if (!pool || pool->count <= 0) return -1;

    pool->current = (pool->current + 1) % pool->count;
    glBindFramebuffer(GL_FRAMEBUFFER, pool->slots[pool->current].fbo);
    glViewport(0, 0, pool->width, pool->height);
    return pool->current;
}

/**
 * JNI: resizeBufferPool(pool, width, height) -> 0 on success, -1 on failure
 *
 * Destroys existing slots and recreates them at the new dimensions.
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_resizeBufferPool(
    JNIEnv *env, jclass clazz, jlong poolPtr, jint width, jint height
) {
    (void)env; (void)clazz;
    BufferPool *pool = (BufferPool *)(uintptr_t)poolPtr;
    if (!pool) return -1;
    if (pool->width == width && pool->height == height) return 0;

    /* Ensure all GPU work targeting old FBOs completes before destroying them. */
    glFinish();

    for (int i = 0; i < pool->count; i++) {
        destroy_slot(&pool->slots[i]);
    }

    pool->width = width;
    pool->height = height;
    pool->current = -1;

    for (int i = 0; i < pool->count; i++) {
        if (init_slot(&pool->slots[i], width, height) != 0) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "resizeBufferPool: init_slot %d failed", i);
            /* Clean up any slots that were successfully initialized. */
            for (int j = 0; j < i; j++) destroy_slot(&pool->slots[j]);
            pool->count = 0;
            return -1;
        }
    }

    __android_log_print(ANDROID_LOG_INFO, "DriftJNI", "Buffer pool resized: %dx%d", width, height);
    return 0;
}

/**
 * JNI: createFence() -> native fence FD
 *
 * Creates an EGL sync fence and extracts its native FD. If fence extensions
 * are unavailable, falls back to glFinish() and returns -1 (no fence).
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_createFence(
    JNIEnv *env, jclass clazz, jlong poolPtr
) {
    (void)env; (void)clazz;
    BufferPool *pool = (BufferPool *)(uintptr_t)poolPtr;

    if (!pool || !pool->hasFenceSupport) {
        glFinish();
        return -1;
    }

    EGLDisplay display = eglGetCurrentDisplay();
    EGLint attribs[] = { EGL_SYNC_NATIVE_FENCE_FD_ANDROID, EGL_NO_NATIVE_FENCE_FD_ANDROID, EGL_NONE };
    EGLSyncKHR sync = egl_createSyncKHR(display, EGL_SYNC_NATIVE_FENCE_ANDROID, attribs);
    if (sync == EGL_NO_SYNC_KHR) {
        glFinish();
        return -1;
    }

    /* Flush to ensure the fence is enqueued in the GPU command stream */
    glFlush();

    int fd = egl_dupNativeFenceFD(display, sync);
    egl_destroySyncKHR(display, sync);

    if (fd < 0) {
        glFinish();
        return -1;
    }

    return fd;
}

/**
 * JNI: createSurfaceControl(surface) -> ASurfaceControl* as jlong
 *
 * Creates a child ASurfaceControl from the given Surface's ANativeWindow.
 */
JNIEXPORT jlong JNICALL
Java_{{.JNIPackage}}_NativeBridge_createSurfaceControl(
    JNIEnv *env, jclass clazz, jobject surface
) {
    (void)clazz;

    if (resolve_surface_control() != 0) return (jlong)0;

    ANativeWindow *window = ANativeWindow_fromSurface(env, surface);
    if (!window) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "ANativeWindow_fromSurface failed");
        return (jlong)0;
    }

    ASurfaceControl *surfaceControl = sc_createFromWindow(window, "DriftContent");
    ANativeWindow_release(window);

    if (!surfaceControl) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "ASurfaceControl_createFromWindow failed");
        return (jlong)0;
    }

    /* Make the child surface visible */
    ASurfaceTransaction *txn = sc_createTransaction();
    if (!txn) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "ASurfaceTransaction_create failed");
        sc_release(surfaceControl);
        return (jlong)0;
    }
    sc_setVisibility(txn, surfaceControl, 1);
    sc_apply(txn);
    sc_deleteTransaction(txn);

    __android_log_print(ANDROID_LOG_INFO, "DriftJNI", "SurfaceControl created");
    return (jlong)(uintptr_t)surfaceControl;
}

/**
 * JNI: destroySurfaceControl(surfaceControlPtr)
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_destroySurfaceControl(
    JNIEnv *env, jclass clazz, jlong surfaceControlPtr
) {
    (void)env; (void)clazz;
    ASurfaceControl *surfaceControl = (ASurfaceControl *)(uintptr_t)surfaceControlPtr;
    if (!surfaceControl) return;
    sc_release(surfaceControl);
}

/**
 * JNI: presentBuffer(pool, bufferIndex, fenceFd)
 *
 * Creates a SurfaceControl transaction, sets the buffer from the pool,
 * and applies it. The fence FD is consumed (caller must not close it).
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_presentBuffer(
    JNIEnv *env, jclass clazz,
    jlong poolPtr, jlong surfaceControlPtr, jint bufferIndex, jint fenceFd
) {
    (void)env; (void)clazz;

    BufferPool *pool = (BufferPool *)(uintptr_t)poolPtr;
    ASurfaceControl *surfaceControl = (ASurfaceControl *)(uintptr_t)surfaceControlPtr;
    if (!pool || !surfaceControl || !sc_createTransaction ||
        bufferIndex < 0 || bufferIndex >= pool->count) {
        if (fenceFd >= 0) close(fenceFd);
        return;
    }

    ASurfaceTransaction *txn = sc_createTransaction();
    if (!txn) {
        if (fenceFd >= 0) close(fenceFd);
        return;
    }
    sc_setBuffer(txn, surfaceControl, pool->slots[bufferIndex].buffer, fenceFd);
    sc_apply(txn);
    sc_deleteTransaction(txn);
}

JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_closeFenceFd(
    JNIEnv *env, jclass clazz, jint fd
) {
    (void)env; (void)clazz;
    if (fd >= 0) {
        close(fd);
    }
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

    /* Already initialized — JNI refs and Go handler are still valid. */
    if (g_platform_channel_class && g_handle_method_call) {
        return 0;
    }

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

    /* Find the static method: nativeScheduleFrame() -> void */
    g_native_schedule_frame = (*env)->GetStaticMethodID(
        env, g_platform_channel_class,
        "nativeScheduleFrame",
        "()V"
    );

    if (!g_native_schedule_frame) {
        __android_log_print(ANDROID_LOG_WARN, "DriftJNI", "nativeScheduleFrame method not found (on-demand scheduling disabled)");
    }

    /* Register our native handler with Go */
    if (resolve_and_register_native_handler() != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Failed to register native handler");
        return -1;
    }

    /* Register the schedule-frame handler with Go for on-demand rendering */
    if (g_native_schedule_frame) {
        if (!drift_set_schedule_frame_handler && drift_handle) {
            drift_set_schedule_frame_handler = (DriftSetScheduleFrameHandlerFn)dlsym(
                drift_handle, "DriftSetScheduleFrameHandler"
            );
        }
        if (drift_set_schedule_frame_handler) {
            drift_set_schedule_frame_handler(schedule_frame_handler);
            __android_log_print(ANDROID_LOG_INFO, "DriftJNI", "Schedule-frame handler registered");
        }
    }

    __android_log_print(ANDROID_LOG_INFO, "DriftJNI", "Platform channels initialized");
    return 0;
}
