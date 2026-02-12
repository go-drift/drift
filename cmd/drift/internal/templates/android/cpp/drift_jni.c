/**
 * @file drift_jni.c
 * @brief JNI bridge between Android Java/Kotlin code and the Go Drift engine.
 *
 * This file provides the native implementation for the NativeBridge Kotlin object.
 * It dynamically loads the Go shared library (libdrift.so) at runtime and resolves
 * the exported Go functions (DriftPointerEvent, DriftStepAndSnapshot, etc.).
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

#include <android/hardware_buffer.h>
#include <android/hardware_buffer_jni.h>
#include <EGL/egl.h>
#include <EGL/eglext.h>
#include <GLES3/gl3.h>
#include <GLES2/gl2ext.h>

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
 * Function pointer type for DriftBackButtonPressed.
 * Matches the signature exported by Go:
 *   func DriftBackButtonPressed() C.int
 *
 * @return 1 if back was handled (route popped), 0 if not handled (at root)
 */
typedef int (*DriftBackButtonFn)(void);

typedef void (*DriftRequestFrameFn)(void);
typedef int (*DriftNeedsFrameFn)(void);
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
static DriftPointerFn drift_pointer_event = NULL;
static DriftSetScaleFn drift_set_scale = NULL;
static DriftAppInitFn drift_app_init = NULL;
static DriftSkiaInitFn drift_skia_init = NULL;
static DriftPlatformHandleEventFn drift_platform_event = NULL;
static DriftPlatformHandleEventErrorFn drift_platform_event_error = NULL;
static DriftPlatformHandleEventDoneFn drift_platform_event_done = NULL;
static DriftPlatformIsStreamActiveFn drift_platform_stream_active = NULL;
static DriftPlatformSetNativeHandlerFn drift_platform_set_handler = NULL;
static DriftBackButtonFn drift_back_button = NULL;
static DriftRequestFrameFn drift_request_frame = NULL;
static DriftNeedsFrameFn drift_needs_frame = NULL;
static DriftHitTestPlatformViewFn drift_hit_test_platform_view = NULL;
static DriftSetScheduleFrameHandlerFn drift_set_schedule_frame_handler = NULL;

/* Function pointer types for unified orchestrator */
typedef int (*DriftStepAndSnapshotFn)(int width, int height, char **outData, int *outLen);
typedef int (*DriftSkiaRenderFrameSyncFn)(int width, int height);
typedef void (*DriftSkiaPurgeResourcesFn)(void);

static DriftStepAndSnapshotFn drift_step_and_snapshot = NULL;
static DriftSkiaRenderFrameSyncFn drift_skia_render_frame_sync = NULL;
static DriftSkiaPurgeResourcesFn drift_skia_purge_resources = NULL;

/* Handle to the loaded Go shared library. NULL until loaded. */
static void *drift_handle = NULL;

/**
 * Generic symbol resolver. Opens libdrift.so if needed, then looks up the
 * named symbol via dlsym. The resolved pointer is written to *out and cached
 * across calls (caller passes the address of a static function pointer).
 *
 * @param name Symbol name to resolve (e.g. "DriftPointerEvent")
 * @param out  Address of the cached function pointer
 * @return 0 on success, 1 if the symbol could not be found
 */
static int resolve_symbol(const char *name, void **out) {
    if (*out) return 0;
    if (!drift_handle) {
        drift_handle = dlopen("libdrift.so", RTLD_NOW | RTLD_GLOBAL);
        if (!drift_handle) {
            __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "dlopen libdrift.so failed: %s", dlerror());
        }
    }
    if (drift_handle) {
        *out = dlsym(drift_handle, name);
    } else {
        *out = dlsym(RTLD_DEFAULT, name);
    }
    if (!*out) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "%s not found: %s", name, dlerror());
    }
    return *out ? 0 : 1;
}

/* ─── EGL state ─── */
static EGLDisplay g_egl_display = EGL_NO_DISPLAY;
static EGLContext g_egl_context = EGL_NO_CONTEXT;
static EGLSurface g_egl_surface = EGL_NO_SURFACE; /* 1x1 pbuffer */

/* ─── HardwareBuffer FBO state ─── */
static AHardwareBuffer *g_hwb = NULL;
static EGLImageKHR g_egl_image = EGL_NO_IMAGE_KHR;
static GLuint g_hwb_texture = 0;
static GLuint g_hwb_fbo = 0;
static GLuint g_hwb_stencil_rb = 0;
static int g_hwb_width = 0;
static int g_hwb_height = 0;

