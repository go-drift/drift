// Drift Skia GL bridge for Android
// Pre-compiled at CI time, not by CGO

#include "../skia_bridge.h"

#include <GLES2/gl2.h>
#include <android/log.h>
#include <algorithm>
#include <cstddef>
#include <cstring>
#include <mutex>
#include <string>
#include <unordered_map>
#include <vector>

#ifndef GL_RGBA8
#define GL_RGBA8 GL_RGBA
#endif

#include "core/SkCanvas.h"
#include "core/SkColor.h"
#include "core/SkColorSpace.h"
#include "core/SkData.h"
#include "core/SkFont.h"
#include "core/SkFontMetrics.h"
#include "core/SkImage.h"
#include "core/SkImageInfo.h"
#include "core/SkPaint.h"
#include "effects/SkGradient.h"
#include "core/SkBlurTypes.h"
#include "core/SkMaskFilter.h"
#include "core/SkRRect.h"
#include "core/SkScalar.h"
#include "core/SkSurface.h"
#include "effects/SkImageFilters.h"
#include "core/SkSurfaceProps.h"
#include "core/SkTypeface.h"
#include "core/SkFontMgr.h"
#include "core/SkString.h"
#include "gpu/ganesh/GrBackendSurface.h"
#include "gpu/ganesh/GrDirectContext.h"
#include "gpu/ganesh/SkSurfaceGanesh.h"
#include "gpu/ganesh/gl/GrGLBackendSurface.h"
#include "gpu/ganesh/gl/GrGLDirectContext.h"
#include "gpu/ganesh/gl/GrGLInterface.h"
#include "ports/SkFontMgr_android.h"
#include "ports/SkFontMgr_android_ndk.h"
#include "ports/SkFontScanner_FreeType.h"
#include "skia_path_impl.h"
#include "skia_svg_impl.h"

