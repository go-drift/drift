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
#include <vulkan/vulkan.h>
#include <vulkan/vulkan_android.h>
#include "drift_vulkan_extensions.h"

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
 * Function pointer type for DriftSkiaInitVulkan.
 * Matches the signature exported by Go:
 *   func DriftSkiaInitVulkan(...) C.int
 */
typedef int (*DriftSkiaInitVulkanFn)(
    uintptr_t instance, uintptr_t phys_device, uintptr_t device,
    uintptr_t queue, uint32_t queue_family_index,
    uintptr_t get_instance_proc_addr
);

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
static DriftSkiaInitVulkanFn drift_skia_init_vulkan = NULL;
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
typedef int (*DriftSkiaRenderVulkanSyncFn)(int width, int height, uintptr_t vk_image, uint32_t vk_format);
typedef void (*DriftSkiaPurgeResourcesFn)(void);

static DriftStepAndSnapshotFn drift_step_and_snapshot = NULL;
static DriftSkiaRenderVulkanSyncFn drift_skia_render_vulkan_sync = NULL;
static DriftSkiaPurgeResourcesFn drift_skia_purge_resources = NULL;

typedef int (*DriftShouldWarmUpViewsFn)(void);
static DriftShouldWarmUpViewsFn drift_should_warm_up_views = NULL;

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

/* ─── Vulkan state ─── */
static VkInstance g_vk_instance = VK_NULL_HANDLE;
static VkPhysicalDevice g_vk_phys_device = VK_NULL_HANDLE;
static VkDevice g_vk_device = VK_NULL_HANDLE;
static VkQueue g_vk_queue = VK_NULL_HANDLE;
static uint32_t g_vk_queue_family_index = 0;
static PFN_vkGetInstanceProcAddr g_vk_get_instance_proc_addr = NULL;
static PFN_vkGetDeviceProcAddr g_vk_get_device_proc_addr = NULL;

/* ─── Double-buffered HardwareBuffer Vulkan state ─── */
#define HWB_COUNT 2

typedef struct {
    AHardwareBuffer *hwb;
    VkImage          image;
    VkDeviceMemory   memory;
    VkFence          fence;
    int              fence_submitted;  /* whether fence is pending */
} HwbSlot;

static HwbSlot g_hwb_slots[HWB_COUNT];
static int     g_hwb_current = 0;       /* index of slot to render into next */
static VkFormat g_vk_format = VK_FORMAT_R8G8B8A8_UNORM;

/* Cached Vulkan function pointers (resolved once in initVulkan) */
static PFN_vkWaitForFences      g_vk_wait_for_fences = NULL;
static PFN_vkResetFences        g_vk_reset_fences = NULL;
static PFN_vkQueueSubmit        g_vk_queue_submit = NULL;
static PFN_vkDeviceWaitIdle     g_vk_device_wait_idle = NULL;
static PFN_vkCreateImage        g_vk_create_image = NULL;
static PFN_vkDestroyImage       g_vk_destroy_image = NULL;
static PFN_vkAllocateMemory     g_vk_allocate_memory = NULL;
static PFN_vkFreeMemory         g_vk_free_memory = NULL;
static PFN_vkBindImageMemory    g_vk_bind_image_memory = NULL;
static PFN_vkCreateFence        g_vk_create_fence = NULL;
static PFN_vkDestroyFence       g_vk_destroy_fence = NULL;
static PFN_vkGetPhysicalDeviceMemoryProperties g_vk_get_phys_dev_mem_props = NULL;
static PFN_vkGetAndroidHardwareBufferPropertiesANDROID g_vk_get_ahb_props = NULL;

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
 * JNI implementation for NativeBridge.initSkiaVulkan().
 *
 * Initializes the Skia Vulkan context using the previously created Vulkan handles.
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_initSkiaVulkan(
    JNIEnv *env,
    jclass clazz
) {
    (void)env;
    (void)clazz;

    if (!g_vk_instance || !g_vk_device) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Vulkan not initialized");
        return 1;
    }

    if (resolve_symbol("DriftSkiaInitVulkan", (void **)&drift_skia_init_vulkan) != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Failed to resolve DriftSkiaInitVulkan");
        return 1;
    }

    return (jint)drift_skia_init_vulkan(
        (uintptr_t)g_vk_instance,
        (uintptr_t)g_vk_phys_device,
        (uintptr_t)g_vk_device,
        (uintptr_t)g_vk_queue,
        g_vk_queue_family_index,
        (uintptr_t)g_vk_get_instance_proc_addr
    );
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
    (void)env; (void)clazz;

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
    (void)env; (void)clazz;

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
        if (resolve_symbol("DriftSetScheduleFrameHandler", (void **)&drift_set_schedule_frame_handler) == 0) {
            drift_set_schedule_frame_handler(schedule_frame_handler);
            __android_log_print(ANDROID_LOG_INFO, "DriftJNI", "Schedule-frame handler registered");
        }
    }

    __android_log_print(ANDROID_LOG_INFO, "DriftJNI", "Platform channels initialized");
    return 0;
}

