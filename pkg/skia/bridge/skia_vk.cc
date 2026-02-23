// Drift Skia Vulkan bridge for Android
// Pre-compiled at CI time, not by CGO

#include "../skia_bridge.h"

#include <android/log.h>
#include <algorithm>
#include <cstddef>
#include <cstring>
#include <limits>
#include <mutex>
#include <string>
#include <unordered_map>
#include <vector>

#include "core/SkCanvas.h"
#include "core/SkColor.h"
#include "core/SkColorSpace.h"
#include "core/SkData.h"
#include "core/SkFont.h"
#include "core/SkFontMetrics.h"
#include "core/SkImage.h"
#include "core/SkImageInfo.h"
#include "core/SkPaint.h"
#include "core/SkPathBuilder.h"
#include "effects/SkGradient.h"
#include "effects/SkDashPathEffect.h"
#include "core/SkBlurTypes.h"
#include "core/SkMaskFilter.h"
#include "core/SkRRect.h"
#include "core/SkScalar.h"
#include "core/SkSurface.h"
#include "effects/SkImageFilters.h"
#include "core/SkColorFilter.h"
#include "core/SkSurfaceProps.h"
#include "core/SkTypeface.h"
#include "core/SkFontMgr.h"
#include "core/SkString.h"
#include "modules/skparagraph/include/FontCollection.h"
#include "modules/skparagraph/include/Paragraph.h"
#include "modules/skparagraph/include/ParagraphBuilder.h"
#include "modules/skparagraph/include/ParagraphStyle.h"
#include "modules/skparagraph/include/TextStyle.h"
#include "modules/skunicode/include/SkUnicode_libgrapheme.h"
#include "gpu/ganesh/GrBackendSurface.h"
#include "gpu/ganesh/GrDirectContext.h"
#include "gpu/ganesh/SkSurfaceGanesh.h"
#include "gpu/GpuTypes.h"
#include "gpu/vk/VulkanBackendContext.h"
#include "gpu/vk/VulkanExtensions.h"
#include "gpu/ganesh/vk/GrVkBackendSurface.h"
#include "gpu/ganesh/vk/GrVkDirectContext.h"
#include "gpu/ganesh/vk/GrVkTypes.h"
#include "ports/SkFontMgr_android.h"
#include "ports/SkFontMgr_android_ndk.h"
#include "ports/SkFontScanner_FreeType.h"
#include "skia_path_impl.h"
#include "skia_skottie_impl.h"
#include "skia_svg_impl.h"

#include <vulkan/vulkan.h>
#include "drift_vulkan_extensions.h"

namespace {

sk_sp<SkFontMgr> get_font_manager();

sk_sp<SkFontMgr> get_font_manager() {
    static std::once_flag once;
    static sk_sp<SkFontMgr> manager;
    std::call_once(once, [] {
        auto scanner = SkFontScanner_Make_FreeType();
        manager = SkFontMgr_New_AndroidNDK(true, std::move(scanner));
        if (!manager) {
            manager = SkFontMgr_New_Android(nullptr, SkFontScanner_Make_FreeType());
        }
        if (!manager) {
            manager = SkFontMgr::RefEmpty();
        }
        if (manager) {
            int families = manager->countFamilies();
            __android_log_print(ANDROID_LOG_INFO, "DriftSkia", "Font manager ready, families=%d", families);
        } else {
            __android_log_print(ANDROID_LOG_ERROR, "DriftSkia", "Font manager init failed");
        }
    });
    return manager;
}

#define DRIFT_PLATFORM_FALLBACK_FONT "sans-serif"
#include "skia_common_impl.h"

}  // namespace

// Provide a weak definition for the default font families used by skparagraph.
const std::vector<SkString>* ::skia::textlayout::TextStyle::kDefaultFontFamilies __attribute__((weak)) = &textlayout_defaults::kDefaultFontFamilies;

// Shared function definitions (canvas, paint, text, paragraph, path, SVG, Skottie, etc.)
DRIFT_SKIA_DEFINE_COMMON_FUNCTIONS

