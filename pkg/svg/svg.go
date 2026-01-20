// Package svg provides SVG loading and rendering.
// Uses oksvg for path geometry parsing, but scans XML directly for
// style attributes since oksvg doesn't handle them reliably.
package svg

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/go-drift/drift/pkg/rendering"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/math/fixed"
)

// Icon represents a loaded SVG icon.
type Icon struct {
	paths   []iconPath
	viewBox rendering.Rect
}

type iconPath struct {
	path        *rendering.Path
	fillColor   rendering.Color
	strokeColor rendering.Color
	strokeWidth float64
	hasFill     bool
	hasStroke   bool
}

// Load parses an SVG from the provided reader.
func Load(r io.Reader) (*Icon, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	style := scanStyle(data)

	svgIcon, err := oksvg.ReadIconStream(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return buildIcon(svgIcon, style), nil
}

// LoadFile parses an SVG from a file path.
func LoadFile(path string) (*Icon, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// ViewBox returns the viewBox of the SVG.
func (i *Icon) ViewBox() rendering.Rect {
	return i.viewBox
}

// Draw renders the SVG onto a canvas within the specified bounds.
// If tintColor is non-zero, it will be used instead of the SVG's colors.
func (i *Icon) Draw(canvas rendering.Canvas, bounds rendering.Rect, tintColor rendering.Color) {
	if len(i.paths) == 0 {
		return
	}

	vb := i.viewBox
	if vb.Width() <= 0 || vb.Height() <= 0 {
		return
	}

	// Scale to fit, maintaining aspect ratio
	scale := min(bounds.Width()/vb.Width(), bounds.Height()/vb.Height())

	// Center within bounds
	scaledW := vb.Width() * scale
	scaledH := vb.Height() * scale
	offsetX := bounds.Left + (bounds.Width()-scaledW)/2
	offsetY := bounds.Top + (bounds.Height()-scaledH)/2

	canvas.Save()
	canvas.Translate(offsetX, offsetY)
	canvas.Scale(scale, scale)
	canvas.Translate(-vb.Left, -vb.Top)

	for _, p := range i.paths {
		if p.hasFill {
			fillColor := tintColor
			if fillColor == 0 {
				fillColor = p.fillColor
			}
			canvas.DrawPath(p.path, rendering.Paint{Color: fillColor, Style: rendering.PaintStyleFill})
		}
		if p.hasStroke {
			strokeColor := tintColor
			if strokeColor == 0 {
				strokeColor = p.strokeColor
			}
			canvas.DrawPath(p.path, rendering.Paint{Color: strokeColor, Style: rendering.PaintStyleStroke, StrokeWidth: p.strokeWidth})
		}
	}

	canvas.Restore()
}

// style holds attributes scanned from XML (oksvg doesn't parse these reliably).
type style struct {
	fill        string
	stroke      string
	strokeWidth float64
	fillRule    string
}

// scanStyle extracts style attributes from SVG XML.
func scanStyle(data []byte) style {
	var s style
	var svgFill string

	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		for _, attr := range start.Attr {
			val := strings.TrimSpace(attr.Value)
			switch attr.Name.Local {
			case "fill":
				if start.Name.Local == "svg" {
					svgFill = val
				} else if start.Name.Local == "path" {
					s.fill = val
				}
			case "stroke":
				if start.Name.Local == "path" {
					s.stroke = val
				}
			case "stroke-width":
				s.strokeWidth, _ = parseFloat(val)
			case "fill-rule":
				s.fillRule = val
			}
		}
	}

	// Inherit fill from svg element if path doesn't specify
	if s.fill == "" && svgFill != "" {
		s.fill = svgFill
	}

	return s
}

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSuffix(s, "px"), 64)
}

func buildIcon(svg *oksvg.SvgIcon, s style) *Icon {
	icon := &Icon{
		viewBox: rendering.Rect{
			Left:   svg.ViewBox.X,
			Top:    svg.ViewBox.Y,
			Right:  svg.ViewBox.X + svg.ViewBox.W,
			Bottom: svg.ViewBox.Y + svg.ViewBox.H,
		},
	}

	fillRule := rendering.FillRuleNonZero
	if strings.EqualFold(s.fillRule, "evenodd") {
		fillRule = rendering.FillRuleEvenOdd
	}

	for _, svgPath := range svg.SVGPaths {
		ip := iconPath{
			path: convertPath(svgPath.Path, fillRule),
		}

		// Use our scanned style, not oksvg's unreliable detection
		if s.fill != "" && !strings.EqualFold(s.fill, "none") {
			ip.hasFill = true
			ip.fillColor = parseColor(s.fill)
		}
		if s.stroke != "" && !strings.EqualFold(s.stroke, "none") {
			ip.hasStroke = true
			ip.strokeColor = parseColor(s.stroke)
			ip.strokeWidth = s.strokeWidth
			if ip.strokeWidth == 0 {
				ip.strokeWidth = 1
			}
		}

		// Default to black fill if nothing specified
		if !ip.hasFill && !ip.hasStroke {
			ip.hasFill = true
			ip.fillColor = rendering.RGBA(0, 0, 0, 255)
		}

		icon.paths = append(icon.paths, ip)
	}

	return icon
}

func parseColor(s string) rendering.Color {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "#") {
		return rendering.RGBA(0, 0, 0, 255) // Default black
	}
	hex := s[1:]
	switch len(hex) {
	case 3:
		r, _ := strconv.ParseUint(hex[0:1]+hex[0:1], 16, 8)
		g, _ := strconv.ParseUint(hex[1:2]+hex[1:2], 16, 8)
		b, _ := strconv.ParseUint(hex[2:3]+hex[2:3], 16, 8)
		return rendering.RGBA(uint8(r), uint8(g), uint8(b), 255)
	case 6:
		r, _ := strconv.ParseUint(hex[0:2], 16, 8)
		g, _ := strconv.ParseUint(hex[2:4], 16, 8)
		b, _ := strconv.ParseUint(hex[4:6], 16, 8)
		return rendering.RGBA(uint8(r), uint8(g), uint8(b), 255)
	}
	return rendering.RGBA(0, 0, 0, 255)
}

func convertPath(rp rasterx.Path, fillRule rendering.PathFillRule) *rendering.Path {
	p := rendering.NewPathWithFillRule(fillRule)
	i := 0
	for i < len(rp) {
		switch rasterx.PathCommand(rp[i]) {
		case rasterx.PathMoveTo:
			p.MoveTo(fixed26ToFloat(rp[i+1]), fixed26ToFloat(rp[i+2]))
			i += 3
		case rasterx.PathLineTo:
			p.LineTo(fixed26ToFloat(rp[i+1]), fixed26ToFloat(rp[i+2]))
			i += 3
		case rasterx.PathQuadTo:
			p.QuadTo(fixed26ToFloat(rp[i+1]), fixed26ToFloat(rp[i+2]), fixed26ToFloat(rp[i+3]), fixed26ToFloat(rp[i+4]))
			i += 5
		case rasterx.PathCubicTo:
			p.CubicTo(fixed26ToFloat(rp[i+1]), fixed26ToFloat(rp[i+2]), fixed26ToFloat(rp[i+3]), fixed26ToFloat(rp[i+4]), fixed26ToFloat(rp[i+5]), fixed26ToFloat(rp[i+6]))
			i += 7
		case rasterx.PathClose:
			p.Close()
			i++
		default:
			i++
		}
	}
	return p
}

func fixed26ToFloat(f fixed.Int26_6) float64 {
	return float64(f) / 64.0
}