/* ─── EGL extension function pointers ─── */
static PFNEGLCREATEIMAGEKHRPROC eglCreateImageKHR_fn = NULL;
static PFNEGLDESTROYIMAGEKHRPROC eglDestroyImageKHR_fn = NULL;
static PFNGLEGLIMAGETARGETTEXTURE2DOESPROC glEGLImageTargetTexture2DOES_fn = NULL;

typedef EGLClientBuffer (EGLAPIENTRYP PFNEGLGETNATIVECLIENTBUFFERANDROIDPROC)(const AHardwareBuffer *buffer);
static PFNEGLGETNATIVECLIENTBUFFERANDROIDPROC eglGetNativeClientBufferANDROID_fn = NULL;

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
 * handled by UnifiedFrameOrchestrator's post-render NeedsFrame() check on
 * the UI thread.
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

    if (resolve_symbol("DriftPlatformSetNativeHandler", (void **)&drift_platform_set_handler) != 0) {
        return -1;
    }

    /* Register our native method handler with Go */
    drift_platform_set_handler((void *)native_method_handler);
    g_native_handler_registered = 1;
    __android_log_print(ANDROID_LOG_INFO, "DriftJNI", "Native method handler registered");

    return 0;
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

    if (resolve_symbol("DriftAppInit", (void **)&drift_app_init) != 0) {
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

    if (resolve_symbol("DriftSkiaInitGL", (void **)&drift_skia_init) != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Failed to resolve DriftSkiaInitGL");
        return 1;
    }

    return (jint)drift_skia_init();
}