namespace {

SkColor to_sk_color(uint32_t argb) {
    return SkColorSetARGB(
        (argb >> 24) & 0xFF,
        (argb >> 16) & 0xFF,
        (argb >> 8) & 0xFF,
        argb & 0xFF
    );
}

SkPaint make_paint(uint32_t argb, int style, float stroke_width, int aa) {
    SkPaint paint;
    paint.setAntiAlias(aa != 0);
    paint.setColor(to_sk_color(argb));
    if (stroke_width > 0) {
        paint.setStrokeWidth(stroke_width);
    }
    switch (style) {
        case 1:
            paint.setStyle(SkPaint::kStroke_Style);
            break;
        case 2:
            paint.setStyle(SkPaint::kStrokeAndFill_Style);
            break;
        default:
            paint.setStyle(SkPaint::kFill_Style);
            break;
    }
    return paint;
}

constexpr int kGradientLinear = 1;
constexpr int kGradientRadial = 2;

bool build_gradient_stops(const uint32_t* colors, const float* positions, int count, std::vector<SkColor4f>& skColors, std::vector<float>& skPositions) {
    if (!colors || !positions || count < 2) {
        return false;
    }
    skColors.reserve(static_cast<size_t>(count));
    skPositions.reserve(static_cast<size_t>(count));
    for (int i = 0; i < count; ++i) {
        skColors.push_back(SkColor4f::FromColor(to_sk_color(colors[i])));
        skPositions.push_back(positions[i]);
    }
    return true;
}

sk_sp<SkShader> make_linear_gradient(float x1, float y1, float x2, float y2, const uint32_t* colors, const float* positions, int count) {
    std::vector<SkColor4f> skColors;
    std::vector<float> skPositions;
    if (!build_gradient_stops(colors, positions, count, skColors, skPositions)) {
        return nullptr;
    }
    SkPoint pts[2] = {{x1, y1}, {x2, y2}};
    SkGradient gradient(
        SkGradient::Colors(
            SkSpan<const SkColor4f>(skColors.data(), skColors.size()),
            SkSpan<const float>(skPositions.data(), skPositions.size()),
            SkTileMode::kClamp
        ),
        SkGradient::Interpolation()
    );
    return SkShaders::LinearGradient(pts, gradient, nullptr);
}

sk_sp<SkShader> make_radial_gradient(float cx, float cy, float radius, const uint32_t* colors, const float* positions, int count) {
    if (radius <= 0) {
        return nullptr;
    }
    std::vector<SkColor4f> skColors;
    std::vector<float> skPositions;
    if (!build_gradient_stops(colors, positions, count, skColors, skPositions)) {
        return nullptr;
    }
    SkPoint center = {cx, cy};
    SkGradient gradient(
        SkGradient::Colors(
            SkSpan<const SkColor4f>(skColors.data(), skColors.size()),
            SkSpan<const float>(skPositions.data(), skPositions.size()),
            SkTileMode::kClamp
        ),
        SkGradient::Interpolation()
    );
    return SkShaders::RadialGradient(center, radius, gradient, nullptr);
}

sk_sp<SkShader> make_gradient_shader(int gradient_type, float x1, float y1, float x2, float y2, float cx, float cy, float radius, const uint32_t* colors, const float* positions, int count) {
    switch (gradient_type) {
        case kGradientLinear:
            return make_linear_gradient(x1, y1, x2, y2, colors, positions, count);
        case kGradientRadial:
            return make_radial_gradient(cx, cy, radius, colors, positions, count);
        default:
            return nullptr;
    }
}

struct FontRegistry {
    std::mutex mu;
    std::unordered_map<std::string, sk_sp<SkTypeface>> custom;
};

sk_sp<SkFontMgr> get_font_manager();

FontRegistry& font_registry() {
    static FontRegistry registry;
    return registry;
}

sk_sp<SkTypeface> lookup_custom_typeface(const char* family) {
    if (!family || family[0] == '\0') {
        return nullptr;
    }
    auto& registry = font_registry();
    std::lock_guard<std::mutex> lock(registry.mu);
    auto it = registry.custom.find(family);
    if (it != registry.custom.end()) {
        return it->second;
    }
    return nullptr;
}

bool register_font(const char* name, const uint8_t* data, int length) {
    if (!name || name[0] == '\0' || !data || length <= 0) {
        return false;
    }
    auto font_data = SkData::MakeWithCopy(data, static_cast<size_t>(length));
    if (!font_data) {
        return false;
    }
    auto manager = get_font_manager();
    auto typeface = manager ? manager->makeFromData(font_data) : nullptr;
    if (!typeface) {
        return false;
    }
    auto& registry = font_registry();
    std::lock_guard<std::mutex> lock(registry.mu);
    registry.custom[name] = typeface;
    return true;
}

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

sk_sp<SkTypeface> resolve_typeface(const char* family, int weight, int style) {
    struct Cache {
        std::string family;
        int weight = -1;
        int style = -1;
        sk_sp<SkTypeface> typeface;
    };
    static Cache cache;

    weight = std::clamp(weight, 100, 900);
    std::string family_name = (family && family[0] != '\0') ? family : "";
    if (cache.typeface && cache.weight == weight && cache.style == style && cache.family == family_name) {
        return cache.typeface;
    }

    SkFontStyle::Slant slant = (style == 1) ? SkFontStyle::kItalic_Slant : SkFontStyle::kUpright_Slant;
    SkFontStyle font_style(weight, SkFontStyle::kNormal_Width, slant);
    auto manager = get_font_manager();
    sk_sp<SkTypeface> typeface = lookup_custom_typeface(family);
    if (!typeface && manager && !family_name.empty()) {
        typeface = manager->matchFamilyStyle(family_name.c_str(), font_style);
    }
    if (!typeface && manager) {
        typeface = manager->matchFamilyStyle(nullptr, font_style);
    }
    if (!typeface && manager) {
        typeface = manager->matchFamilyStyle("sans-serif", font_style);
    }
    if (!typeface && manager) {
        int family_count = manager->countFamilies();
        if (family_count > 0) {
            SkString fallback_name;
            manager->getFamilyName(0, &fallback_name);
            typeface = manager->matchFamilyStyle(fallback_name.c_str(), font_style);
        }
    }
    if (!typeface && manager) {
        SkFontStyle fallback_style(400, SkFontStyle::kNormal_Width, slant);
        typeface = manager->matchFamilyStyle("sans-serif", fallback_style);
    }
    if (!typeface) {
        __android_log_print(ANDROID_LOG_WARN, "DriftSkia", "No typeface match for family=%s weight=%d style=%d", family_name.c_str(), weight, style);
    }
    cache.family = family_name;
    cache.weight = weight;
    cache.style = style;
    cache.typeface = typeface;
    return typeface;
}

SkFont make_font(const char* family, float size, int weight, int style) {
    SkFont font;
    auto typeface = resolve_typeface(family, weight, style);
    if (typeface) {
        font.setTypeface(typeface);
    }
    font.setSize(size);
    font.setEdging(SkFont::Edging::kSubpixelAntiAlias);
    font.setHinting(SkFontHinting::kNormal);
    if (style == 1) {
        font.setSkewX(-0.25f);
    }
    return font;
}

}  // namespace

