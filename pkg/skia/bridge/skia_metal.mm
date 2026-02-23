// Drift Skia Metal bridge for iOS/macOS
// Pre-compiled at CI time, not by CGO

#import <Metal/Metal.h>
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
#include "gpu/ganesh/mtl/GrMtlBackendContext.h"
#include "gpu/ganesh/mtl/GrMtlBackendSurface.h"
#include "gpu/ganesh/mtl/GrMtlDirectContext.h"
#include "ports/SkFontMgr_mac_ct.h"
#include "skia_path_impl.h"
#include "skia_skottie_impl.h"
#include "skia_svg_impl.h"

#ifdef __cplusplus
extern "C" {
#endif
#include "../skia_bridge.h"
#ifdef __cplusplus
}
#endif

namespace {

sk_sp<SkFontMgr> get_font_manager() {
    static std::once_flag once;
    static sk_sp<SkFontMgr> manager;
    std::call_once(once, [] {
        manager = SkFontMgr_New_CoreText(nullptr);
        if (!manager) {
            manager = SkFontMgr::RefEmpty();
        }
    });
    return manager;
}

#define DRIFT_PLATFORM_FALLBACK_FONT "SF Pro Text"
#include "skia_common_impl.h"

}  // namespace

// Provide a weak definition for the default font families used by skparagraph.
// This allows the paragraph module to fall back to our configured default font
// when no explicit font family is specified in the text style.
const std::vector<SkString>* ::skia::textlayout::TextStyle::kDefaultFontFamilies __attribute__((weak)) = &textlayout_defaults::kDefaultFontFamilies;

DRIFT_SKIA_DEFINE_COMMON_FUNCTIONS

extern "C" {

DriftSkiaContext drift_skia_context_create_metal(void* device, void* queue) {
    if (!device || !queue) {
        return nullptr;
    }
    GrMtlBackendContext backend;
    // Cast to const void* for SkCFObject::retain() - the objects are already retained by the caller
    backend.fDevice.retain((const void*)device);
    backend.fQueue.retain((const void*)queue);
    auto context = GrDirectContexts::MakeMetal(backend);
    if (!context) {
        return nullptr;
    }
    return context.release();
}

void drift_skia_context_destroy(DriftSkiaContext ctx) {
    if (!ctx) {
        return;
    }
    reinterpret_cast<GrDirectContext*>(ctx)->unref();
}

DriftSkiaSurface drift_skia_surface_create_metal(DriftSkiaContext ctx, void* texture, int width, int height) {
    if (!ctx || !texture || width <= 0 || height <= 0) {
        return nullptr;
    }

    GrMtlTextureInfo texture_info;
    texture_info.fTexture.retain((const void*)texture);

    GrBackendRenderTarget backend_target = GrBackendRenderTargets::MakeMtl(
        width,
        height,
        texture_info
    );
    SkSurfaceProps props(0, kRGB_H_SkPixelGeometry);

    auto surface = SkSurfaces::WrapBackendRenderTarget(
        reinterpret_cast<GrDirectContext*>(ctx),
        backend_target,
        kTopLeft_GrSurfaceOrigin,
        kRGBA_8888_SkColorType,
        SkColorSpace::MakeSRGB(),
        &props
    );

    if (!surface) {
        return nullptr;
    }

    return surface.release();
}

void drift_skia_surface_flush(DriftSkiaContext ctx, DriftSkiaSurface surface) {
    if (!ctx || !surface) {
        return;
    }
    auto sk_surface = reinterpret_cast<SkSurface*>(surface);
    // Use GrSyncCpu::kNo to let GPU work pipeline naturally with triple buffering.
    // GrSyncCpu::kYes causes CPU stalls during rapid interaction (flickering).
    reinterpret_cast<GrDirectContext*>(ctx)->flushAndSubmit(sk_surface, GrSyncCpu::kNo);
}

DriftSkiaContext drift_skia_context_create_vulkan(
    uintptr_t instance, uintptr_t phys_device, uintptr_t device,
    uintptr_t queue, uint32_t queue_family_index, uintptr_t get_instance_proc_addr
) {
    (void)instance; (void)phys_device; (void)device;
    (void)queue; (void)queue_family_index; (void)get_instance_proc_addr;
    return nullptr;
}

DriftSkiaSurface drift_skia_surface_create_vulkan(
    DriftSkiaContext ctx, int width, int height, uintptr_t vk_image, uint32_t vk_format
) {
    (void)ctx; (void)width; (void)height; (void)vk_image; (void)vk_format;
    return nullptr;
}

DriftSkiaSurface drift_skia_surface_create_offscreen_vulkan(DriftSkiaContext ctx, int width, int height) {
    (void)ctx; (void)width; (void)height;
    return nullptr;
}

DriftSkiaSurface drift_skia_surface_create_offscreen_metal(DriftSkiaContext ctx, int width, int height) {
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