/**
 * JNI implementation for NativeBridge.pointerEvent().
 *
 * Called from SkiaHostView.onTouchEvent() when the user touches the screen.
 * This function forwards touch events to the Go engine for processing.
 *
 * @param env       JNI environment pointer (provides JNI functions)
 * @param clazz     Reference to the NativeBridge class (unused, static method)
 * @param pointerID Unique identifier for this pointer/touch (from MotionEvent.getPointerId())
 * @param phase     Touch phase: 0=Down, 1=Move, 2=Up, 3=Cancel
 *                  Maps from Android MotionEvent actions in SkiaHostView
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
    if (resolve_symbol("DriftPointerEvent", (void **)&drift_pointer_event) != 0) {
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
    if (resolve_symbol("DriftSetDeviceScale", (void **)&drift_set_scale) != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Failed to resolve DriftSetDeviceScale");
        return;
    }

    /* Forward the scale to the Go engine */
    drift_set_scale(scale);
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

    if (resolve_symbol("DriftPlatformHandleEvent", (void **)&drift_platform_event) != 0) {
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

    if (resolve_symbol("DriftPlatformHandleEventError", (void **)&drift_platform_event_error) != 0) {
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

    if (resolve_symbol("DriftPlatformHandleEventDone", (void **)&drift_platform_event_done) != 0) {
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

    if (resolve_symbol("DriftPlatformIsStreamActive", (void **)&drift_platform_stream_active) != 0) {
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

    if (resolve_symbol("DriftBackButtonPressed", (void **)&drift_back_button) != 0) {
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

    if (resolve_symbol("DriftRequestFrame", (void **)&drift_request_frame) != 0) {
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

    if (resolve_symbol("DriftNeedsFrame", (void **)&drift_needs_frame) != 0) {
        return 1;  /* Fail-safe: render if we can't check */
    }

    return (jint)drift_needs_frame();
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

    if (resolve_symbol("DriftHitTestPlatformView", (void **)&drift_hit_test_platform_view) != 0) {
        return 1; /* Fail-safe: allow touch if we can't check */
    }

    return (jint)drift_hit_test_platform_view((int64_t)viewID, x, y);
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

/* ═══════════════════════════════════════════════════════════════════════════
 * Unified Frame Orchestrator: EGL, HardwareBuffer, new JNI
 * ═══════════════════════════════════════════════════════════════════════════ */

/**
 * Cleans up EGL context and display. Call on init failure to avoid leaking
 * resources allocated before the error.
 */
static void cleanup_egl(void) {
    if (g_egl_context != EGL_NO_CONTEXT) {
        eglDestroyContext(g_egl_display, g_egl_context);
        g_egl_context = EGL_NO_CONTEXT;
    }
    if (g_egl_display != EGL_NO_DISPLAY) {
        eglTerminate(g_egl_display);
        g_egl_display = EGL_NO_DISPLAY;
    }
}

static void resolve_egl_extensions(void) {
    if (!eglCreateImageKHR_fn) {
        eglCreateImageKHR_fn = (PFNEGLCREATEIMAGEKHRPROC)eglGetProcAddress("eglCreateImageKHR");
    }
    if (!eglDestroyImageKHR_fn) {
        eglDestroyImageKHR_fn = (PFNEGLDESTROYIMAGEKHRPROC)eglGetProcAddress("eglDestroyImageKHR");
    }
    if (!glEGLImageTargetTexture2DOES_fn) {
        glEGLImageTargetTexture2DOES_fn = (PFNGLEGLIMAGETARGETTEXTURE2DOESPROC)eglGetProcAddress("glEGLImageTargetTexture2DOES");
    }
    if (!eglGetNativeClientBufferANDROID_fn) {
        eglGetNativeClientBufferANDROID_fn = (PFNEGLGETNATIVECLIENTBUFFERANDROIDPROC)eglGetProcAddress("eglGetNativeClientBufferANDROID");
    }
}

/**
 * JNI: NativeBridge.initEGL()
 * Creates an EGL display, context, and 1x1 pbuffer surface.
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_initEGL(JNIEnv *env, jclass clazz) {
    (void)env; (void)clazz;

    g_egl_display = eglGetDisplay(EGL_DEFAULT_DISPLAY);
    if (g_egl_display == EGL_NO_DISPLAY) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "eglGetDisplay failed");
        return -1;
    }

    EGLint major, minor;
    if (!eglInitialize(g_egl_display, &major, &minor)) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "eglInitialize failed");
        return -1;
    }

    /* Choose config with GLES3 + RGBA8 */
    EGLint configAttribs[] = {
        EGL_RENDERABLE_TYPE, EGL_OPENGL_ES3_BIT,
        EGL_RED_SIZE, 8,
        EGL_GREEN_SIZE, 8,
        EGL_BLUE_SIZE, 8,
        EGL_ALPHA_SIZE, 8,
        EGL_STENCIL_SIZE, 8,
        EGL_SURFACE_TYPE, EGL_PBUFFER_BIT,
        EGL_NONE
    };
    EGLConfig config;
    EGLint numConfigs;
    if (!eglChooseConfig(g_egl_display, configAttribs, &config, 1, &numConfigs) || numConfigs == 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "eglChooseConfig failed");
        return -1;
    }

    /* Create GLES3 context */
    EGLint ctxAttribs[] = { EGL_CONTEXT_CLIENT_VERSION, 3, EGL_NONE };
    g_egl_context = eglCreateContext(g_egl_display, config, EGL_NO_CONTEXT, ctxAttribs);
    if (g_egl_context == EGL_NO_CONTEXT) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "eglCreateContext failed");
        cleanup_egl();
        return -1;
    }

    /* Create 1x1 pbuffer (context needs a surface to be current) */
    EGLint pbufAttribs[] = { EGL_WIDTH, 1, EGL_HEIGHT, 1, EGL_NONE };
    g_egl_surface = eglCreatePbufferSurface(g_egl_display, config, pbufAttribs);
    if (g_egl_surface == EGL_NO_SURFACE) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "eglCreatePbufferSurface failed");
        cleanup_egl();
        return -1;
    }

    resolve_egl_extensions();

    __android_log_print(ANDROID_LOG_INFO, "DriftJNI", "EGL initialized: %d.%d", major, minor);
    return 0;
}