extern "C" {

DriftSkiaContext drift_skia_context_create_gl(void) {
    auto interface = GrGLMakeNativeInterface();
    if (!interface) {
        return nullptr;
    }
    auto context = GrDirectContexts::MakeGL(interface);
    if (!context) {
        return nullptr;
    }
    return context.release();
}

DriftSkiaContext drift_skia_context_create_metal(void* device, void* queue) {
    (void)device;
    (void)queue;
    return nullptr;
}

void drift_skia_context_destroy(DriftSkiaContext ctx) {
    if (!ctx) {
        return;
    }
    reinterpret_cast<GrDirectContext*>(ctx)->unref();
}

static SkSurface* create_gl_surface(GrDirectContext* context, int width, int height, GrGLenum format, SkColorType color_type, int samples, int stencil, GrGLuint framebuffer) {
    GrGLFramebufferInfo fb_info;
    fb_info.fFBOID = framebuffer;
    fb_info.fFormat = format;

    GrBackendRenderTarget backend_target = GrBackendRenderTargets::MakeGL(
        width,
        height,
        samples,
        stencil,
        fb_info
    );
    SkSurfaceProps props(0, kRGB_H_SkPixelGeometry);

    auto surface = SkSurfaces::WrapBackendRenderTarget(
        context,
        backend_target,
        kTopLeft_GrSurfaceOrigin,
        color_type,
        SkColorSpace::MakeSRGB(),
        &props
    );

    if (!surface) {
        return nullptr;
    }
    return surface.release();
}

DriftSkiaSurface drift_skia_surface_create_gl(DriftSkiaContext ctx, int width, int height) {
    if (!ctx || width <= 0 || height <= 0) {
        return nullptr;
    }

    GLint framebuffer = 0;
    GLint samples = 0;
    GLint stencil = 0;
    glGetIntegerv(GL_FRAMEBUFFER_BINDING, &framebuffer);
    glGetIntegerv(GL_SAMPLES, &samples);
    glGetIntegerv(GL_STENCIL_BITS, &stencil);
    auto context = reinterpret_cast<GrDirectContext*>(ctx);

    SkSurface* surface = create_gl_surface(context, width, height, GL_RGBA8, kRGBA_8888_SkColorType, samples, stencil, static_cast<GrGLuint>(framebuffer));
    if (!surface) {
        surface = create_gl_surface(context, width, height, GL_RGBA, kRGBA_8888_SkColorType, samples, stencil, static_cast<GrGLuint>(framebuffer));
    }
#ifdef GL_BGRA8_EXT
    if (!surface) {
        surface = create_gl_surface(context, width, height, GL_BGRA8_EXT, kBGRA_8888_SkColorType, samples, stencil, static_cast<GrGLuint>(framebuffer));
    }
#endif
    if (!surface) {
        surface = create_gl_surface(context, width, height, GL_RGB565, kRGB_565_SkColorType, samples, stencil, static_cast<GrGLuint>(framebuffer));
    }

    if (!surface && stencil != 0) {
        surface = create_gl_surface(context, width, height, GL_RGBA8, kRGBA_8888_SkColorType, samples, 0, static_cast<GrGLuint>(framebuffer));
        if (!surface) {
            surface = create_gl_surface(context, width, height, GL_RGBA, kRGBA_8888_SkColorType, samples, 0, static_cast<GrGLuint>(framebuffer));
        }
#ifdef GL_BGRA8_EXT
        if (!surface) {
            surface = create_gl_surface(context, width, height, GL_BGRA8_EXT, kBGRA_8888_SkColorType, samples, 0, static_cast<GrGLuint>(framebuffer));
        }
#endif
        if (!surface) {
            surface = create_gl_surface(context, width, height, GL_RGB565, kRGB_565_SkColorType, samples, 0, static_cast<GrGLuint>(framebuffer));
        }
    }

    if (!surface) {
        const GLubyte* version = glGetString(GL_VERSION);
        const GLubyte* renderer = glGetString(GL_RENDERER);
        __android_log_print(ANDROID_LOG_ERROR, "DriftSkia", "Failed GL surface: fbo=%d samples=%d stencil=%d version=%s renderer=%s",
                            framebuffer, samples, stencil,
                            version ? reinterpret_cast<const char*>(version) : "unknown",
                            renderer ? reinterpret_cast<const char*>(renderer) : "unknown");
        return nullptr;
    }

    return surface;
}

DriftSkiaSurface drift_skia_surface_create_metal(DriftSkiaContext ctx, void* texture, int width, int height) {
    (void)ctx;
    (void)texture;
    (void)width;
    (void)height;
    return nullptr;
}

DriftSkiaCanvas drift_skia_surface_get_canvas(DriftSkiaSurface surface) {
    if (!surface) {
        return nullptr;
    }
    return reinterpret_cast<SkSurface*>(surface)->getCanvas();
}

void drift_skia_surface_flush(DriftSkiaContext ctx, DriftSkiaSurface surface) {
    if (!ctx || !surface) {
        return;
    }
    auto sk_surface = reinterpret_cast<SkSurface*>(surface);
    reinterpret_cast<GrDirectContext*>(ctx)->flushAndSubmit(sk_surface);
}

void drift_skia_surface_destroy(DriftSkiaSurface surface) {
    if (!surface) {
        return;
    }
    reinterpret_cast<SkSurface*>(surface)->unref();
}

void drift_skia_canvas_save(DriftSkiaCanvas canvas) {
    if (!canvas) {
        return;
    }
    reinterpret_cast<SkCanvas*>(canvas)->save();
}

void drift_skia_canvas_save_layer_alpha(DriftSkiaCanvas canvas, float l, float t, float r, float b, uint8_t alpha) {
    if (!canvas) {
        return;
    }
    SkRect bounds = SkRect::MakeLTRB(l, t, r, b);
    reinterpret_cast<SkCanvas*>(canvas)->saveLayerAlpha(&bounds, alpha);
}

void drift_skia_canvas_restore(DriftSkiaCanvas canvas) {
    if (!canvas) {
        return;
    }
    reinterpret_cast<SkCanvas*>(canvas)->restore();
}

void drift_skia_canvas_translate(DriftSkiaCanvas canvas, float dx, float dy) {
    if (!canvas) {
        return;
    }
    reinterpret_cast<SkCanvas*>(canvas)->translate(dx, dy);
}

void drift_skia_canvas_scale(DriftSkiaCanvas canvas, float sx, float sy) {
    if (!canvas) {
        return;
    }
    reinterpret_cast<SkCanvas*>(canvas)->scale(sx, sy);
}

void drift_skia_canvas_rotate(DriftSkiaCanvas canvas, float radians) {
    if (!canvas) {
        return;
    }
    reinterpret_cast<SkCanvas*>(canvas)->rotate(radians * 180.0f / 3.14159265f);
}

void drift_skia_canvas_clip_rect(DriftSkiaCanvas canvas, float l, float t, float r, float b) {
    if (!canvas) {
        return;
    }
    SkRect rect = SkRect::MakeLTRB(l, t, r, b);
    reinterpret_cast<SkCanvas*>(canvas)->clipRect(rect);
}

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
) {
    if (!canvas) {
        return;
    }
    SkRect rect = SkRect::MakeLTRB(l, t, r, b);
    SkVector radii[4] = {
        {rx1, ry1},
        {rx2, ry2},
        {rx3, ry3},
        {rx4, ry4}
    };
    SkRRect rrect;
    rrect.setRectRadii(rect, radii);
    reinterpret_cast<SkCanvas*>(canvas)->clipRRect(rrect);
}