/* ═══════════════════════════════════════════════════════════════════════════
 * Unified Frame Orchestrator: Vulkan, HardwareBuffer, new JNI
 * ═══════════════════════════════════════════════════════════════════════════ */

/**
 * JNI: NativeBridge.initVulkan()
 * Creates a Vulkan instance, picks a physical device, and creates a logical
 * device with a graphics queue. Enables the
 * VK_ANDROID_external_memory_android_hardware_buffer extension for AHB import.
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_initVulkan(JNIEnv *env, jclass clazz) {
    (void)env; (void)clazz;

    g_vk_get_instance_proc_addr = (PFN_vkGetInstanceProcAddr)dlsym(RTLD_DEFAULT, "vkGetInstanceProcAddr");
    if (!g_vk_get_instance_proc_addr) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "vkGetInstanceProcAddr not found");
        return -1;
    }

    /* Create Vulkan instance */
    VkApplicationInfo appInfo = {
        .sType = VK_STRUCTURE_TYPE_APPLICATION_INFO,
        .pApplicationName = "Drift",
        .applicationVersion = VK_MAKE_VERSION(1, 0, 0),
        .pEngineName = "Drift",
        .engineVersion = VK_MAKE_VERSION(1, 0, 0),
        .apiVersion = VK_API_VERSION_1_1,
    };

    VkInstanceCreateInfo instanceCI = {
        .sType = VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO,
        .pApplicationInfo = &appInfo,
        .enabledExtensionCount = DRIFT_VK_INSTANCE_EXTENSION_COUNT,
        .ppEnabledExtensionNames = DRIFT_VK_INSTANCE_EXTENSIONS,
    };

    PFN_vkCreateInstance vkCreateInstanceFn =
        (PFN_vkCreateInstance)g_vk_get_instance_proc_addr(VK_NULL_HANDLE, "vkCreateInstance");
    if (!vkCreateInstanceFn) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "vkCreateInstance not found");
        return -1;
    }

    VkResult res = vkCreateInstanceFn(&instanceCI, NULL, &g_vk_instance);
    if (res != VK_SUCCESS) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "vkCreateInstance failed: %d", res);
        return -1;
    }

    /* Enumerate physical devices and pick the first one */
    PFN_vkEnumeratePhysicalDevices vkEnumPhys =
        (PFN_vkEnumeratePhysicalDevices)g_vk_get_instance_proc_addr(g_vk_instance, "vkEnumeratePhysicalDevices");
    uint32_t physCount = 0;
    vkEnumPhys(g_vk_instance, &physCount, NULL);
    if (physCount == 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "No Vulkan physical devices");
        return -1;
    }

    VkPhysicalDevice *physDevices = (VkPhysicalDevice *)malloc(sizeof(VkPhysicalDevice) * physCount);
    vkEnumPhys(g_vk_instance, &physCount, physDevices);
    g_vk_phys_device = physDevices[0];
    free(physDevices);

    /* Find a graphics queue family */
    PFN_vkGetPhysicalDeviceQueueFamilyProperties vkGetQueueFamilyProps =
        (PFN_vkGetPhysicalDeviceQueueFamilyProperties)g_vk_get_instance_proc_addr(
            g_vk_instance, "vkGetPhysicalDeviceQueueFamilyProperties");
    uint32_t queueFamilyCount = 0;
    vkGetQueueFamilyProps(g_vk_phys_device, &queueFamilyCount, NULL);

    VkQueueFamilyProperties *queueFamilies =
        (VkQueueFamilyProperties *)malloc(sizeof(VkQueueFamilyProperties) * queueFamilyCount);
    vkGetQueueFamilyProps(g_vk_phys_device, &queueFamilyCount, queueFamilies);

    g_vk_queue_family_index = UINT32_MAX;
    for (uint32_t i = 0; i < queueFamilyCount; i++) {
        if (queueFamilies[i].queueFlags & VK_QUEUE_GRAPHICS_BIT) {
            g_vk_queue_family_index = i;
            break;
        }
    }
    free(queueFamilies);

    if (g_vk_queue_family_index == UINT32_MAX) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "No graphics queue family found");
        return -1;
    }

    /* Create logical device with required extensions */
    float queuePriority = 1.0f;
    VkDeviceQueueCreateInfo queueCI = {
        .sType = VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO,
        .queueFamilyIndex = g_vk_queue_family_index,
        .queueCount = 1,
        .pQueuePriorities = &queuePriority,
    };

    VkDeviceCreateInfo deviceCI = {
        .sType = VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO,
        .queueCreateInfoCount = 1,
        .pQueueCreateInfos = &queueCI,
        .enabledExtensionCount = DRIFT_VK_DEVICE_EXTENSION_COUNT,
        .ppEnabledExtensionNames = DRIFT_VK_DEVICE_EXTENSIONS,
    };

    PFN_vkCreateDevice vkCreateDeviceFn =
        (PFN_vkCreateDevice)g_vk_get_instance_proc_addr(g_vk_instance, "vkCreateDevice");
    res = vkCreateDeviceFn(g_vk_phys_device, &deviceCI, NULL, &g_vk_device);
    if (res != VK_SUCCESS) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "vkCreateDevice failed: %d", res);
        return -1;
    }

    /* Resolve vkGetDeviceProcAddr for device-level function lookups */
    g_vk_get_device_proc_addr = (PFN_vkGetDeviceProcAddr)g_vk_get_instance_proc_addr(g_vk_instance, "vkGetDeviceProcAddr");
    if (!g_vk_get_device_proc_addr) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "vkGetDeviceProcAddr not found");
        return -1;
    }

    PFN_vkGetDeviceQueue vkGetDeviceQueueFn =
        (PFN_vkGetDeviceQueue)g_vk_get_device_proc_addr(g_vk_device, "vkGetDeviceQueue");
    vkGetDeviceQueueFn(g_vk_device, g_vk_queue_family_index, 0, &g_vk_queue);

    /* Cache device-level Vulkan function pointers for per-frame and resource operations */
    g_vk_wait_for_fences = (PFN_vkWaitForFences)g_vk_get_device_proc_addr(g_vk_device, "vkWaitForFences");
    g_vk_reset_fences = (PFN_vkResetFences)g_vk_get_device_proc_addr(g_vk_device, "vkResetFences");
    g_vk_queue_submit = (PFN_vkQueueSubmit)g_vk_get_device_proc_addr(g_vk_device, "vkQueueSubmit");
    g_vk_device_wait_idle = (PFN_vkDeviceWaitIdle)g_vk_get_device_proc_addr(g_vk_device, "vkDeviceWaitIdle");
    g_vk_create_image = (PFN_vkCreateImage)g_vk_get_device_proc_addr(g_vk_device, "vkCreateImage");
    g_vk_destroy_image = (PFN_vkDestroyImage)g_vk_get_device_proc_addr(g_vk_device, "vkDestroyImage");
    g_vk_allocate_memory = (PFN_vkAllocateMemory)g_vk_get_device_proc_addr(g_vk_device, "vkAllocateMemory");
    g_vk_free_memory = (PFN_vkFreeMemory)g_vk_get_device_proc_addr(g_vk_device, "vkFreeMemory");
    g_vk_bind_image_memory = (PFN_vkBindImageMemory)g_vk_get_device_proc_addr(g_vk_device, "vkBindImageMemory");
    g_vk_create_fence = (PFN_vkCreateFence)g_vk_get_device_proc_addr(g_vk_device, "vkCreateFence");
    g_vk_destroy_fence = (PFN_vkDestroyFence)g_vk_get_device_proc_addr(g_vk_device, "vkDestroyFence");
    /* Instance-level: physical device queries are resolved via instance proc addr */
    g_vk_get_phys_dev_mem_props = (PFN_vkGetPhysicalDeviceMemoryProperties)g_vk_get_instance_proc_addr(g_vk_instance, "vkGetPhysicalDeviceMemoryProperties");
    g_vk_get_ahb_props = (PFN_vkGetAndroidHardwareBufferPropertiesANDROID)g_vk_get_device_proc_addr(g_vk_device, "vkGetAndroidHardwareBufferPropertiesANDROID");

    __android_log_print(ANDROID_LOG_INFO, "DriftJNI", "Vulkan initialized: queue family %u", g_vk_queue_family_index);
    return 0;
}

