#ifndef DRIFT_SKIA_SKOTTIE_IMPL_H
#define DRIFT_SKIA_SKOTTIE_IMPL_H

#include "../skia_bridge.h"
#include "core/SkCanvas.h"
#include "core/SkData.h"
#include "modules/skottie/include/Skottie.h"

inline DriftSkiaSkottie drift_skia_skottie_create_impl(const uint8_t* data, int length) {
    if (!data || length <= 0) return nullptr;

    // Copy the data since Go memory may be moved/freed after cgo call returns
    sk_sp<SkData> skData = SkData::MakeWithCopy(data, static_cast<size_t>(length));
    if (!skData) return nullptr;

    auto anim = skottie::Animation::Builder().make(
        static_cast<const char*>(skData->data()), skData->size());
    if (!anim) return nullptr;

    return anim.release();
}

inline void drift_skia_skottie_destroy_impl(DriftSkiaSkottie anim) {
    if (anim) reinterpret_cast<skottie::Animation*>(anim)->unref();
}

inline int drift_skia_skottie_get_duration_impl(DriftSkiaSkottie anim, float* duration) {
    if (!anim || !duration) return 0;
    *duration = reinterpret_cast<skottie::Animation*>(anim)->duration();
    return 1;
}

inline int drift_skia_skottie_get_size_impl(DriftSkiaSkottie anim, float* width, float* height) {
    if (!anim || !width || !height) return 0;
    auto size = reinterpret_cast<skottie::Animation*>(anim)->size();
    *width = size.width();
    *height = size.height();
    return (size.width() > 0 && size.height() > 0) ? 1 : 0;
}

inline void drift_skia_skottie_seek_impl(DriftSkiaSkottie anim, float t) {
    if (!anim) return;
    // Clamp t to [0, 1]
    if (t < 0.0f) t = 0.0f;
    if (t > 1.0f) t = 1.0f;
    reinterpret_cast<skottie::Animation*>(anim)->seek(t);
}

inline void drift_skia_skottie_render_impl(DriftSkiaSkottie anim, DriftSkiaCanvas canvas, float width, float height) {
    if (!anim || !canvas || width <= 0 || height <= 0) return;
    SkRect dst = SkRect::MakeWH(width, height);
    reinterpret_cast<skottie::Animation*>(anim)->render(
        reinterpret_cast<SkCanvas*>(canvas), &dst);
}

#endif