void drift_skia_canvas_clear(DriftSkiaCanvas canvas, uint32_t argb) {
    if (!canvas) {
        return;
    }
    reinterpret_cast<SkCanvas*>(canvas)->clear(to_sk_color(argb));
}

void drift_skia_canvas_draw_rect(DriftSkiaCanvas canvas, float l, float t, float r, float b, uint32_t argb, int style, float stroke_width, int aa) {
    if (!canvas) {
        return;
    }
    SkRect rect = SkRect::MakeLTRB(l, t, r, b);
    SkPaint paint = make_paint(argb, style, stroke_width, aa);
    reinterpret_cast<SkCanvas*>(canvas)->drawRect(rect, paint);
}

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
) {
    if (!canvas) {
        return;
    }
    SkRect rect = SkRect::MakeLTRB(l, t, r, b);
    SkVector radii[4] = {
        {rx1, ry1},
        {rx2, ry2},
        {rx3, ry3},
        {rx4, ry4}
    };
    SkRRect rrect;
    rrect.setRectRadii(rect, radii);
    SkPaint paint = make_paint(argb, style, stroke_width, aa);
    reinterpret_cast<SkCanvas*>(canvas)->drawRRect(rrect, paint);
}

void drift_skia_canvas_draw_circle(DriftSkiaCanvas canvas, float cx, float cy, float radius, uint32_t argb, int style, float stroke_width, int aa) {
    if (!canvas) {
        return;
    }
    SkPaint paint = make_paint(argb, style, stroke_width, aa);
    reinterpret_cast<SkCanvas*>(canvas)->drawCircle(cx, cy, radius, paint);
}

