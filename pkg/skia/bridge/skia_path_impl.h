// Shared path implementation for Metal and Vulkan backends.
// This header should only be included from skia_metal.mm and skia_vk.cc.

#ifndef DRIFT_SKIA_PATH_IMPL_H
#define DRIFT_SKIA_PATH_IMPL_H

#include "../skia_bridge.h"
#include "core/SkPath.h"
#include "core/SkPathBuilder.h"

inline DriftSkiaPath drift_skia_path_create_impl(int fill_type) {
    SkPathFillType ft = (fill_type == 1) ? SkPathFillType::kEvenOdd : SkPathFillType::kWinding;
    return new SkPathBuilder(ft);
}

inline void drift_skia_path_destroy_impl(DriftSkiaPath path) {
    if (!path) {
        return;
    }
    delete reinterpret_cast<SkPathBuilder*>(path);
}

inline void drift_skia_path_move_to_impl(DriftSkiaPath path, float x, float y) {
    if (!path) {
        return;
    }
    reinterpret_cast<SkPathBuilder*>(path)->moveTo(x, y);
}

inline void drift_skia_path_line_to_impl(DriftSkiaPath path, float x, float y) {
    if (!path) {
        return;
    }
    reinterpret_cast<SkPathBuilder*>(path)->lineTo(x, y);
}

inline void drift_skia_path_quad_to_impl(DriftSkiaPath path, float x1, float y1, float x2, float y2) {
    if (!path) {
        return;
    }
    reinterpret_cast<SkPathBuilder*>(path)->quadTo(x1, y1, x2, y2);
}

inline void drift_skia_path_cubic_to_impl(DriftSkiaPath path, float x1, float y1, float x2, float y2, float x3, float y3) {
    if (!path) {
        return;
    }
    reinterpret_cast<SkPathBuilder*>(path)->cubicTo(x1, y1, x2, y2, x3, y3);
}

inline void drift_skia_path_close_impl(DriftSkiaPath path) {
    if (!path) {
        return;
    }
    reinterpret_cast<SkPathBuilder*>(path)->close();
}

inline SkPath drift_skia_path_snapshot(DriftSkiaPath path) {
    return reinterpret_cast<SkPathBuilder*>(path)->snapshot();
}

#endif  // DRIFT_SKIA_PATH_IMPL_H