// ═══════════════════════════════════════════════════════════════════════════
// Vulkan-specific functions
// ═══════════════════════════════════════════════════════════════════════════

extern "C" {

DriftSkiaContext drift_skia_context_create_metal(void* device, void* queue) {
    (void)device;
    (void)queue;
    return nullptr;
}

DriftSkiaContext drift_skia_context_create_vulkan(
    uintptr_t instance,
    uintptr_t phys_device,
    uintptr_t device,
    uintptr_t queue,
    uint32_t queue_family_index,
    uintptr_t get_instance_proc_addr
) {
    auto vkInstance = reinterpret_cast<VkInstance>(instance);
    auto vkPhysDevice = reinterpret_cast<VkPhysicalDevice>(phys_device);
    auto vkDevice = reinterpret_cast<VkDevice>(device);

    auto vkGetInstanceProc = reinterpret_cast<PFN_vkGetInstanceProcAddr>(get_instance_proc_addr);
    if (!vkGetInstanceProc) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftSkia", "vkGetInstanceProcAddr is null");
        return nullptr;
    }

    PFN_vkGetDeviceProcAddr vkGetDeviceProc =
        reinterpret_cast<PFN_vkGetDeviceProcAddr>(
            vkGetInstanceProc(vkInstance, "vkGetDeviceProcAddr"));

    skgpu::VulkanGetProc getProc = [vkGetInstanceProc, vkGetDeviceProc, vkInstance](
            const char* name, VkInstance inst, VkDevice dev) -> PFN_vkVoidFunction {
        PFN_vkVoidFunction fn = nullptr;
        if (dev != VK_NULL_HANDLE && vkGetDeviceProc) {
            fn = vkGetDeviceProc(dev, name);
            if (fn) return fn;
        }
        // For global functions (both null), use VK_NULL_HANDLE.
        // For instance functions, use the provided instance.
        // For device functions that fell through, use the captured instance.
        VkInstance resolveInst = inst;
        if (inst == VK_NULL_HANDLE && dev != VK_NULL_HANDLE) {
            resolveInst = vkInstance;
        }
        fn = vkGetInstanceProc(resolveInst, name);
        return fn;
    };

    skgpu::VulkanExtensions extensions;
    extensions.init(getProc, vkInstance, vkPhysDevice,
                    DRIFT_VK_INSTANCE_EXTENSION_COUNT, DRIFT_VK_INSTANCE_EXTENSIONS,
                    DRIFT_VK_DEVICE_EXTENSION_COUNT, DRIFT_VK_DEVICE_EXTENSIONS);

    // Query physical device features so Skia knows what's available.
    VkPhysicalDeviceFeatures2 deviceFeatures2 = {};
    deviceFeatures2.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_FEATURES_2;
    auto vkGetFeatures2 = reinterpret_cast<PFN_vkGetPhysicalDeviceFeatures2>(
        vkGetInstanceProc(vkInstance, "vkGetPhysicalDeviceFeatures2"));
    if (vkGetFeatures2) {
        vkGetFeatures2(vkPhysDevice, &deviceFeatures2);
    }

    skgpu::VulkanBackendContext backend;
    backend.fInstance = vkInstance;
    backend.fPhysicalDevice = vkPhysDevice;
    backend.fDevice = vkDevice;
    backend.fQueue = reinterpret_cast<VkQueue>(queue);
    backend.fGraphicsQueueIndex = queue_family_index;
    backend.fMaxAPIVersion = VK_API_VERSION_1_1;
    backend.fVkExtensions = &extensions;
    backend.fDeviceFeatures2 = &deviceFeatures2;
    backend.fGetProc = getProc;

    auto context = GrDirectContexts::MakeVulkan(backend);
    if (!context) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftSkia", "Failed to create Vulkan GrDirectContext");
        return nullptr;
    }
    __android_log_print(ANDROID_LOG_INFO, "DriftSkia", "Vulkan GrDirectContext created");
    return context.release();
}

void drift_skia_context_destroy(DriftSkiaContext ctx) {
    if (!ctx) {
        return;
    }
    reinterpret_cast<GrDirectContext*>(ctx)->unref();
}