void drift_skia_canvas_draw_line(DriftSkiaCanvas canvas, float x1, float y1, float x2, float y2, uint32_t argb, float stroke_width, int aa) {
    if (!canvas) {
        return;
    }
    SkPaint paint = make_paint(argb, 1, stroke_width, aa);
    reinterpret_cast<SkCanvas*>(canvas)->drawLine(x1, y1, x2, y2, paint);
}

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
) {
    if (!canvas) {
        return;
    }
    SkRect rect = SkRect::MakeLTRB(l, t, r, b);
    SkPaint paint = make_paint(argb, style, stroke_width, aa);
    auto shader = make_gradient_shader(gradient_type, x1, y1, x2, y2, cx, cy, radius, colors, positions, count);
    if (shader) {
        paint.setShader(shader);
    }
    reinterpret_cast<SkCanvas*>(canvas)->drawRect(rect, paint);
}

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
) {
    if (!canvas) {
        return;
    }
    SkRect rect = SkRect::MakeLTRB(l, t, r, b);
    SkVector radii[4] = {
        {rx1, ry1},
        {rx2, ry2},
        {rx3, ry3},
        {rx4, ry4}
    };
    SkRRect rrect;
    rrect.setRectRadii(rect, radii);
    SkPaint paint = make_paint(argb, style, stroke_width, aa);
    auto shader = make_gradient_shader(gradient_type, x1, y1, x2, y2, cx, cy, radius, colors, positions, count);
    if (shader) {
        paint.setShader(shader);
    }
    reinterpret_cast<SkCanvas*>(canvas)->drawRRect(rrect, paint);
}

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
) {
    if (!canvas) {
        return;
    }
    SkPaint paint = make_paint(argb, style, stroke_width, aa);
    auto shader = make_gradient_shader(gradient_type, x1, y1, x2, y2, rcx, rcy, rradius, colors, positions, count);
    if (shader) {
        paint.setShader(shader);
    }
    reinterpret_cast<SkCanvas*>(canvas)->drawCircle(cx, cy, radius, paint);
}

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
) {
    if (!canvas) {
        return;
    }
    SkPaint paint = make_paint(argb, 1, stroke_width, aa);
    auto shader = make_gradient_shader(gradient_type, lx1, ly1, lx2, ly2, rcx, rcy, rradius, colors, positions, count);
    if (shader) {
        paint.setShader(shader);
    }
    reinterpret_cast<SkCanvas*>(canvas)->drawLine(x1, y1, x2, y2, paint);
}

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
) {
    if (!canvas || !path) {
        return;
    }
    SkPaint paint = make_paint(argb, style, stroke_width, aa);
    auto shader = make_gradient_shader(gradient_type, x1, y1, x2, y2, rcx, rcy, rradius, colors, positions, count);
    if (shader) {
        paint.setShader(shader);
    }
    reinterpret_cast<SkCanvas*>(canvas)->drawPath(drift_skia_path_snapshot(path), paint);
}

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
) {
    if (!canvas || !text) {
        return;
    }
    SkFont font = make_font(family, size, weight, style);
    SkPaint paint;
    paint.setAntiAlias(true);
    paint.setColor(to_sk_color(argb));
    auto shader = make_gradient_shader(gradient_type, x1, y1, x2, y2, cx, cy, radius, colors, positions, count);
    if (shader) {
        paint.setShader(shader);
    }

    reinterpret_cast<SkCanvas*>(canvas)->drawSimpleText(
        text,
        std::strlen(text),
        SkTextEncoding::kUTF8,
        x,
        y,
        font,
        paint
    );
}

