#ifndef DRIFT_SKIA_SVG_IMPL_H
#define DRIFT_SKIA_SVG_IMPL_H

#include "../skia_bridge.h"
#include "core/SkCanvas.h"
#include "core/SkColor.h"
#include "core/SkColorFilter.h"
#include "core/SkData.h"
#include "core/SkStream.h"
#include "modules/svg/include/SkSVGDOM.h"
#include "modules/svg/include/SkSVGNode.h"
#include "modules/svg/include/SkSVGRenderContext.h"
#include "modules/svg/include/SkSVGTypes.h"
#include "modules/skresources/include/SkResources.h"

namespace drift_skia_svg_impl {
inline SkColor to_sk_color(uint32_t argb) {
    return SkColorSetARGB(
        (argb >> 24) & 0xFF,
        (argb >> 16) & 0xFF,
        (argb >> 8) & 0xFF,
        argb & 0xFF
    );
}
}  // namespace drift_skia_svg_impl

inline DriftSkiaSVGDOM drift_skia_svg_dom_create_with_base_impl(const uint8_t* data, int length, const char* base_path) {
    if (!data || length <= 0) return nullptr;

    // CRITICAL: Copy the data - Go memory may be moved/freed after cgo call returns
    sk_sp<SkData> skData = SkData::MakeWithCopy(data, static_cast<size_t>(length));
    if (!skData) return nullptr;

    auto stream = SkMemoryStream::Make(skData);
    if (!stream) return nullptr;

    // Build resource provider only if we have a valid base path.
    // FileResourceProvider::Make returns nullptr for empty/invalid paths,
    // which would crash DataURIResourceProviderProxy::Make on iOS.
    sk_sp<skresources::ResourceProvider> resourceProvider;
    if (base_path && base_path[0] != '\0') {
        SkString basePath(base_path);
        auto fileProvider = skresources::FileResourceProvider::Make(basePath);
        if (fileProvider) {
            resourceProvider = skresources::DataURIResourceProviderProxy::Make(
                std::move(fileProvider),
                skresources::ImageDecodeStrategy::kPreDecode
            );
        }
    }

    auto builder = SkSVGDOM::Builder();
    if (resourceProvider) {
        builder.setResourceProvider(std::move(resourceProvider));
    }
    auto dom = builder.make(*stream);

    return dom ? dom.release() : nullptr;
}

inline DriftSkiaSVGDOM drift_skia_svg_dom_create_impl(const uint8_t* data, int length) {
    return drift_skia_svg_dom_create_with_base_impl(data, length, nullptr);
}

inline void drift_skia_svg_dom_destroy_impl(DriftSkiaSVGDOM svg) {
    if (svg) reinterpret_cast<SkSVGDOM*>(svg)->unref();
}

inline void drift_skia_svg_dom_render_impl(DriftSkiaSVGDOM svg, DriftSkiaCanvas canvas, float width, float height) {
    if (!svg || !canvas || width <= 0 || height <= 0) return;
    auto dom = reinterpret_cast<SkSVGDOM*>(svg);
    // setContainerSize + render - size is set per-call to support multiple render sizes
    // NOTE: This mutates the DOM. If the same Icon is rendered at two different sizes
    // in the same frame, last write wins. Render on UI thread only.
    dom->setContainerSize(SkSize::Make(width, height));
    dom->render(reinterpret_cast<SkCanvas*>(canvas));
}

inline int drift_skia_svg_dom_get_size_impl(DriftSkiaSVGDOM svg, float* width, float* height) {
    if (!svg || !width || !height) return 0;
    auto dom = reinterpret_cast<SkSVGDOM*>(svg);
    SkSize size = dom->containerSize();
    if (size.isEmpty()) {
        if (const auto* root = dom->getRoot()) {
            size = root->intrinsicSize(SkSVGLengthContext(SkSize::Make(0, 0)));
        }
    }
    *width = size.width();
    *height = size.height();
    return (size.width() > 0 && size.height() > 0) ? 1 : 0;
}

inline void drift_skia_svg_dom_set_preserve_aspect_ratio_impl(DriftSkiaSVGDOM svg, int align, int scale) {
    if (!svg) return;
    auto dom = reinterpret_cast<SkSVGDOM*>(svg);
    auto* root = dom->getRoot();
    if (!root) return;

    SkSVGPreserveAspectRatio par;
    static const SkSVGPreserveAspectRatio::Align alignMap[] = {
        SkSVGPreserveAspectRatio::kXMidYMid, // 0 - default
        SkSVGPreserveAspectRatio::kXMinYMin, // 1
        SkSVGPreserveAspectRatio::kXMidYMin, // 2
        SkSVGPreserveAspectRatio::kXMaxYMin, // 3
        SkSVGPreserveAspectRatio::kXMinYMid, // 4
        SkSVGPreserveAspectRatio::kXMaxYMid, // 5
        SkSVGPreserveAspectRatio::kXMinYMax, // 6
        SkSVGPreserveAspectRatio::kXMidYMax, // 7
        SkSVGPreserveAspectRatio::kXMaxYMax, // 8
        SkSVGPreserveAspectRatio::kNone,     // 9
    };
    par.fAlign = (align >= 0 && align <= 9) ? alignMap[align] : SkSVGPreserveAspectRatio::kXMidYMid;
    par.fScale = (scale == 1) ? SkSVGPreserveAspectRatio::kSlice : SkSVGPreserveAspectRatio::kMeet;

    root->setPreserveAspectRatio(par);
}

inline void drift_skia_svg_dom_set_size_to_container_impl(DriftSkiaSVGDOM svg) {
    if (!svg) return;
    auto dom = reinterpret_cast<SkSVGDOM*>(svg);
    auto* root = dom->getRoot();
    if (!root) return;

    // Set width/height to 100% so SVG fills its container
    root->setWidth(SkSVGLength(100, SkSVGLength::Unit::kPercentage));
    root->setHeight(SkSVGLength(100, SkSVGLength::Unit::kPercentage));
}

inline void drift_skia_svg_dom_render_tinted_impl(
    DriftSkiaSVGDOM svg, DriftSkiaCanvas canvas,
    float width, float height, uint32_t tint_argb
) {
    if (!svg || !canvas || width <= 0 || height <= 0) return;
    auto dom = reinterpret_cast<SkSVGDOM*>(svg);
    auto sk_canvas = reinterpret_cast<SkCanvas*>(canvas);

    dom->setContainerSize(SkSize::Make(width, height));

    if (tint_argb == 0) {
        dom->render(sk_canvas);
        return;
    }

    // Use nullptr for bounds since Go-side already clips via CanvasClipRect.
    // This avoids double-clipping effects that extend beyond the SVG bounds.
    SkPaint paint;
    paint.setColorFilter(SkColorFilters::Blend(
        drift_skia_svg_impl::to_sk_color(tint_argb), SkBlendMode::kSrcIn));
    sk_canvas->saveLayer(nullptr, &paint);
    dom->render(sk_canvas);
    sk_canvas->restore();
}

#endif