/**
 * Helper: allocate a single HWB slot (AHardwareBuffer + VkImage + VkDeviceMemory + VkFence).
 * Returns 0 on success, -1 on failure. On failure the slot is left zeroed.
 */
static int create_hwb_slot(HwbSlot *slot, int width, int height) {
    memset(slot, 0, sizeof(*slot));
    VkResult res;

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
    if (AHardwareBuffer_allocate(&desc, &slot->hwb) != 0) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "AHardwareBuffer_allocate failed");
        return -1;
    }

    /* Get VkFormat and memory requirements from the AHardwareBuffer */
    if (!g_vk_get_ahb_props) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "vkGetAndroidHardwareBufferPropertiesANDROID not found");
        goto fail_hwb;
    }

    VkAndroidHardwareBufferFormatPropertiesANDROID formatProps = {
        .sType = VK_STRUCTURE_TYPE_ANDROID_HARDWARE_BUFFER_FORMAT_PROPERTIES_ANDROID,
    };
    VkAndroidHardwareBufferPropertiesANDROID ahbProps = {
        .sType = VK_STRUCTURE_TYPE_ANDROID_HARDWARE_BUFFER_PROPERTIES_ANDROID,
        .pNext = &formatProps,
    };
    res = g_vk_get_ahb_props(g_vk_device, slot->hwb, &ahbProps);
    if (res != VK_SUCCESS) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "vkGetAndroidHardwareBufferPropertiesANDROID failed: %d", res);
        goto fail_hwb;
    }

    g_vk_format = formatProps.format;

    /* Create VkImage backed by the AHardwareBuffer */
    VkExternalMemoryImageCreateInfo extMemCI = {
        .sType = VK_STRUCTURE_TYPE_EXTERNAL_MEMORY_IMAGE_CREATE_INFO,
        .handleTypes = VK_EXTERNAL_MEMORY_HANDLE_TYPE_ANDROID_HARDWARE_BUFFER_BIT_ANDROID,
    };

    VkImageCreateInfo imageCI = {
        .sType = VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO,
        .pNext = &extMemCI,
        .imageType = VK_IMAGE_TYPE_2D,
        .format = g_vk_format,
        .extent = { (uint32_t)width, (uint32_t)height, 1 },
        .mipLevels = 1,
        .arrayLayers = 1,
        .samples = VK_SAMPLE_COUNT_1_BIT,
        .tiling = VK_IMAGE_TILING_OPTIMAL,
        .usage = VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT |
                 VK_IMAGE_USAGE_TRANSFER_SRC_BIT |
                 VK_IMAGE_USAGE_TRANSFER_DST_BIT,
        .sharingMode = VK_SHARING_MODE_EXCLUSIVE,
        .initialLayout = VK_IMAGE_LAYOUT_UNDEFINED,
    };

    res = g_vk_create_image(g_vk_device, &imageCI, NULL, &slot->image);
    if (res != VK_SUCCESS) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "vkCreateImage failed: %d", res);
        goto fail_hwb;
    }

    /* Allocate and bind memory from the AHardwareBuffer */
    VkImportAndroidHardwareBufferInfoANDROID importInfo = {
        .sType = VK_STRUCTURE_TYPE_IMPORT_ANDROID_HARDWARE_BUFFER_INFO_ANDROID,
        .buffer = slot->hwb,
    };

    VkMemoryDedicatedAllocateInfo dedicatedInfo = {
        .sType = VK_STRUCTURE_TYPE_MEMORY_DEDICATED_ALLOCATE_INFO,
        .pNext = &importInfo,
        .image = slot->image,
    };

    VkPhysicalDeviceMemoryProperties memProps;
    g_vk_get_phys_dev_mem_props(g_vk_phys_device, &memProps);

    uint32_t memoryTypeIndex = UINT32_MAX;
    for (uint32_t i = 0; i < memProps.memoryTypeCount; i++) {
        if (ahbProps.memoryTypeBits & (1u << i)) {
            memoryTypeIndex = i;
            break;
        }
    }
    if (memoryTypeIndex == UINT32_MAX) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "No compatible memory type for AHB");
        goto fail_image;
    }

    VkMemoryAllocateInfo allocInfo = {
        .sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,
        .pNext = &dedicatedInfo,
        .allocationSize = ahbProps.allocationSize,
        .memoryTypeIndex = memoryTypeIndex,
    };

    res = g_vk_allocate_memory(g_vk_device, &allocInfo, NULL, &slot->memory);
    if (res != VK_SUCCESS) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "vkAllocateMemory failed: %d", res);
        goto fail_image;
    }

    res = g_vk_bind_image_memory(g_vk_device, slot->image, slot->memory, 0);
    if (res != VK_SUCCESS) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "vkBindImageMemory failed: %d", res);
        goto fail_memory;
    }

    /* Create VkFence (signaled initially so first wait is a no-op) */
    VkFenceCreateInfo fenceCI = {
        .sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO,
        .flags = VK_FENCE_CREATE_SIGNALED_BIT,
    };
    res = g_vk_create_fence(g_vk_device, &fenceCI, NULL, &slot->fence);
    if (res != VK_SUCCESS) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "vkCreateFence failed: %d", res);
        goto fail_memory;
    }
    slot->fence_submitted = 0;

    return 0;

