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
#include "skia_skottie_impl.h"
#include "skia_svg_impl.h"

namespace {

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

#include "skia_common_impl.h"

}  // namespace

// Provide a weak definition for the default font families used by skparagraph.
// This allows the paragraph module to fall back to our configured default font
// when no explicit font family is specified in the text style.
const std::vector<SkString>* ::skia::textlayout::TextStyle::kDefaultFontFamilies __attribute__((weak)) = &textlayout_defaults::kDefaultFontFamilies;

DRIFT_SKIA_DEFINE_COMMON_FUNCTIONS

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

void drift_skia_surface_flush(DriftSkiaContext ctx, DriftSkiaSurface surface) {
    if (!ctx || !surface) {
        return;
    }
    auto sk_surface = reinterpret_cast<SkSurface*>(surface);
    reinterpret_cast<GrDirectContext*>(ctx)->flushAndSubmit(sk_surface);
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