/**
 * JNI: NativeBridge.createHwbFBO(width, height)
 * Allocates AHardwareBuffer, creates EGLImage, GL texture, stencil RB, FBO.
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_createHwbFBO(JNIEnv *env, jclass clazz, jint width, jint height) {
    (void)env; (void)clazz;

    if (!eglGetNativeClientBufferANDROID_fn || !eglCreateImageKHR_fn || !glEGLImageTargetTexture2DOES_fn) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "EGL extensions not available for HWB");
        return -1;
    }

    /* Allocate AHardwareBuffer */
    AHardwareBuffer_Desc desc = {
        .width = (uint32_t)width,
        .height = (uint32_t)height,
        .layers = 1,
        .format = AHARDWAREBUFFER_FORMAT_R8G8B8A8_UNORM,
        .usage = AHARDWAREBUFFER_USAGE_GPU_FRAMEBUFFER |
                 AHARDWAREBUFFER_USAGE_GPU_SAMPLED_IMAGE |
                 AHARDWAREBUFFER_USAGE_COMPOSER_OVERLAY,
        .stride = 0,
        .rfu0 = 0,
        .rfu1 = 0,
    };
    if (AHardwareBuffer_allocate(&desc, &g_hwb) != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "AHardwareBuffer_allocate failed");
        return -1;
    }

    /* Create EGLImage from HWB */
    EGLClientBuffer clientBuf = eglGetNativeClientBufferANDROID_fn(g_hwb);
    if (!clientBuf) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "eglGetNativeClientBufferANDROID failed");
        AHardwareBuffer_release(g_hwb);
        g_hwb = NULL;
        return -1;
    }

    EGLint imageAttribs[] = { EGL_NONE };
    g_egl_image = eglCreateImageKHR_fn(g_egl_display, EGL_NO_CONTEXT, EGL_NATIVE_BUFFER_ANDROID, clientBuf, imageAttribs);
    if (g_egl_image == EGL_NO_IMAGE_KHR) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "eglCreateImageKHR failed");
        AHardwareBuffer_release(g_hwb);
        g_hwb = NULL;
        return -1;
    }

    /* Create GL texture backed by EGLImage */
    glGenTextures(1, &g_hwb_texture);
    glBindTexture(GL_TEXTURE_2D, g_hwb_texture);
    glEGLImageTargetTexture2DOES_fn(GL_TEXTURE_2D, (GLeglImageOES)g_egl_image);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
    glBindTexture(GL_TEXTURE_2D, 0);

    /* Create stencil renderbuffer */
    glGenRenderbuffers(1, &g_hwb_stencil_rb);
    glBindRenderbuffer(GL_RENDERBUFFER, g_hwb_stencil_rb);
    glRenderbufferStorage(GL_RENDERBUFFER, GL_STENCIL_INDEX8, width, height);
    glBindRenderbuffer(GL_RENDERBUFFER, 0);

    /* Create FBO */
    glGenFramebuffers(1, &g_hwb_fbo);
    glBindFramebuffer(GL_FRAMEBUFFER, g_hwb_fbo);
    glFramebufferTexture2D(GL_FRAMEBUFFER, GL_COLOR_ATTACHMENT0, GL_TEXTURE_2D, g_hwb_texture, 0);
    glFramebufferRenderbuffer(GL_FRAMEBUFFER, GL_STENCIL_ATTACHMENT, GL_RENDERBUFFER, g_hwb_stencil_rb);

    GLenum status = glCheckFramebufferStatus(GL_FRAMEBUFFER);
    glBindFramebuffer(GL_FRAMEBUFFER, 0);

    if (status != GL_FRAMEBUFFER_COMPLETE) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "FBO incomplete: 0x%x", status);
        glDeleteFramebuffers(1, &g_hwb_fbo); g_hwb_fbo = 0;
        glDeleteRenderbuffers(1, &g_hwb_stencil_rb); g_hwb_stencil_rb = 0;
        glDeleteTextures(1, &g_hwb_texture); g_hwb_texture = 0;
        eglDestroyImageKHR_fn(g_egl_display, g_egl_image); g_egl_image = EGL_NO_IMAGE_KHR;
        AHardwareBuffer_release(g_hwb); g_hwb = NULL;
        return -1;
    }

    g_hwb_width = width;
    g_hwb_height = height;

    __android_log_print(ANDROID_LOG_INFO, "DriftJNI", "HWB FBO created: %dx%d fbo=%u", width, height, g_hwb_fbo);
    return 0;
}

