// Drift Skia GL bridge for Android
// Pre-compiled at CI time, not by CGO

#include "../skia_bridge.h"

#include <GLES2/gl2.h>
#include <android/log.h>
#include <algorithm>
#include <cstddef>
#include <cstring>
#include <limits>
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
#include "gpu/ganesh/gl/GrGLBackendSurface.h"
#include "gpu/ganesh/gl/GrGLDirectContext.h"
#include "gpu/ganesh/gl/GrGLInterface.h"
#include "ports/SkFontMgr_android.h"
#include "ports/SkFontMgr_android_ndk.h"
#include "ports/SkFontScanner_FreeType.h"
#include "skia_path_impl.h"
#include "skia_svg_impl.h"

namespace {

namespace textlayout_defaults {
static const std::vector<SkString> kDefaultFontFamilies = {SkString(DEFAULT_FONT_FAMILY)};
}

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

SkPaint make_paint_ext(
    uint32_t argb, int style, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha
) {
    SkPaint paint;
    paint.setAntiAlias(aa != 0);

    // Apply alpha to color (clamp to valid range)
    SkColor color = to_sk_color(argb);
    float clamped_alpha = std::clamp(alpha, 0.0f, 1.0f);
    if (clamped_alpha < 1.0f) {
        int a = static_cast<int>(SkColorGetA(color) * clamped_alpha);
        color = SkColorSetA(color, a);
    }
    paint.setColor(color);

    // Style
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

    // Stroke properties
    if (stroke_width > 0) {
        paint.setStrokeWidth(stroke_width);
    }
    switch (stroke_cap) {
        case 1: paint.setStrokeCap(SkPaint::kRound_Cap); break;
        case 2: paint.setStrokeCap(SkPaint::kSquare_Cap); break;
        default: paint.setStrokeCap(SkPaint::kButt_Cap); break;
    }
    switch (stroke_join) {
        case 1: paint.setStrokeJoin(SkPaint::kRound_Join); break;
        case 2: paint.setStrokeJoin(SkPaint::kBevel_Join); break;
        default: paint.setStrokeJoin(SkPaint::kMiter_Join); break;
    }
    if (miter_limit > 0) {
        paint.setStrokeMiter(miter_limit);
    }

    // Dash pattern
    if (dash_intervals && dash_count >= 2) {
        paint.setPathEffect(SkDashPathEffect::Make(SkSpan(dash_intervals, dash_count), dash_phase));
    }

    // Blend mode (Skia's SkBlendMode enum values match our definitions)
    paint.setBlendMode(static_cast<SkBlendMode>(blend_mode));

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

struct ParagraphRegistry {
    std::mutex mu;
    sk_sp<skia::textlayout::FontCollection> collection;
};

sk_sp<SkFontMgr> get_font_manager();

FontRegistry& font_registry() {
    static FontRegistry registry;
    return registry;
}

ParagraphRegistry& paragraph_registry() {
    static ParagraphRegistry registry;
    return registry;
}

sk_sp<skia::textlayout::FontCollection> get_paragraph_collection() {
    auto& registry = paragraph_registry();
    std::lock_guard<std::mutex> lock(registry.mu);
    if (!registry.collection) {
        registry.collection = sk_make_sp<skia::textlayout::FontCollection>();
        registry.collection->setDefaultFontManager(get_font_manager());
    }
    return registry.collection;
}

void register_paragraph_typeface(const char* name, const sk_sp<SkTypeface>& typeface) {
    (void)name;
    (void)typeface;
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

SkSamplingOptions make_sampling_options(int quality) {
    switch (quality) {
        case 0: return SkSamplingOptions(SkFilterMode::kNearest);
        case 1: return SkSamplingOptions(SkFilterMode::kLinear);
        case 2: return SkSamplingOptions(SkFilterMode::kLinear, SkMipmapMode::kLinear);
        case 3: return SkSamplingOptions(SkCubicResampler::Mitchell());
        default: return SkSamplingOptions(SkFilterMode::kLinear);
    }
}

struct ImageCache {
    uintptr_t key = 0;
    sk_sp<SkImage> image;
    int width = 0, height = 0;
};
thread_local ImageCache g_image_cache;

// Filter type constants (must match Go constants in filter_encode.go)
constexpr float kColorFilterBlend = 0;
constexpr float kColorFilterMatrix = 1;

constexpr float kImageFilterBlur = 0;
constexpr float kImageFilterDropShadow = 1;
constexpr float kImageFilterColorFilter = 2;

// Parse serialized ColorFilter data and create SkColorFilter
// Returns nullptr if data is invalid or empty
sk_sp<SkColorFilter> parse_color_filter(const float* data, int len, int& consumed) {
    consumed = 0;
    if (!data || len < 1) {
        return nullptr;
    }

    float type = data[0];
    sk_sp<SkColorFilter> filter;

    if (type == kColorFilterBlend) {
        // Format: [0, color_bits, blend_mode, inner_len, ...inner]
        if (len < 4) return nullptr;
        uint32_t color_bits;
        std::memcpy(&color_bits, &data[1], sizeof(float));
        SkColor color = to_sk_color(color_bits);
        SkBlendMode mode = static_cast<SkBlendMode>(static_cast<int>(data[2]));
        filter = SkColorFilters::Blend(color, mode);
        consumed = 3;
    } else if (type == kColorFilterMatrix) {
        // Format: [1, m0..m19, inner_len, ...inner]
        if (len < 22) return nullptr;
        float matrix[20];
        for (int i = 0; i < 20; ++i) {
            matrix[i] = data[1 + i];
        }
        filter = SkColorFilters::Matrix(matrix);
        consumed = 21;
    } else {
        return nullptr;
    }

    // Parse inner filter for composition
    if (consumed < len) {
        int inner_len = static_cast<int>(data[consumed]);
        consumed += 1;
        if (inner_len > 0 && consumed + inner_len <= len) {
            int inner_consumed = 0;
            auto inner = parse_color_filter(data + consumed, inner_len, inner_consumed);
            if (inner) {
                filter = SkColorFilters::Compose(filter, inner);
            }
            consumed += inner_len;
        }
    }

    return filter;
}

// Parse serialized ImageFilter data and create SkImageFilter
// Returns nullptr if data is invalid or empty
sk_sp<SkImageFilter> parse_image_filter(const float* data, int len, int& consumed) {
    consumed = 0;
    if (!data || len < 1) {
        return nullptr;
    }

    float type = data[0];
    sk_sp<SkImageFilter> filter;
    int base_consumed = 0;

    if (type == kImageFilterBlur) {
        // Format: [0, sigma_x, sigma_y, tile_mode, input_len, ...input]
        if (len < 5) return nullptr;
        float sigma_x = data[1];
        float sigma_y = data[2];
        int tile_mode_int = static_cast<int>(data[3]);
        SkTileMode tile_mode;
        switch (tile_mode_int) {
            case 1: tile_mode = SkTileMode::kRepeat; break;
            case 2: tile_mode = SkTileMode::kMirror; break;
            case 3: tile_mode = SkTileMode::kDecal; break;
            default: tile_mode = SkTileMode::kClamp; break;
        }
        base_consumed = 4;

        // Parse input filter
        int input_len = static_cast<int>(data[base_consumed]);
        base_consumed += 1;
        sk_sp<SkImageFilter> input;
        if (input_len > 0 && base_consumed + input_len <= len) {
            int input_consumed = 0;
            input = parse_image_filter(data + base_consumed, input_len, input_consumed);
            base_consumed += input_len;
        }

        filter = SkImageFilters::Blur(sigma_x, sigma_y, tile_mode, input);
        consumed = base_consumed;

    } else if (type == kImageFilterDropShadow) {
        // Format: [1, dx, dy, sigma_x, sigma_y, color_bits, shadow_only, input_len, ...input]
        if (len < 8) return nullptr;
        float dx = data[1];
        float dy = data[2];
        float sigma_x = data[3];
        float sigma_y = data[4];
        uint32_t color_bits;
        std::memcpy(&color_bits, &data[5], sizeof(float));
        SkColor color = to_sk_color(color_bits);
        bool shadow_only = data[6] != 0;
        base_consumed = 7;

        // Parse input filter
        int input_len = static_cast<int>(data[base_consumed]);
        base_consumed += 1;
        sk_sp<SkImageFilter> input;
        if (input_len > 0 && base_consumed + input_len <= len) {
            int input_consumed = 0;
            input = parse_image_filter(data + base_consumed, input_len, input_consumed);
            base_consumed += input_len;
        }

        if (shadow_only) {
            filter = SkImageFilters::DropShadowOnly(dx, dy, sigma_x, sigma_y, color, input);
        } else {
            filter = SkImageFilters::DropShadow(dx, dy, sigma_x, sigma_y, color, input);
        }
        consumed = base_consumed;

    } else if (type == kImageFilterColorFilter) {
        // Format: [2, cf_len, ...cf_encoding, input_len, ...input]
        if (len < 3) return nullptr;
        int cf_len = static_cast<int>(data[1]);
        base_consumed = 2;

        sk_sp<SkColorFilter> cf;
        if (cf_len > 0 && base_consumed + cf_len <= len) {
            int cf_consumed = 0;
            cf = parse_color_filter(data + base_consumed, cf_len, cf_consumed);
            base_consumed += cf_len;
        }

        // Parse input filter
        if (base_consumed < len) {
            int input_len = static_cast<int>(data[base_consumed]);
            base_consumed += 1;
            sk_sp<SkImageFilter> input;
            if (input_len > 0 && base_consumed + input_len <= len) {
                int input_consumed = 0;
                input = parse_image_filter(data + base_consumed, input_len, input_consumed);
                base_consumed += input_len;
            }
            filter = SkImageFilters::ColorFilter(cf, input);
        } else {
            filter = SkImageFilters::ColorFilter(cf, nullptr);
        }
        consumed = base_consumed;

    } else {
        return nullptr;
    }

    return filter;
}

}  // namespace

// Provide a weak definition for the default font families used by skparagraph.
// This allows the paragraph module to fall back to our configured default font
// when no explicit font family is specified in the text style.
const std::vector<SkString>* ::skia::textlayout::TextStyle::kDefaultFontFamilies __attribute__((weak)) = &textlayout_defaults::kDefaultFontFamilies;

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

void drift_skia_canvas_clip_path(
    DriftSkiaCanvas canvas,
    DriftSkiaPath path,
    int clip_op,
    int antialias
) {
    if (!canvas || !path) {
        return;
    }
    SkClipOp op = clip_op == 1 ? SkClipOp::kDifference : SkClipOp::kIntersect;
    reinterpret_cast<SkCanvas*>(canvas)->clipPath(
        drift_skia_path_snapshot(path),
        op,
        antialias != 0
    );
}

void drift_skia_canvas_save_layer(
    DriftSkiaCanvas canvas,
    float l, float t, float r, float b,
    int blend_mode,
    float alpha
) {
    if (!canvas) {
        return;
    }

    SkRect bounds = SkRect::MakeLTRB(l, t, r, b);
    SkRect* boundsPtr = (l == 0 && t == 0 && r == 0 && b == 0) ? nullptr : &bounds;

    SkPaint paint;
    paint.setBlendMode(static_cast<SkBlendMode>(blend_mode));
    if (alpha < 1.0f) {
        paint.setAlphaf(alpha);
    }

    reinterpret_cast<SkCanvas*>(canvas)->saveLayer(boundsPtr, &paint);
}

void drift_skia_canvas_save_layer_filtered(
    DriftSkiaCanvas canvas,
    float l, float t, float r, float b,
    int blend_mode,
    float alpha,
    const float* color_filter_data, int color_filter_len,
    const float* image_filter_data, int image_filter_len
) {
    if (!canvas) {
        return;
    }

    SkRect bounds = SkRect::MakeLTRB(l, t, r, b);
    SkRect* boundsPtr = (l == 0 && t == 0 && r == 0 && b == 0) ? nullptr : &bounds;

    SkPaint paint;
    paint.setBlendMode(static_cast<SkBlendMode>(blend_mode));
    if (alpha < 1.0f) {
        paint.setAlphaf(alpha);
    }

    // Parse and apply color filter
    if (color_filter_data && color_filter_len > 0) {
        int consumed = 0;
        auto cf = parse_color_filter(color_filter_data, color_filter_len, consumed);
        if (cf) {
            paint.setColorFilter(cf);
        }
    }

    // Parse and apply image filter
    if (image_filter_data && image_filter_len > 0) {
        int consumed = 0;
        auto imf = parse_image_filter(image_filter_data, image_filter_len, consumed);
        if (imf) {
            paint.setImageFilter(imf);
        }
    }

    reinterpret_cast<SkCanvas*>(canvas)->saveLayer(boundsPtr, &paint);
}

void drift_skia_canvas_clear(DriftSkiaCanvas canvas, uint32_t argb) {
    if (!canvas) {
        return;
    }
    reinterpret_cast<SkCanvas*>(canvas)->clear(to_sk_color(argb));
}

void drift_skia_canvas_draw_rect(
    DriftSkiaCanvas canvas, float l, float t, float r, float b,
    uint32_t argb, int style, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha
) {
    if (!canvas) {
        return;
    }
    SkRect rect = SkRect::MakeLTRB(l, t, r, b);
    SkPaint paint = make_paint_ext(argb, style, stroke_width, aa,
        stroke_cap, stroke_join, miter_limit,
        dash_intervals, dash_count, dash_phase,
        blend_mode, alpha);
    reinterpret_cast<SkCanvas*>(canvas)->drawRect(rect, paint);
}

void drift_skia_canvas_draw_rrect(
    DriftSkiaCanvas canvas,
    float l, float t, float r, float b,
    float rx1, float ry1, float rx2, float ry2,
    float rx3, float ry3, float rx4, float ry4,
    uint32_t argb, int style, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha
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
    SkPaint paint = make_paint_ext(argb, style, stroke_width, aa,
        stroke_cap, stroke_join, miter_limit,
        dash_intervals, dash_count, dash_phase,
        blend_mode, alpha);
    reinterpret_cast<SkCanvas*>(canvas)->drawRRect(rrect, paint);
}

void drift_skia_canvas_draw_circle(
    DriftSkiaCanvas canvas, float cx, float cy, float radius,
    uint32_t argb, int style, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha
) {
    if (!canvas) {
        return;
    }
    SkPaint paint = make_paint_ext(argb, style, stroke_width, aa,
        stroke_cap, stroke_join, miter_limit,
        dash_intervals, dash_count, dash_phase,
        blend_mode, alpha);
    reinterpret_cast<SkCanvas*>(canvas)->drawCircle(cx, cy, radius, paint);
}

void drift_skia_canvas_draw_line(
    DriftSkiaCanvas canvas, float x1, float y1, float x2, float y2,
    uint32_t argb, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha
) {
    if (!canvas) {
        return;
    }
    SkPaint paint = make_paint_ext(argb, 1, stroke_width, aa,
        stroke_cap, stroke_join, miter_limit,
        dash_intervals, dash_count, dash_phase,
        blend_mode, alpha);
    reinterpret_cast<SkCanvas*>(canvas)->drawLine(x1, y1, x2, y2, paint);
}

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
) {
    if (!canvas) {
        return;
    }
    SkRect rect = SkRect::MakeLTRB(l, t, r, b);
    SkPaint paint = make_paint_ext(argb, style, stroke_width, aa,
        stroke_cap, stroke_join, miter_limit,
        dash_intervals, dash_count, dash_phase,
        blend_mode, alpha);
    auto shader = make_gradient_shader(gradient_type, x1, y1, x2, y2, cx, cy, radius, colors, positions, count);
    if (shader) {
        paint.setShader(shader);
    }
    reinterpret_cast<SkCanvas*>(canvas)->drawRect(rect, paint);
}

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
    SkPaint paint = make_paint_ext(argb, style, stroke_width, aa,
        stroke_cap, stroke_join, miter_limit,
        dash_intervals, dash_count, dash_phase,
        blend_mode, alpha);
    auto shader = make_gradient_shader(gradient_type, x1, y1, x2, y2, cx, cy, radius, colors, positions, count);
    if (shader) {
        paint.setShader(shader);
    }
    reinterpret_cast<SkCanvas*>(canvas)->drawRRect(rrect, paint);
}

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
) {
    if (!canvas) {
        return;
    }
    SkPaint paint = make_paint_ext(argb, style, stroke_width, aa,
        stroke_cap, stroke_join, miter_limit,
        dash_intervals, dash_count, dash_phase,
        blend_mode, alpha);
    auto shader = make_gradient_shader(gradient_type, x1, y1, x2, y2, rcx, rcy, rradius, colors, positions, count);
    if (shader) {
        paint.setShader(shader);
    }
    reinterpret_cast<SkCanvas*>(canvas)->drawCircle(cx, cy, radius, paint);
}

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
) {
    if (!canvas) {
        return;
    }
    SkPaint paint = make_paint_ext(argb, 1, stroke_width, aa,
        stroke_cap, stroke_join, miter_limit,
        dash_intervals, dash_count, dash_phase,
        blend_mode, alpha);
    auto shader = make_gradient_shader(gradient_type, lx1, ly1, lx2, ly2, rcx, rcy, rradius, colors, positions, count);
    if (shader) {
        paint.setShader(shader);
    }
    reinterpret_cast<SkCanvas*>(canvas)->drawLine(x1, y1, x2, y2, paint);
}

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
) {
    if (!canvas || !path) {
        return;
    }
    SkPaint paint = make_paint_ext(argb, style, stroke_width, aa,
        stroke_cap, stroke_join, miter_limit,
        dash_intervals, dash_count, dash_phase,
        blend_mode, alpha);
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

void drift_skia_canvas_draw_image_rect(
    DriftSkiaCanvas canvas,
    const uint8_t* pixels, int width, int height, int stride,
    float src_l, float src_t, float src_r, float src_b,
    float dst_l, float dst_t, float dst_r, float dst_b,
    int filter_quality,
    uintptr_t cache_key
) {
    if (!canvas || !pixels || width <= 0 || height <= 0 || stride <= 0) {
        return;
    }

    sk_sp<SkImage> image;
    if (cache_key != 0 && g_image_cache.key == cache_key &&
        g_image_cache.width == width && g_image_cache.height == height) {
        image = g_image_cache.image;
    } else {
        SkImageInfo info = SkImageInfo::Make(width, height, kRGBA_8888_SkColorType, kPremul_SkAlphaType);
        auto data = SkData::MakeWithCopy(pixels, static_cast<size_t>(stride) * height);
        if (!data) {
            return;
        }
        image = SkImages::RasterFromData(info, data, stride);
        if (!image) {
            return;
        }
        if (cache_key != 0) {
            g_image_cache = {cache_key, image, width, height};
        }
    }

    SkRect srcRect = (src_l == 0 && src_t == 0 && src_r == 0 && src_b == 0)
        ? SkRect::MakeWH(width, height)
        : SkRect::MakeLTRB(src_l, src_t, src_r, src_b);
    SkRect dstRect = SkRect::MakeLTRB(dst_l, dst_t, dst_r, dst_b);

    auto sampling = make_sampling_options(filter_quality);
    reinterpret_cast<SkCanvas*>(canvas)->drawImageRect(
        image, srcRect, dstRect, sampling, nullptr, SkCanvas::kStrict_SrcRectConstraint);
}

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
) {
    auto collection = get_paragraph_collection();
    if (!collection) {
        return nullptr;
    }
    skia::textlayout::ParagraphStyle paragraph_style;
    if (max_lines > 0) {
        paragraph_style.setMaxLines(static_cast<size_t>(max_lines));
    }
    // text_align values match skia::textlayout::TextAlign enum directly.
    paragraph_style.setTextAlign(static_cast<skia::textlayout::TextAlign>(text_align));
    skia::textlayout::TextStyle text_style;
    text_style.setFontSize(size);
    SkFontStyle::Slant slant = (style == 1) ? SkFontStyle::kItalic_Slant : SkFontStyle::kUpright_Slant;
    text_style.setFontStyle(SkFontStyle(std::clamp(weight, 100, 900), SkFontStyle::kNormal_Width, slant));
    if (family && family[0] != '\0') {
        std::vector<SkString> families;
        families.emplace_back(family);
        text_style.setFontFamilies(families);
    }
    auto typeface = resolve_typeface(family, weight, style);
    if (typeface) {
        text_style.setTypeface(typeface);
    }
    text_style.setColor(to_sk_color(argb));
    auto shader = make_gradient_shader(gradient_type, x1, y1, x2, y2, cx, cy, radius, colors, positions, count);
    if (shader) {
        SkPaint paint;
        paint.setAntiAlias(true);
        paint.setColor(to_sk_color(argb));
        paint.setShader(shader);
        text_style.setForegroundPaint(paint);
    }
    if (shadow_enabled != 0) {
        skia::textlayout::TextShadow shadow;
        shadow.fColor = to_sk_color(shadow_argb);
        shadow.fOffset = SkPoint::Make(shadow_dx, shadow_dy);
        shadow.fBlurSigma = shadow_sigma;
        text_style.addShadow(shadow);
    }
    auto unicode = SkUnicodes::Libgrapheme::Make();
    auto builder = skia::textlayout::ParagraphBuilder::make(paragraph_style, collection, unicode);
    builder->pushStyle(text_style);
    if (text) {
        builder->addText(text);
    }
    builder->pop();
    auto paragraph = builder->Build();
    return paragraph.release();
}

