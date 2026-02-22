// Shared rich paragraph implementation for Metal and Vulkan backends.
// This header should only be included from skia_metal.mm and skia_vk.cc.

#ifndef DRIFT_SKIA_RICH_PARAGRAPH_IMPL_H
#define DRIFT_SKIA_RICH_PARAGRAPH_IMPL_H

#include "../skia_bridge.h"
#include "modules/skparagraph/include/ParagraphBuilder.h"
#include "modules/skparagraph/include/ParagraphStyle.h"
#include "modules/skparagraph/include/TextStyle.h"
#include "modules/skunicode/include/SkUnicode_libgrapheme.h"
#include <algorithm>
#include <vector>

inline skia::textlayout::TextStyle span_to_text_style_impl(const DriftTextSpan& span) {
    skia::textlayout::TextStyle text_style;
    text_style.setFontSize(span.size > 0 ? span.size : 16.0f);
    SkFontStyle::Slant slant = (span.style == 1) ? SkFontStyle::kItalic_Slant : SkFontStyle::kUpright_Slant;
    int weight = std::clamp(span.weight > 0 ? span.weight : 400, 100, 900);
    text_style.setFontStyle(SkFontStyle(weight, SkFontStyle::kNormal_Width, slant));
    if (span.family && span.family[0] != '\0') {
        std::vector<SkString> families;
        families.emplace_back(span.family);
        text_style.setFontFamilies(families);
    }
    auto typeface = resolve_typeface(span.family, weight, span.style);
    if (typeface) {
        text_style.setTypeface(typeface);
    }
    text_style.setColor(to_sk_color(span.color));
    if (span.letter_spacing != 0) {
        text_style.setLetterSpacing(span.letter_spacing);
    }
    if (span.word_spacing != 0) {
        text_style.setWordSpacing(span.word_spacing);
    }
    if (span.height > 0) {
        text_style.setHeight(span.height);
        text_style.setHeightOverride(true);
    }
    if (span.decoration != 0) {
        text_style.setDecoration(static_cast<skia::textlayout::TextDecoration>(span.decoration));
        if (span.decoration_color != 0) {
            text_style.setDecorationColor(to_sk_color(span.decoration_color));
        }
        text_style.setDecorationStyle(static_cast<skia::textlayout::TextDecorationStyle>(span.decoration_style));
    }
    if (span.has_background != 0) {
        SkPaint bg;
        bg.setColor(to_sk_color(span.background_color));
        text_style.setBackgroundPaint(bg);
    }
    return text_style;
}

inline DriftSkiaParagraph drift_skia_rich_paragraph_create_impl(
    const DriftTextSpan* spans,
    int span_count,
    int max_lines,
    int text_align
) {
    if (!spans || span_count <= 0) {
        return nullptr;
    }
    auto collection = get_paragraph_collection();
    if (!collection) {
        return nullptr;
    }
    skia::textlayout::ParagraphStyle paragraph_style;
    if (max_lines > 0) {
        paragraph_style.setMaxLines(static_cast<size_t>(max_lines));
    }
    paragraph_style.setTextAlign(static_cast<skia::textlayout::TextAlign>(text_align));
    auto unicode = SkUnicodes::Libgrapheme::Make();
    auto builder = skia::textlayout::ParagraphBuilder::make(paragraph_style, collection, unicode);
    for (int i = 0; i < span_count; ++i) {
        const auto& span = spans[i];
        builder->pushStyle(span_to_text_style_impl(span));
        if (span.text) {
            builder->addText(span.text);
        }
        builder->pop();
    }
    auto paragraph = builder->Build();
    return paragraph.release();
}

#endif  // DRIFT_SKIA_RICH_PARAGRAPH_IMPL_H