/**
 * JNI: NativeBridge.destroyHwbFBO()
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_destroyHwbFBO(JNIEnv *env, jclass clazz) {
    (void)env; (void)clazz;

    if (g_hwb_fbo) { glDeleteFramebuffers(1, &g_hwb_fbo); g_hwb_fbo = 0; }
    if (g_hwb_stencil_rb) { glDeleteRenderbuffers(1, &g_hwb_stencil_rb); g_hwb_stencil_rb = 0; }
    if (g_hwb_texture) { glDeleteTextures(1, &g_hwb_texture); g_hwb_texture = 0; }
    if (g_egl_image != EGL_NO_IMAGE_KHR && eglDestroyImageKHR_fn) {
        eglDestroyImageKHR_fn(g_egl_display, g_egl_image);
        g_egl_image = EGL_NO_IMAGE_KHR;
    }
    if (g_hwb) { AHardwareBuffer_release(g_hwb); g_hwb = NULL; }
    g_hwb_width = 0;
    g_hwb_height = 0;
}

/**
 * JNI: NativeBridge.bindHwbFBO()
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_bindHwbFBO(JNIEnv *env, jclass clazz) {
    (void)env; (void)clazz;
    glBindFramebuffer(GL_FRAMEBUFFER, g_hwb_fbo);
    glViewport(0, 0, g_hwb_width, g_hwb_height);
}

/**
 * JNI: NativeBridge.unbindHwbFBO()
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_unbindHwbFBO(JNIEnv *env, jclass clazz) {
    (void)env; (void)clazz;
    glBindFramebuffer(GL_FRAMEBUFFER, 0);
}

/**
 * JNI: NativeBridge.makeCurrent()
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_makeCurrent(JNIEnv *env, jclass clazz) {
    (void)env; (void)clazz;
    if (g_egl_display != EGL_NO_DISPLAY && g_egl_context != EGL_NO_CONTEXT) {
        eglMakeCurrent(g_egl_display, g_egl_surface, g_egl_surface, g_egl_context);
    }
}

/**
 * JNI: NativeBridge.releaseContext()
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_releaseContext(JNIEnv *env, jclass clazz) {
    (void)env; (void)clazz;
    if (g_egl_display != EGL_NO_DISPLAY) {
        eglMakeCurrent(g_egl_display, EGL_NO_SURFACE, EGL_NO_SURFACE, EGL_NO_CONTEXT);
    }
}

/**
 * JNI: NativeBridge.getHardwareBuffer()
 * Returns the current AHardwareBuffer as a Java HardwareBuffer object.
 * Used by SkiaHostView to wrap as a Bitmap for HWUI onDraw().
 */
JNIEXPORT jobject JNICALL
Java_{{.JNIPackage}}_NativeBridge_getHardwareBuffer(JNIEnv *env, jclass clazz) {
    (void)clazz;
    if (!g_hwb) return NULL;
    return AHardwareBuffer_toHardwareBuffer(env, g_hwb);
}


/**
 * JNI: NativeBridge.stepAndSnapshot(width, height) -> ByteArray?
 * Calls Go DriftStepAndSnapshot and returns the JSON snapshot bytes.
 */
JNIEXPORT jbyteArray JNICALL
Java_{{.JNIPackage}}_NativeBridge_stepAndSnapshot(JNIEnv *env, jclass clazz, jint width, jint height) {
    (void)clazz;

    if (resolve_symbol("DriftStepAndSnapshot", (void **)&drift_step_and_snapshot) != 0) {
        return NULL;
    }

    char *outData = NULL;
    int outLen = 0;
    int result = drift_step_and_snapshot(width, height, &outData, &outLen);
    if (result != 0) {
        if (outData) free(outData);
        return NULL;
    }

    if (!outData || outLen <= 0) {
        if (outData) free(outData);
        return NULL;
    }

    jbyteArray jdata = (*env)->NewByteArray(env, outLen);
    if (jdata) {
        (*env)->SetByteArrayRegion(env, jdata, 0, outLen, (const jbyte *)outData);
    }
    free(outData);
    return jdata;
}

/**
 * JNI: NativeBridge.renderFrameSync(width, height)
 * Calls Go DriftSkiaRenderFrameSync (split pipeline render after StepAndSnapshot).
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_renderFrameSync(JNIEnv *env, jclass clazz, jint width, jint height) {
    (void)env; (void)clazz;

    if (resolve_symbol("DriftSkiaRenderFrameSync", (void **)&drift_skia_render_frame_sync) != 0) {
        return -1;
    }

    return (jint)drift_skia_render_frame_sync(width, height);
}

/**
 * JNI: NativeBridge.purgeResources()
 * Resets GL state tracking and releases all cached GPU resources.
 * Call after sleep/wake or surface recreation.
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_purgeResources(JNIEnv *env, jclass clazz) {
    (void)env; (void)clazz;

    if (resolve_symbol("DriftSkiaPurgeResources", (void **)&drift_skia_purge_resources) != 0) {
        return;
    }

    drift_skia_purge_resources();
}
