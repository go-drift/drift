#ifndef DRIFT_SKIA_BRIDGE_H
#define DRIFT_SKIA_BRIDGE_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef void* DriftSkiaContext;
typedef void* DriftSkiaSurface;
typedef void* DriftSkiaCanvas;
typedef void* DriftSkiaPath;

DriftSkiaContext drift_skia_context_create_gl(void);
DriftSkiaContext drift_skia_context_create_metal(void* device, void* queue);
void drift_skia_context_destroy(DriftSkiaContext ctx);

DriftSkiaSurface drift_skia_surface_create_gl(DriftSkiaContext ctx, int width, int height);
DriftSkiaSurface drift_skia_surface_create_metal(DriftSkiaContext ctx, void* texture, int width, int height);
DriftSkiaCanvas drift_skia_surface_get_canvas(DriftSkiaSurface surface);
void drift_skia_surface_flush(DriftSkiaContext ctx, DriftSkiaSurface surface);
void drift_skia_surface_destroy(DriftSkiaSurface surface);

void drift_skia_canvas_save(DriftSkiaCanvas canvas);
void drift_skia_canvas_save_layer_alpha(DriftSkiaCanvas canvas, float l, float t, float r, float b, uint8_t alpha);
void drift_skia_canvas_restore(DriftSkiaCanvas canvas);
void drift_skia_canvas_translate(DriftSkiaCanvas canvas, float dx, float dy);
void drift_skia_canvas_scale(DriftSkiaCanvas canvas, float sx, float sy);
void drift_skia_canvas_rotate(DriftSkiaCanvas canvas, float radians);
void drift_skia_canvas_clip_rect(DriftSkiaCanvas canvas, float l, float t, float r, float b);
void drift_skia_canvas_clip_rrect(
    DriftSkiaCanvas canvas,
    float l,
    float t,
    float r,
    float b,
    float rx1,
    float ry1,
    float rx2,
    float ry2,
    float rx3,
    float ry3,
    float rx4,
    float ry4
);
void drift_skia_canvas_clear(DriftSkiaCanvas canvas, uint32_t argb);
void drift_skia_canvas_draw_rect(DriftSkiaCanvas canvas, float l, float t, float r, float b, uint32_t argb, int style, float stroke_width, int aa);
void drift_skia_canvas_draw_rrect(
    DriftSkiaCanvas canvas,
    float l,
    float t,
    float r,
    float b,
    float rx1,
    float ry1,
    float rx2,
    float ry2,
    float rx3,
    float ry3,
    float rx4,
    float ry4,
    uint32_t argb,
    int style,
    float stroke_width,
    int aa
);
void drift_skia_canvas_draw_circle(DriftSkiaCanvas canvas, float cx, float cy, float radius, uint32_t argb, int style, float stroke_width, int aa);
void drift_skia_canvas_draw_line(DriftSkiaCanvas canvas, float x1, float y1, float x2, float y2, uint32_t argb, float stroke_width, int aa);
void drift_skia_canvas_draw_rect_gradient(
    DriftSkiaCanvas canvas,
    float l,
    float t,
    float r,
    float b,
    uint32_t argb,
    int style,
    float stroke_width,
    int aa,
    int gradient_type,
    float x1,
    float y1,
    float x2,
    float y2,
    float cx,
    float cy,
    float radius,
    const uint32_t* colors,
    const float* positions,
    int count
);
void drift_skia_canvas_draw_rrect_gradient(
    DriftSkiaCanvas canvas,
    float l,
    float t,
    float r,
    float b,
    float rx1,
    float ry1,
    float rx2,
    float ry2,
    float rx3,
    float ry3,
    float rx4,
    float ry4,
    uint32_t argb,
    int style,
    float stroke_width,
    int aa,
    int gradient_type,
    float x1,
    float y1,
    float x2,
    float y2,
    float cx,
    float cy,
    float radius,
    const uint32_t* colors,
    const float* positions,
    int count
);
void drift_skia_canvas_draw_circle_gradient(
    DriftSkiaCanvas canvas,
    float cx,
    float cy,
    float radius,
    uint32_t argb,
    int style,
    float stroke_width,
    int aa,
    int gradient_type,
    float x1,
    float y1,
    float x2,
    float y2,
    float rcx,
    float rcy,
    float rradius,
    const uint32_t* colors,
    const float* positions,
    int count
);
void drift_skia_canvas_draw_line_gradient(
    DriftSkiaCanvas canvas,
    float x1,
    float y1,
    float x2,
    float y2,
    uint32_t argb,
    float stroke_width,
    int aa,
    int gradient_type,
    float lx1,
    float ly1,
    float lx2,
    float ly2,
    float rcx,
    float rcy,
    float rradius,
    const uint32_t* colors,
    const float* positions,
    int count
);
void drift_skia_canvas_draw_path_gradient(
    DriftSkiaCanvas canvas,
    DriftSkiaPath path,
    uint32_t argb,
    int style,
    float stroke_width,
    int aa,
    int gradient_type,
    float x1,
    float y1,
    float x2,
    float y2,
    float rcx,
    float rcy,
    float rradius,
    const uint32_t* colors,
    const float* positions,
    int count
);
void drift_skia_canvas_draw_text_gradient(
    DriftSkiaCanvas canvas,
    const char* text,
    const char* family,
    float x,
    float y,
    float size,
    uint32_t argb,
    int weight,
    int style,
    int gradient_type,
    float x1,
    float y1,
    float x2,
    float y2,
    float cx,
    float cy,
    float radius,
    const uint32_t* colors,
    const float* positions,
    int count
);
void drift_skia_canvas_draw_text(DriftSkiaCanvas canvas, const char* text, const char* family, float x, float y, float size, uint32_t argb, int weight, int style);
void drift_skia_canvas_draw_image_rgba(DriftSkiaCanvas canvas, const uint8_t* pixels, int width, int height, int stride, float x, float y);
int drift_skia_register_font(const char* name, const uint8_t* data, int length);
int drift_skia_measure_text(const char* text, const char* family, float size, int weight, int style, float* width);
int drift_skia_font_metrics(const char* family, float size, int weight, int style, float* ascent, float* descent, float* leading);

DriftSkiaPath drift_skia_path_create(int fill_type);
void drift_skia_path_destroy(DriftSkiaPath path);
void drift_skia_path_move_to(DriftSkiaPath path, float x, float y);
void drift_skia_path_line_to(DriftSkiaPath path, float x, float y);
void drift_skia_path_quad_to(DriftSkiaPath path, float x1, float y1, float x2, float y2);
void drift_skia_path_cubic_to(DriftSkiaPath path, float x1, float y1, float x2, float y2, float x3, float y3);
void drift_skia_path_close(DriftSkiaPath path);
void drift_skia_canvas_draw_path(DriftSkiaCanvas canvas, DriftSkiaPath path, uint32_t argb, int style, float stroke_width, int aa);

#ifdef __cplusplus
}
#endif

#endif