void drift_skia_paragraph_layout(DriftSkiaParagraph paragraph, float width) {
    if (!paragraph) {
        return;
    }
    if (width <= 0) {
        width = std::numeric_limits<float>::max();
    }
    reinterpret_cast<skia::textlayout::Paragraph*>(paragraph)->layout(width);
}

int drift_skia_paragraph_get_metrics(DriftSkiaParagraph paragraph, float* height, float* longest_line, float* max_intrinsic_width, int* line_count) {
    if (!paragraph || !height || !longest_line || !max_intrinsic_width || !line_count) {
        return 0;
    }
    auto sk_paragraph = reinterpret_cast<skia::textlayout::Paragraph*>(paragraph);
    *height = sk_paragraph->getHeight();
    *longest_line = sk_paragraph->getLongestLine();
    *max_intrinsic_width = sk_paragraph->getMaxIntrinsicWidth();
    std::vector<skia::textlayout::LineMetrics> metrics;
    sk_paragraph->getLineMetrics(metrics);
    *line_count = static_cast<int>(metrics.size());
    return 1;
}

int drift_skia_paragraph_get_line_metrics(DriftSkiaParagraph paragraph, float* widths, float* ascents, float* descents, float* heights, int count) {
    if (!paragraph || !widths || !ascents || !descents || !heights || count <= 0) {
        return 0;
    }
    auto sk_paragraph = reinterpret_cast<skia::textlayout::Paragraph*>(paragraph);
    std::vector<skia::textlayout::LineMetrics> metrics;
    sk_paragraph->getLineMetrics(metrics);
    int lines = std::min(count, static_cast<int>(metrics.size()));
    for (int i = 0; i < lines; ++i) {
        widths[i] = metrics[i].fWidth;
        ascents[i] = metrics[i].fAscent;
        descents[i] = metrics[i].fDescent;
        heights[i] = metrics[i].fHeight;
    }
    return 1;
}