fail_memory:
    g_vk_free_memory(g_vk_device, slot->memory, NULL);
    slot->memory = VK_NULL_HANDLE;
fail_image:
    g_vk_destroy_image(g_vk_device, slot->image, NULL);
    slot->image = VK_NULL_HANDLE;
fail_hwb:
    AHardwareBuffer_release(slot->hwb);
    slot->hwb = NULL;
    return -1;
}

/**
 * Helper: destroy a single HWB slot.
 */
static void destroy_hwb_slot(HwbSlot *slot) {
    if (!slot) return;
    if (g_vk_device) {
        if (slot->fence != VK_NULL_HANDLE && g_vk_destroy_fence) {
            g_vk_destroy_fence(g_vk_device, slot->fence, NULL);
        }
        if (slot->image != VK_NULL_HANDLE && g_vk_destroy_image) {
            g_vk_destroy_image(g_vk_device, slot->image, NULL);
        }
        if (slot->memory != VK_NULL_HANDLE && g_vk_free_memory) {
            g_vk_free_memory(g_vk_device, slot->memory, NULL);
        }
    }
    if (slot->hwb) { AHardwareBuffer_release(slot->hwb); }
    memset(slot, 0, sizeof(*slot));
}

/**
 * JNI: NativeBridge.createHwbResources(width, height)
 * Allocates two AHardwareBuffers and imports each as a VkImage via
 * VK_ANDROID_external_memory_android_hardware_buffer. Creates a VkFence
 * per slot for double-buffered rendering.
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_createHwbResources(JNIEnv *env, jclass clazz, jint width, jint height) {
    (void)env; (void)clazz;

    if (!g_vk_device) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "Vulkan device not initialized");
        return -1;
    }

    for (int i = 0; i < HWB_COUNT; i++) {
        if (create_hwb_slot(&g_hwb_slots[i], width, height) != 0) {
            /* Clean up any slots already created */
            for (int j = 0; j < i; j++) {
                destroy_hwb_slot(&g_hwb_slots[j]);
            }
            return -1;
        }
    }

    g_hwb_current = 0;

    __android_log_print(ANDROID_LOG_INFO, "DriftJNI", "HWB Vulkan resources created (double-buffered): %dx%d format=%u",
        width, height, g_vk_format);
    return 0;
}

