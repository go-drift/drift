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
typedef void* DriftSkiaSVGDOM;
typedef void* DriftSkiaParagraph;

DriftSkiaContext drift_skia_context_create_metal(void* device, void* queue);
DriftSkiaContext drift_skia_context_create_vulkan(
    uintptr_t instance,
    uintptr_t phys_device,
    uintptr_t device,
    uintptr_t queue,
    uint32_t queue_family_index,
    uintptr_t get_instance_proc_addr
);
void drift_skia_context_destroy(DriftSkiaContext ctx);

DriftSkiaSurface drift_skia_surface_create_metal(DriftSkiaContext ctx, void* texture, int width, int height);
DriftSkiaSurface drift_skia_surface_create_vulkan(
    DriftSkiaContext ctx,
    int width, int height,
    uintptr_t vk_image,
    uint32_t vk_format
);
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
void drift_skia_canvas_clip_path(
    DriftSkiaCanvas canvas,
    DriftSkiaPath path,
    int clip_op,
    int antialias
);
void drift_skia_canvas_save_layer(
    DriftSkiaCanvas canvas,
    float l, float t, float r, float b,
    int blend_mode,
    float alpha
);
void drift_skia_canvas_save_layer_filtered(
    DriftSkiaCanvas canvas,
    float l, float t, float r, float b,
    int blend_mode,
    float alpha,
    const float* color_filter_data, int color_filter_len,
    const float* image_filter_data, int image_filter_len
);
void drift_skia_canvas_clear(DriftSkiaCanvas canvas, uint32_t argb);
void drift_skia_canvas_draw_rect(
    DriftSkiaCanvas canvas, float l, float t, float r, float b,
    uint32_t argb, int style, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha
);
void drift_skia_canvas_draw_rrect(
    DriftSkiaCanvas canvas,
    float l, float t, float r, float b,
    float rx1, float ry1, float rx2, float ry2,
    float rx3, float ry3, float rx4, float ry4,
    uint32_t argb, int style, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha
);
void drift_skia_canvas_draw_circle(
    DriftSkiaCanvas canvas, float cx, float cy, float radius,
    uint32_t argb, int style, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha
);
void drift_skia_canvas_draw_line(
    DriftSkiaCanvas canvas, float x1, float y1, float x2, float y2,
    uint32_t argb, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha
);
void drift_skia_canvas_draw_rect_gradient(
    DriftSkiaCanvas canvas,
    float l, float t, float r, float b,
    uint32_t argb, int style, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha,
    int gradient_type,
    float x1, float y1, float x2, float y2,
    float cx, float cy, float radius,
    const uint32_t* colors, const float* positions, int count
);
void drift_skia_canvas_draw_rrect_gradient(
    DriftSkiaCanvas canvas,
    float l, float t, float r, float b,
    float rx1, float ry1, float rx2, float ry2,
    float rx3, float ry3, float rx4, float ry4,
    uint32_t argb, int style, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha,
    int gradient_type,
    float x1, float y1, float x2, float y2,
    float cx, float cy, float radius,
    const uint32_t* colors, const float* positions, int count
);
void drift_skia_canvas_draw_circle_gradient(
    DriftSkiaCanvas canvas,
    float cx, float cy, float radius,
    uint32_t argb, int style, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha,
    int gradient_type,
    float x1, float y1, float x2, float y2,
    float rcx, float rcy, float rradius,
    const uint32_t* colors, const float* positions, int count
);
void drift_skia_canvas_draw_line_gradient(
    DriftSkiaCanvas canvas,
    float x1, float y1, float x2, float y2,
    uint32_t argb, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha,
    int gradient_type,
    float lx1, float ly1, float lx2, float ly2,
    float rcx, float rcy, float rradius,
    const uint32_t* colors, const float* positions, int count
);
void drift_skia_canvas_draw_path_gradient(
    DriftSkiaCanvas canvas, DriftSkiaPath path,
    uint32_t argb, int style, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha,
    int gradient_type,
    float x1, float y1, float x2, float y2,
    float rcx, float rcy, float rradius,
    const uint32_t* colors, const float* positions, int count
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
void drift_skia_canvas_draw_text_shadow(DriftSkiaCanvas canvas, const char* text, const char* family, float x, float y, float size, uint32_t color, float sigma, int weight, int style);
void drift_skia_canvas_draw_image_rgba(DriftSkiaCanvas canvas, const uint8_t* pixels, int width, int height, int stride, float x, float y);
void drift_skia_canvas_draw_image_rect(
    DriftSkiaCanvas canvas,
    const uint8_t* pixels, int width, int height, int stride,
    float src_l, float src_t, float src_r, float src_b,
    float dst_l, float dst_t, float dst_r, float dst_b,
    int filter_quality,
    uintptr_t cache_key
);
DriftSkiaParagraph drift_skia_paragraph_create(
    const char* text,
    const char* family,
    float size,
    int weight,
    int style,
    uint32_t argb,
    int max_lines,
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
    int count,
    int shadow_enabled,
    uint32_t shadow_argb,
    float shadow_dx,
    float shadow_dy,
    float shadow_sigma,
    int text_align
);
void drift_skia_paragraph_layout(DriftSkiaParagraph paragraph, float width);
int drift_skia_paragraph_get_metrics(DriftSkiaParagraph paragraph, float* height, float* longest_line, float* max_intrinsic_width, int* line_count);
int drift_skia_paragraph_get_line_metrics(DriftSkiaParagraph paragraph, float* widths, float* ascents, float* descents, float* heights, int count);
void drift_skia_paragraph_paint(DriftSkiaParagraph paragraph, DriftSkiaCanvas canvas, float x, float y);
void drift_skia_paragraph_destroy(DriftSkiaParagraph paragraph);

typedef struct {
    const char* text;
    const char* family;
    float size;
    int weight;
    int style;
    uint32_t color;
    int decoration;
    uint32_t decoration_color;
    int decoration_style;
    float letter_spacing;
    float word_spacing;
    float height;
    int has_background;
    uint32_t background_color;
} DriftTextSpan;

DriftSkiaParagraph drift_skia_rich_paragraph_create(
    const DriftTextSpan* spans,
    int span_count,
    int max_lines,
    int text_align
);

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
void drift_skia_canvas_draw_path(
    DriftSkiaCanvas canvas, DriftSkiaPath path,
    uint32_t argb, int style, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha
);

void drift_skia_canvas_draw_rect_shadow(
    DriftSkiaCanvas canvas,
    float l, float t, float r, float b,
    uint32_t color, float sigma, float dx, float dy, float spread, int blur_style
);
void drift_skia_canvas_draw_rrect_shadow(
    DriftSkiaCanvas canvas,
    float l, float t, float r, float b,
    float rx1, float ry1, float rx2, float ry2, float rx3, float ry3, float rx4, float ry4,
    uint32_t color, float sigma, float dx, float dy, float spread, int blur_style
);
void drift_skia_canvas_save_layer_blur(
    DriftSkiaCanvas canvas,
    float l, float t, float r, float b,
    float sigma_x, float sigma_y
);

DriftSkiaSVGDOM drift_skia_svg_dom_create(const uint8_t* data, int length);
DriftSkiaSVGDOM drift_skia_svg_dom_create_with_base(const uint8_t* data, int length, const char* base_path);
void drift_skia_svg_dom_destroy(DriftSkiaSVGDOM svg);
void drift_skia_svg_dom_render(DriftSkiaSVGDOM svg, DriftSkiaCanvas canvas, float width, float height);
int drift_skia_svg_dom_get_size(DriftSkiaSVGDOM svg, float* width, float* height);
// align: 0=xMidYMid(default), 1=xMinYMin, 2=xMidYMin, 3=xMaxYMin, 4=xMinYMid,
//        5=xMaxYMid, 6=xMinYMax, 7=xMidYMax, 8=xMaxYMax, 9=none
// scale: 0=meet(contain), 1=slice(cover)
void drift_skia_svg_dom_set_preserve_aspect_ratio(DriftSkiaSVGDOM svg, int align, int scale);
void drift_skia_svg_dom_set_size_to_container(DriftSkiaSVGDOM svg);
void drift_skia_svg_dom_render_tinted(DriftSkiaSVGDOM svg, DriftSkiaCanvas canvas,
    float width, float height, uint32_t tint_argb);

typedef void* DriftSkiaSkottie;

DriftSkiaSkottie drift_skia_skottie_create(const uint8_t* data, int length);
void drift_skia_skottie_destroy(DriftSkiaSkottie anim);
int drift_skia_skottie_get_duration(DriftSkiaSkottie anim, float* duration);
int drift_skia_skottie_get_size(DriftSkiaSkottie anim, float* width, float* height);
void drift_skia_skottie_seek(DriftSkiaSkottie anim, float t);
void drift_skia_skottie_render(DriftSkiaSkottie anim, DriftSkiaCanvas canvas, float width, float height);

DriftSkiaSurface drift_skia_surface_create_offscreen_metal(DriftSkiaContext ctx, int width, int height);
DriftSkiaSurface drift_skia_surface_create_offscreen_vulkan(DriftSkiaContext ctx, int width, int height);
void drift_skia_context_flush_and_submit(DriftSkiaContext ctx, int sync_cpu);
void drift_skia_context_purge_resources(DriftSkiaContext ctx);

#ifdef __cplusplus
}
#endif

#endif