void drift_skia_canvas_draw_text(DriftSkiaCanvas canvas, const char* text, const char* family, float x, float y, float size, uint32_t argb, int weight, int style) {
    if (!canvas || !text) {
        return;
    }
    SkFont font = make_font(family, size, weight, style);
    SkPaint paint;
    paint.setAntiAlias(true);
    paint.setColor(to_sk_color(argb));

    reinterpret_cast<SkCanvas*>(canvas)->drawSimpleText(
        text,
        std::strlen(text),
        SkTextEncoding::kUTF8,
        x,
        y,
        font,
        paint
    );
}

void drift_skia_canvas_draw_text_shadow(DriftSkiaCanvas canvas, const char* text, const char* family, float x, float y, float size, uint32_t color, float sigma, int weight, int style) {
    if (!canvas || !text) {
        return;
    }
    SkFont font = make_font(family, size, weight, style);
    SkPaint paint;
    paint.setAntiAlias(true);
    paint.setColor(to_sk_color(color));
    if (sigma > 0) {
        paint.setMaskFilter(SkMaskFilter::MakeBlur(kNormal_SkBlurStyle, sigma));
    }

    reinterpret_cast<SkCanvas*>(canvas)->drawSimpleText(
        text,
        std::strlen(text),
        SkTextEncoding::kUTF8,
        x,
        y,
        font,
        paint
    );
}

int drift_skia_register_font(const char* name, const uint8_t* data, int length) {
    return register_font(name, data, length) ? 1 : 0;
}

int drift_skia_measure_text(const char* text, const char* family, float size, int weight, int style, float* width) {
    if (!width) {
        return 0;
    }
    if (!text) {
        *width = 0.0f;
        return 1;
    }
    SkFont font = make_font(family, size, weight, style);
    *width = font.measureText(text, std::strlen(text), SkTextEncoding::kUTF8);
    return 1;
}

int drift_skia_font_metrics(const char* family, float size, int weight, int style, float* ascent, float* descent, float* leading) {
    if (!ascent || !descent || !leading) {
        return 0;
    }
    SkFont font = make_font(family, size, weight, style);
    SkFontMetrics metrics;
    font.getMetrics(&metrics);
    *ascent = -metrics.fAscent;
    *descent = metrics.fDescent;
    *leading = metrics.fLeading;
    return 1;
}