void drift_skia_paragraph_paint(DriftSkiaParagraph paragraph, DriftSkiaCanvas canvas, float x, float y) {
    if (!paragraph || !canvas) {
        return;
    }
    reinterpret_cast<skia::textlayout::Paragraph*>(paragraph)->paint(reinterpret_cast<SkCanvas*>(canvas), x, y);
}

void drift_skia_paragraph_destroy(DriftSkiaParagraph paragraph) {
    if (!paragraph) {
        return;
    }
    delete reinterpret_cast<skia::textlayout::Paragraph*>(paragraph);
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

void drift_skia_canvas_draw_path(
    DriftSkiaCanvas canvas, DriftSkiaPath path,
    uint32_t argb, int style, float stroke_width, int aa,
    int stroke_cap, int stroke_join, float miter_limit,
    const float* dash_intervals, int dash_count, float dash_phase,
    int blend_mode, float alpha
) {
    if (!canvas || !path) {
        return;
    }
    SkPaint paint = make_paint_ext(argb, style, stroke_width, aa,
        stroke_cap, stroke_join, miter_limit,
        dash_intervals, dash_count, dash_phase,
        blend_mode, alpha);
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
    auto sk_canvas = reinterpret_cast<SkCanvas*>(canvas);
    sk_canvas->save();

    SkPaint paint;
    paint.setAntiAlias(true);
    paint.setColor(to_sk_color(color));

    // Inner (3) is for inset shadows - clip to bounds and draw a blurred frame.
    // Use a blurred frame (outer - inner) so the shadow is strongest at edges.
    if (blur_style == 3) {
        // No visible effect when both sigma and spread are zero.
        if (sigma <= 0 && spread <= 0) {
            sk_canvas->restore();
            return;
        }

        SkRect origRect = SkRect::MakeLTRB(l, t, r, b);
        sk_canvas->clipRect(origRect, SkClipOp::kIntersect, true);
        sk_canvas->translate(dx, dy);

        // For inset shadows, spread moves the edge inward (expands shadow region).
        SkRect insetRect = SkRect::MakeLTRB(l + spread, t + spread, r - spread, b - spread);
        if (insetRect.isEmpty()) {
            // Spread consumed entire rect; fill with color (fully shadowed).
            sk_canvas->drawRect(origRect, paint);
            sk_canvas->restore();
            return;
        }

        SkPathBuilder frameBuilder;
        if (sigma > 0) {
            float pad = sigma * 3 + spread;
            SkRect outerRect = SkRect::MakeLTRB(l - pad, t - pad, r + pad, b + pad);
            frameBuilder.addRect(outerRect);
            paint.setMaskFilter(SkMaskFilter::MakeBlur(kNormal_SkBlurStyle, sigma));
        } else {
            frameBuilder.addRect(origRect);
        }
        frameBuilder.addRect(insetRect, SkPathDirection::kCCW);
        sk_canvas->drawPath(frameBuilder.detach(), paint);
    } else if (blur_style == 0) {
        // Outer: draw with smooth Normal blur to a temp layer, then erase
        // the box interior via DstOut blending. kOuter_SkBlurStyle creates a
        // hard edge at the shape boundary, and AA difference clips produce
        // visible seam artifacts; this approach avoids both.
        SkRect origRect = SkRect::MakeLTRB(l, t, r, b);
        float pad = sigma > 0 ? sigma * 3 : 0;
        SkRect layerBounds = SkRect::MakeLTRB(
            l + std::min(0.0f, dx) - spread - pad,
            t + std::min(0.0f, dy) - spread - pad,
            r + std::max(0.0f, dx) + spread + pad,
            b + std::max(0.0f, dy) + spread + pad
        );
        sk_canvas->saveLayer(&layerBounds, nullptr);

        sk_canvas->translate(dx, dy);
        SkRect rect = SkRect::MakeLTRB(l - spread, t - spread, r + spread, b + spread);
        if (sigma > 0) {
            paint.setMaskFilter(SkMaskFilter::MakeBlur(kNormal_SkBlurStyle, sigma));
        }
        sk_canvas->drawRect(rect, paint);

        sk_canvas->translate(-dx, -dy);
        SkPaint erasePaint;
        erasePaint.setAntiAlias(true);
        erasePaint.setBlendMode(SkBlendMode::kDstOut);
        erasePaint.setColor(SK_ColorBLACK);
        sk_canvas->drawRect(origRect, erasePaint);

        sk_canvas->restore();
    } else {
        // Normal (1) / Solid (2): clip out the original box bounds so
        // shadow never appears inside the box area.
        SkRect origRect = SkRect::MakeLTRB(l, t, r, b);
        sk_canvas->clipRect(origRect, SkClipOp::kDifference, true);
        sk_canvas->translate(dx, dy);

        SkRect rect = SkRect::MakeLTRB(l - spread, t - spread, r + spread, b + spread);
        if (sigma > 0) {
            SkBlurStyle skStyle;
            switch (blur_style) {
                case 2: skStyle = kSolid_SkBlurStyle; break;
                default: skStyle = kNormal_SkBlurStyle; break;
            }
            paint.setMaskFilter(SkMaskFilter::MakeBlur(skStyle, sigma));
        }
        sk_canvas->drawRect(rect, paint);
    }

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
    auto sk_canvas = reinterpret_cast<SkCanvas*>(canvas);
    sk_canvas->save();

    SkPaint paint;
    paint.setAntiAlias(true);
    paint.setColor(to_sk_color(color));

    // Inner (3) is for inset shadows - clip to bounds and draw a blurred frame.
    // Use a blurred frame (outer - inner) so the shadow is strongest at edges.
    if (blur_style == 3) {
        // No visible effect when both sigma and spread are zero.
        if (sigma <= 0 && spread <= 0) {
            sk_canvas->restore();
            return;
        }

        SkRect origRect = SkRect::MakeLTRB(l, t, r, b);
        SkVector origRadii[4] = {{rx1, ry1}, {rx2, ry2}, {rx3, ry3}, {rx4, ry4}};
        SkRRect origRRect;
        origRRect.setRectRadii(origRect, origRadii);
        sk_canvas->clipRRect(origRRect, SkClipOp::kIntersect, true);
        sk_canvas->translate(dx, dy);

        // For inset shadows, spread moves the edge inward (expands shadow region).
        SkRect insetRect = SkRect::MakeLTRB(l + spread, t + spread, r - spread, b - spread);
        if (insetRect.isEmpty()) {
            // Spread consumed entire rect; fill with color (fully shadowed).
            sk_canvas->drawRRect(origRRect, paint);
            sk_canvas->restore();
            return;
        }

        float maxRadiusX = insetRect.width() / 2;
        float maxRadiusY = insetRect.height() / 2;
        SkVector insetRadii[4] = {
            {std::min(maxRadiusX, std::max(0.0f, rx1 - spread)), std::min(maxRadiusY, std::max(0.0f, ry1 - spread))},
            {std::min(maxRadiusX, std::max(0.0f, rx2 - spread)), std::min(maxRadiusY, std::max(0.0f, ry2 - spread))},
            {std::min(maxRadiusX, std::max(0.0f, rx3 - spread)), std::min(maxRadiusY, std::max(0.0f, ry3 - spread))},
            {std::min(maxRadiusX, std::max(0.0f, rx4 - spread)), std::min(maxRadiusY, std::max(0.0f, ry4 - spread))}
        };
        SkRRect insetRRect;
        insetRRect.setRectRadii(insetRect, insetRadii);

        SkPathBuilder frameBuilder;
        if (sigma > 0) {
            float pad = sigma * 3 + spread;
            SkRect outerRect = SkRect::MakeLTRB(l - pad, t - pad, r + pad, b + pad);
            SkRRect outerRRect;
            outerRRect.setRectRadii(outerRect, origRadii);
            frameBuilder.addRRect(outerRRect);
            paint.setMaskFilter(SkMaskFilter::MakeBlur(kNormal_SkBlurStyle, sigma));
        } else {
            frameBuilder.addRRect(origRRect);
        }
        frameBuilder.addRRect(insetRRect, SkPathDirection::kCCW);
        sk_canvas->drawPath(frameBuilder.detach(), paint);
    } else if (blur_style == 0) {
        // Outer: draw with smooth Normal blur to a temp layer, then erase
        // the box interior via DstOut blending. kOuter_SkBlurStyle creates a
        // hard edge at the shape boundary, and AA difference clips produce
        // visible seam artifacts; this approach avoids both.
        SkRect origRect = SkRect::MakeLTRB(l, t, r, b);
        SkVector origRadii[4] = {{rx1, ry1}, {rx2, ry2}, {rx3, ry3}, {rx4, ry4}};
        SkRRect origRRect;
        origRRect.setRectRadii(origRect, origRadii);

        float pad = sigma > 0 ? sigma * 3 : 0;
        SkRect layerBounds = SkRect::MakeLTRB(
            l + std::min(0.0f, dx) - spread - pad,
            t + std::min(0.0f, dy) - spread - pad,
            r + std::max(0.0f, dx) + spread + pad,
            b + std::max(0.0f, dy) + spread + pad
        );
        sk_canvas->saveLayer(&layerBounds, nullptr);

        sk_canvas->translate(dx, dy);
        SkRect rect = SkRect::MakeLTRB(l - spread, t - spread, r + spread, b + spread);
        SkVector radii[4] = {
            {rx1 + spread, ry1 + spread},
            {rx2 + spread, ry2 + spread},
            {rx3 + spread, ry3 + spread},
            {rx4 + spread, ry4 + spread}
        };
        SkRRect rrect;
        rrect.setRectRadii(rect, radii);
        if (sigma > 0) {
            paint.setMaskFilter(SkMaskFilter::MakeBlur(kNormal_SkBlurStyle, sigma));
        }
        sk_canvas->drawRRect(rrect, paint);

        sk_canvas->translate(-dx, -dy);
        SkPaint erasePaint;
        erasePaint.setAntiAlias(true);
        erasePaint.setBlendMode(SkBlendMode::kDstOut);
        erasePaint.setColor(SK_ColorBLACK);
        sk_canvas->drawRRect(origRRect, erasePaint);

        sk_canvas->restore();
    } else {
        // Normal (1) / Solid (2): clip out the original box bounds so
        // shadow never appears inside the box area.
        SkRect origRect = SkRect::MakeLTRB(l, t, r, b);
        SkVector origRadii[4] = {{rx1, ry1}, {rx2, ry2}, {rx3, ry3}, {rx4, ry4}};
        SkRRect origRRect;
        origRRect.setRectRadii(origRect, origRadii);
        sk_canvas->clipRRect(origRRect, SkClipOp::kDifference, true);
        sk_canvas->translate(dx, dy);

        SkRect rect = SkRect::MakeLTRB(l - spread, t - spread, r + spread, b + spread);
        SkVector radii[4] = {
            {rx1 + spread, ry1 + spread},
            {rx2 + spread, ry2 + spread},
            {rx3 + spread, ry3 + spread},
            {rx4 + spread, ry4 + spread}
        };
        SkRRect rrect;
        rrect.setRectRadii(rect, radii);

        if (sigma > 0) {
            SkBlurStyle skStyle;
            switch (blur_style) {
                case 2: skStyle = kSolid_SkBlurStyle; break;
                default: skStyle = kNormal_SkBlurStyle; break;
            }
            paint.setMaskFilter(SkMaskFilter::MakeBlur(skStyle, sigma));
        }
        sk_canvas->drawRRect(rrect, paint);
    }

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

DriftSkiaSurface drift_skia_surface_create_offscreen_gl(DriftSkiaContext ctx, int width, int height) {
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

DriftSkiaSurface drift_skia_surface_create_offscreen_metal(DriftSkiaContext ctx, int width, int height) {
    (void)ctx;
    (void)width;
    (void)height;
    return nullptr;
}

void drift_skia_context_flush_and_submit(DriftSkiaContext ctx, int sync_cpu) {
    if (!ctx) {
        return;
    }
    auto context = reinterpret_cast<GrDirectContext*>(ctx);
    context->flushAndSubmit(sync_cpu ? GrSyncCpu::kYes : GrSyncCpu::kNo);
}

int drift_skia_gl_get_framebuffer_binding(void) {
    GLint fbo = 0;
    glGetIntegerv(GL_FRAMEBUFFER_BINDING, &fbo);
    return static_cast<int>(fbo);
}

void drift_skia_gl_bind_framebuffer(int fbo) {
    glBindFramebuffer(GL_FRAMEBUFFER, static_cast<GLuint>(fbo));
}

void drift_skia_context_purge_resources(DriftSkiaContext ctx) {
    if (!ctx) {
        return;
    }
    auto context = reinterpret_cast<GrDirectContext*>(ctx);
    context->resetContext();
    context->freeGpuResources();
}

}  // extern "C"
