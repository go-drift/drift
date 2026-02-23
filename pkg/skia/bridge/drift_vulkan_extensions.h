/**
 * Vulkan extension arrays shared between the Skia bridge (skia_vk.cc) and
 * the Android JNI bridge (drift_jni.c).  Both files must enable the same
 * extensions so the VkDevice created in drift_jni.c is compatible with the
 * GrDirectContext created in skia_vk.cc.
 *
 * KEEP IN SYNC: this file is duplicated in two locations because the Skia
 * bridge and the JNI bridge are compiled by different build systems with no
 * shared include path:
 *
 *   pkg/skia/bridge/drift_vulkan_extensions.h          (Skia CI build)
 *   cmd/drift/internal/templates/android/cpp/drift_vulkan_extensions.h  (NDK build)
 *
 * When modifying this file, update BOTH copies.
 */

#ifndef DRIFT_VULKAN_EXTENSIONS_H
#define DRIFT_VULKAN_EXTENSIONS_H

#include <vulkan/vulkan.h>

static const char *const DRIFT_VK_INSTANCE_EXTENSIONS[] = {
    VK_KHR_EXTERNAL_MEMORY_CAPABILITIES_EXTENSION_NAME,
    VK_KHR_GET_PHYSICAL_DEVICE_PROPERTIES_2_EXTENSION_NAME,
};

static const char *const DRIFT_VK_DEVICE_EXTENSIONS[] = {
    VK_KHR_EXTERNAL_MEMORY_EXTENSION_NAME,
    VK_EXT_QUEUE_FAMILY_FOREIGN_EXTENSION_NAME,
    VK_ANDROID_EXTERNAL_MEMORY_ANDROID_HARDWARE_BUFFER_EXTENSION_NAME,
    VK_KHR_SAMPLER_YCBCR_CONVERSION_EXTENSION_NAME,
    VK_KHR_MAINTENANCE1_EXTENSION_NAME,
    VK_KHR_BIND_MEMORY_2_EXTENSION_NAME,
    VK_KHR_GET_MEMORY_REQUIREMENTS_2_EXTENSION_NAME,
    VK_KHR_DEDICATED_ALLOCATION_EXTENSION_NAME,
};

#define DRIFT_VK_INSTANCE_EXTENSION_COUNT \
    (sizeof(DRIFT_VK_INSTANCE_EXTENSIONS) / sizeof(DRIFT_VK_INSTANCE_EXTENSIONS[0]))

#define DRIFT_VK_DEVICE_EXTENSION_COUNT \
    (sizeof(DRIFT_VK_DEVICE_EXTENSIONS) / sizeof(DRIFT_VK_DEVICE_EXTENSIONS[0]))

#endif /* DRIFT_VULKAN_EXTENSIONS_H */