void drift_skia_canvas_draw_image_rgba(DriftSkiaCanvas canvas, const uint8_t* pixels, int width, int height, int stride, float x, float y) {
    if (!canvas || !pixels || width <= 0 || height <= 0 || stride <= 0) {
        return;
    }

    SkImageInfo info = SkImageInfo::Make(width, height, kRGBA_8888_SkColorType, kPremul_SkAlphaType);
    auto data = SkData::MakeWithCopy(pixels, static_cast<size_t>(stride) * height);
    if (!data) {
        return;
    }
    auto image = SkImages::RasterFromData(info, data, stride);
    if (!image) {
        return;
    }
    reinterpret_cast<SkCanvas*>(canvas)->drawImage(image, x, y);
}

DriftSkiaPath drift_skia_path_create(int fill_type) {
    return drift_skia_path_create_impl(fill_type);
}

void drift_skia_path_destroy(DriftSkiaPath path) {
    drift_skia_path_destroy_impl(path);
}

void drift_skia_path_move_to(DriftSkiaPath path, float x, float y) {
    drift_skia_path_move_to_impl(path, x, y);
}

void drift_skia_path_line_to(DriftSkiaPath path, float x, float y) {
    drift_skia_path_line_to_impl(path, x, y);
}

void drift_skia_path_quad_to(DriftSkiaPath path, float x1, float y1, float x2, float y2) {
    drift_skia_path_quad_to_impl(path, x1, y1, x2, y2);
}

void drift_skia_path_cubic_to(DriftSkiaPath path, float x1, float y1, float x2, float y2, float x3, float y3) {
    drift_skia_path_cubic_to_impl(path, x1, y1, x2, y2, x3, y3);
}

void drift_skia_path_close(DriftSkiaPath path) {
    drift_skia_path_close_impl(path);
}

void drift_skia_canvas_draw_path(DriftSkiaCanvas canvas, DriftSkiaPath path, uint32_t argb, int style, float stroke_width, int aa) {
    if (!canvas || !path) {
        return;
    }
    SkPaint paint = make_paint(argb, style, stroke_width, aa);
    reinterpret_cast<SkCanvas*>(canvas)->drawPath(drift_skia_path_snapshot(path), paint);
}

void drift_skia_canvas_draw_rect_shadow(
    DriftSkiaCanvas canvas,
    float l, float t, float r, float b,
    uint32_t color, float sigma, float dx, float dy, float spread, int blur_style
) {
    if (!canvas) {
        return;
    }
    if (spread < 0) spread = 0;
    SkRect rect = SkRect::MakeLTRB(l - spread, t - spread, r + spread, b + spread);
    SkPaint paint;
    paint.setAntiAlias(true);
    paint.setColor(to_sk_color(color));
    if (sigma > 0) {
        SkBlurStyle skStyle;
        switch (blur_style) {
            case 1: skStyle = kSolid_SkBlurStyle; break;
            case 2: skStyle = kOuter_SkBlurStyle; break;
            case 3: skStyle = kInner_SkBlurStyle; break;
            default: skStyle = kNormal_SkBlurStyle; break;
        }
        paint.setMaskFilter(SkMaskFilter::MakeBlur(skStyle, sigma));
    }
    auto sk_canvas = reinterpret_cast<SkCanvas*>(canvas);
    sk_canvas->save();
    sk_canvas->translate(dx, dy);
    sk_canvas->drawRect(rect, paint);
    sk_canvas->restore();
}