DriftSkiaSurface drift_skia_surface_create_metal(DriftSkiaContext ctx, void* texture, int width, int height) {
    (void)ctx;
    (void)texture;
    (void)width;
    (void)height;
    return nullptr;
}

DriftSkiaSurface drift_skia_surface_create_vulkan(
    DriftSkiaContext ctx,
    int width, int height,
    uintptr_t vk_image,
    uint32_t vk_format
) {
    if (!ctx || width <= 0 || height <= 0 || !vk_image) {
        return nullptr;
    }

    auto context = reinterpret_cast<GrDirectContext*>(ctx);

    GrVkImageInfo imageInfo;
    VkImage image{};
    static_assert(sizeof(vk_image) <= sizeof(image), "VkImage must be at least as wide as uintptr_t");
    std::memcpy(&image, &vk_image, sizeof(vk_image));
    imageInfo.fImage = image;
    imageInfo.fImageTiling = VK_IMAGE_TILING_OPTIMAL;
    imageInfo.fImageLayout = VK_IMAGE_LAYOUT_UNDEFINED;
    imageInfo.fFormat = static_cast<VkFormat>(vk_format);
    imageInfo.fImageUsageFlags = VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT |
                                 VK_IMAGE_USAGE_TRANSFER_SRC_BIT |
                                 VK_IMAGE_USAGE_TRANSFER_DST_BIT;
    imageInfo.fSampleCount = 1;
    imageInfo.fLevelCount = 1;
    imageInfo.fCurrentQueueFamily = VK_QUEUE_FAMILY_IGNORED;

    GrBackendRenderTarget backend_target = GrBackendRenderTargets::MakeVk(
        width,
        height,
        imageInfo
    );

    SkSurfaceProps props(0, kRGB_H_SkPixelGeometry);

    auto surface = SkSurfaces::WrapBackendRenderTarget(
        context,
        backend_target,
        kTopLeft_GrSurfaceOrigin,
        kRGBA_8888_SkColorType,
        SkColorSpace::MakeSRGB(),
        &props
    );

    if (!surface) {
        __android_log_print(ANDROID_LOG_ERROR, "DriftSkia", "Failed to create Vulkan surface: %dx%d format=%u", width, height, vk_format);
        return nullptr;
    }

    return surface.release();
}

void drift_skia_surface_flush(DriftSkiaContext ctx, DriftSkiaSurface surface) {
    if (!ctx || !surface) {
        return;
    }
    auto sk_surface = reinterpret_cast<SkSurface*>(surface);
    // Double-buffered: VkFence tracking in the JNI layer handles GPU completion.
    // No CPU wait needed here; the fence only blocks when reusing a slot.
    reinterpret_cast<GrDirectContext*>(ctx)->flushAndSubmit(sk_surface, GrSyncCpu::kNo);
}

DriftSkiaSurface drift_skia_surface_create_offscreen_metal(DriftSkiaContext ctx, int width, int height) {
    (void)ctx;
    (void)width;
    (void)height;
    return nullptr;
}

DriftSkiaSurface drift_skia_surface_create_offscreen_vulkan(DriftSkiaContext ctx, int width, int height) {
    if (!ctx || width <= 0 || height <= 0) {
        return nullptr;
    }
    auto context = reinterpret_cast<GrDirectContext*>(ctx);
    SkImageInfo info = SkImageInfo::Make(width, height, kRGBA_8888_SkColorType, kPremul_SkAlphaType, SkColorSpace::MakeSRGB());
    SkSurfaceProps props(0, kRGB_H_SkPixelGeometry);
    auto surface = SkSurfaces::RenderTarget(context, skgpu::Budgeted::kNo, info, 0, kTopLeft_GrSurfaceOrigin, &props);
    if (!surface) {
        return nullptr;
    }
    return surface.release();
}

void drift_skia_context_purge_resources(DriftSkiaContext ctx) {
    if (!ctx) {
        return;
    }
    auto context = reinterpret_cast<GrDirectContext*>(ctx);
    context->freeGpuResources();
}

}  // extern "C"