/**
 * JNI: NativeBridge.destroyHwbResources()
 * Waits for the GPU to idle, then destroys both buffer slots.
 */
JNIEXPORT void JNICALL
Java_{{.JNIPackage}}_NativeBridge_destroyHwbResources(JNIEnv *env, jclass clazz) {
    (void)env; (void)clazz;

    if (g_vk_device && g_vk_device_wait_idle) {
        g_vk_device_wait_idle(g_vk_device);
    }

    for (int i = 0; i < HWB_COUNT; i++) {
        destroy_hwb_slot(&g_hwb_slots[i]);
    }
    g_hwb_current = 0;
}

/**
 * JNI: NativeBridge.getHardwareBuffer(index)
 * Returns the AHardwareBuffer for the given slot as a Java HardwareBuffer object.
 * Used by SkiaHostView to wrap each slot as a Bitmap for HWUI onDraw().
 */
JNIEXPORT jobject JNICALL
Java_{{.JNIPackage}}_NativeBridge_getHardwareBuffer(JNIEnv *env, jclass clazz, jint index) {
    (void)clazz;
    if (index < 0 || index >= HWB_COUNT) return NULL;
    AHardwareBuffer *hwb = g_hwb_slots[index].hwb;
    if (!hwb) return NULL;
    return AHardwareBuffer_toHardwareBuffer(env, hwb);
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
    if (result != 0 || !outData || outLen <= 0) {
        free(outData);
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
 * Double-buffered: picks the next slot, waits on its fence (from two frames ago),
 * renders into that slot's VkImage, then submits a fence for this frame.
 * Returns the slot index rendered into (0 or 1), or -1 on error.
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_renderFrameSync(JNIEnv *env, jclass clazz, jint width, jint height) {
    (void)env; (void)clazz;

    if (resolve_symbol("DriftSkiaRenderVulkanSync", (void **)&drift_skia_render_vulkan_sync) != 0) {
        return -1;
    }

    int slot_idx = g_hwb_current;
    HwbSlot *slot = &g_hwb_slots[slot_idx];

    /* Wait on this slot's fence (ensures GPU finished the frame that last used it).
     * Use a finite timeout to avoid hanging forever if the GPU stalls (e.g. during
     * app backgrounding on some devices). 1 second is generous for a single frame. */
    if (slot->fence_submitted) {
        if (g_vk_wait_for_fences && g_vk_reset_fences) {
            static const uint64_t FENCE_TIMEOUT_NS = 1000000000ULL; /* 1 second */
            VkResult fence_res = g_vk_wait_for_fences(g_vk_device, 1, &slot->fence, VK_TRUE, FENCE_TIMEOUT_NS);
            if (fence_res == VK_TIMEOUT) {
                __android_log_print(ANDROID_LOG_WARN, "DriftJNI", "Fence wait timed out on slot %d, resetting device", slot_idx);
                if (g_vk_device_wait_idle) {
                    g_vk_device_wait_idle(g_vk_device);
                }
            } else if (fence_res != VK_SUCCESS) {
                __android_log_print(ANDROID_LOG_ERROR, "DriftJNI", "vkWaitForFences failed: %d", fence_res);
                return -1;
            }
            g_vk_reset_fences(g_vk_device, 1, &slot->fence);
        }
        slot->fence_submitted = 0;
    }

    /* Render into this slot's VkImage */
    int result = drift_skia_render_vulkan_sync(width, height, (uintptr_t)slot->image, (uint32_t)g_vk_format);
    if (result != 0) {
        return -1;
    }

    /* Submit an empty batch with just the fence to track GPU completion */
    if (g_vk_queue_submit) {
        VkSubmitInfo submitInfo = {
            .sType = VK_STRUCTURE_TYPE_SUBMIT_INFO,
        };
        VkResult res = g_vk_queue_submit(g_vk_queue, 1, &submitInfo, slot->fence);
        if (res == VK_SUCCESS) {
            slot->fence_submitted = 1;
        } else {
            __android_log_print(ANDROID_LOG_WARN, "DriftJNI", "vkQueueSubmit fence failed: %d", res);
        }
    }

    /* Advance to next slot */
    g_hwb_current = (g_hwb_current + 1) % HWB_COUNT;

    return (jint)slot_idx;
}

/**
 * JNI: NativeBridge.purgeResources()
 * Releases all cached GPU resources.
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

/**
 * JNI: NativeBridge.shouldWarmUpViews()
 * Returns 1 if the Go engine wants platform views to be pre-warmed at startup,
 * 0 if warmup has been disabled via engine.DisableViewWarmup().
 */
JNIEXPORT jint JNICALL
Java_{{.JNIPackage}}_NativeBridge_shouldWarmUpViews(JNIEnv *env, jclass clazz) {
    (void)env; (void)clazz;

    if (resolve_symbol("DriftShouldWarmUpViews", (void **)&drift_should_warm_up_views) != 0) {
        return 1; /* Fail-safe: warm up if we can't check */
    }

    return (jint)drift_should_warm_up_views();
}