void drift_skia_canvas_draw_rrect_shadow(
    DriftSkiaCanvas canvas,
    float l, float t, float r, float b,
    float rx1, float ry1, float rx2, float ry2, float rx3, float ry3, float rx4, float ry4,
    uint32_t color, float sigma, float dx, float dy, float spread, int blur_style
) {
    if (!canvas) {
        return;
    }
    if (spread < 0) spread = 0;
    SkRect rect = SkRect::MakeLTRB(l - spread, t - spread, r + spread, b + spread);
    SkVector radii[4] = {
        {rx1 + spread, ry1 + spread},
        {rx2 + spread, ry2 + spread},
        {rx3 + spread, ry3 + spread},
        {rx4 + spread, ry4 + spread}
    };
    SkRRect rrect;
    rrect.setRectRadii(rect, radii);
    SkPaint paint;
    paint.setAntiAlias(true);
    paint.setColor(to_sk_color(color));
    if (sigma > 0) {
        SkBlurStyle skStyle;
        switch (blur_style) {
            case 1: skStyle = kSolid_SkBlurStyle; break;
            case 2: skStyle = kOuter_SkBlurStyle; break;
            case 3: skStyle = kInner_SkBlurStyle; break;
            default: skStyle = kNormal_SkBlurStyle; break;
        }
        paint.setMaskFilter(SkMaskFilter::MakeBlur(skStyle, sigma));
    }
    auto sk_canvas = reinterpret_cast<SkCanvas*>(canvas);
    sk_canvas->save();
    sk_canvas->translate(dx, dy);
    sk_canvas->drawRRect(rrect, paint);
    sk_canvas->restore();
}

void drift_skia_canvas_save_layer_blur(
    DriftSkiaCanvas canvas,
    float l, float t, float r, float b,
    float sigma_x, float sigma_y
) {
    if (!canvas) {
        return;
    }
    auto sk_canvas = reinterpret_cast<SkCanvas*>(canvas);
    SkRect bounds = SkRect::MakeLTRB(l, t, r, b);

    // Skip blur if sigma is negligible
    if (sigma_x < 0.5f && sigma_y < 0.5f) {
        sk_canvas->saveLayer(&bounds, nullptr);
        return;
    }

    // kDecal avoids edge artifacts for bounded blur
    auto blur = SkImageFilters::Blur(sigma_x, sigma_y, SkTileMode::kDecal, nullptr);
    if (!blur) {
        sk_canvas->saveLayer(&bounds, nullptr);
        return;
    }

    // fBackdrop applies blur to existing content (the backdrop)
    // Note: kInitWithPrevious is implicit when fBackdrop is set
    SkCanvas::SaveLayerRec rec;
    rec.fBounds = &bounds;
    rec.fBackdrop = blur.get();
    sk_canvas->saveLayer(rec);
}

DriftSkiaSVGDOM drift_skia_svg_dom_create(const uint8_t* data, int length) {
    return drift_skia_svg_dom_create_impl(data, length);
}

DriftSkiaSVGDOM drift_skia_svg_dom_create_with_base(const uint8_t* data, int length, const char* base_path) {
    return drift_skia_svg_dom_create_with_base_impl(data, length, base_path);
}

void drift_skia_svg_dom_destroy(DriftSkiaSVGDOM svg) {
    drift_skia_svg_dom_destroy_impl(svg);
}

void drift_skia_svg_dom_render(DriftSkiaSVGDOM svg, DriftSkiaCanvas canvas, float width, float height) {
    drift_skia_svg_dom_render_impl(svg, canvas, width, height);
}

int drift_skia_svg_dom_get_size(DriftSkiaSVGDOM svg, float* width, float* height) {
    return drift_skia_svg_dom_get_size_impl(svg, width, height);
}

void drift_skia_svg_dom_set_preserve_aspect_ratio(DriftSkiaSVGDOM svg, int align, int scale) {
    drift_skia_svg_dom_set_preserve_aspect_ratio_impl(svg, align, scale);
}

void drift_skia_svg_dom_set_size_to_container(DriftSkiaSVGDOM svg) {
    drift_skia_svg_dom_set_size_to_container_impl(svg);
}

void drift_skia_svg_dom_render_tinted(DriftSkiaSVGDOM svg, DriftSkiaCanvas canvas,
    float width, float height, uint32_t tint_argb) {
    drift_skia_svg_dom_render_tinted_impl(svg, canvas, width, height, tint_argb);
}

}  // extern "C"
