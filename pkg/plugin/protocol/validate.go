package protocol

import (
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
)

// Every op validates its own fields (Op.Validate). The recorders on
// pkg/plugin's BuildCtx run it at record time and DecodeOps runs it again
// on ops arriving from the bridge, so mutators only ever see valid ops and
// never re-check. The helpers below are the shared vocabulary.

// Drift-owned Android value resource files, relative to res/. The CLI
// writes them from color, string and style ops, so WriteXML may not target
// them.
const (
	AndroidPluginColorsFile  = "values/plugin_colors.xml"
	AndroidPluginStringsFile = "values/plugin_strings.xml"
	AndroidPluginStylesFile  = "values/plugin_styles.xml"
)

var (
	identRe       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	dottedIdentRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)
	// An Android activity name: fully qualified, or relative to the app
	// package with a leading dot (".MainActivity").
	activityNameRe = regexp.MustCompile(`^\.?[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)
	sourceGroupRe  = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	// Value resource names (colors, strings, styles) may contain dots
	// ("Theme.App"); aapt maps them to underscores in R.
	valueResNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)
	// File-based resource names must be lowercase; an optional extension
	// (including nine-patch ".9.png") selects the format.
	drawableNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.9)?(\.(png|jpg|jpeg|webp|gif))?$`)
	resDirRe       = regexp.MustCompile(`^[a-z]+(-[A-Za-z0-9+]+)*$`)
	resFileRe      = regexp.MustCompile(`^[a-z][a-z0-9_]*\.xml$`)
	// Namespaced manifest or style attribute ("android:theme") or a bare
	// app attribute ("windowSplashScreenBackground").
	attrNameRe   = regexp.MustCompile(`^([a-z]+:)?[A-Za-z_][A-Za-z0-9_]*$`)
	gradleTokRe  = regexp.MustCompile(`^[A-Za-z0-9_.+\-\[\](),]+$`)
	gradleIDRe   = regexp.MustCompile(`^[A-Za-z0-9_.\-]+$`)
	metaParentRe = regexp.MustCompile(`^(application|activity:\.?[A-Za-z_][A-Za-z0-9_.]*)$`)
)

func checkMatch(re *regexp.Regexp, what, v string) error {
	if !re.MatchString(v) {
		return fmt.Errorf("%s %q is invalid (want %s)", what, v, re.String())
	}
	return nil
}

func checkNonEmpty(what, v string) error {
	if v == "" {
		return fmt.Errorf("%s is empty", what)
	}
	return nil
}

// checkRelPath requires a canonical, slash-separated relative path: no
// empty, ".", ".." or repeated segments, no leading or trailing slash, no
// backslash. Canonical input lets the CLI join it under a root and key
// conflicts on it without normalising.
func checkRelPath(what, p string) error {
	if p == "" {
		return fmt.Errorf("%s is empty", what)
	}
	if strings.Contains(p, "\\") {
		return fmt.Errorf("%s %q must use forward slashes", what, p)
	}
	if path.IsAbs(p) || (len(p) >= 2 && p[1] == ':') {
		return fmt.Errorf("%s %q must be relative", what, p)
	}
	for seg := range strings.SplitSeq(p, "/") {
		if seg == ".." {
			return fmt.Errorf("%s %q contains a `..` segment", what, p)
		}
	}
	if path.Clean(p) != p || p == "." {
		return fmt.Errorf("%s %q is not canonical (want %q)", what, p, path.Clean(p))
	}
	return nil
}

// checkFileName requires a plain file name with no directory part.
func checkFileName(what, name string) error {
	if name == "" {
		return fmt.Errorf("%s is empty", what)
	}
	if strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return fmt.Errorf("%s %q must be a plain file name", what, name)
	}
	return nil
}

// checkContent requires s to be valid base64 (the wire form of file bytes),
// and non-empty when required.
func checkContent(what, s string, required bool) error {
	if s == "" {
		if required {
			return fmt.Errorf("%s is empty", what)
		}
		return nil
	}
	if _, err := base64.StdEncoding.DecodeString(s); err != nil {
		return fmt.Errorf("%s is not valid base64: %w", what, err)
	}
	return nil
}

// checkPlistValue requires v to be encodable in a property list. Values
// arrive either from a plugin's Go map or from JSON, so both concrete Go
// number types and JSON's float64 are accepted.
func checkPlistValue(where string, v any) error {
	switch t := v.(type) {
	case string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return nil
	case []any:
		for i, e := range t {
			if err := checkPlistValue(fmt.Sprintf("%s[%d]", where, i), e); err != nil {
				return err
			}
		}
		return nil
	case []string:
		return nil
	case map[string]any:
		for k, e := range t {
			if k == "" {
				return fmt.Errorf("%s has an empty key", where)
			}
			if err := checkPlistValue(where+"."+k, e); err != nil {
				return err
			}
		}
		return nil
	case map[string]string:
		return nil
	case nil:
		return fmt.Errorf("%s is null, which a property list cannot represent", where)
	default:
		return fmt.Errorf("%s has type %T, which a property list cannot represent", where, v)
	}
}

// checkXMLRoot requires s to be well-formed XML whose root element is named
// root (ignoring any namespace prefix).
func checkXMLRoot(what, s, root string) error {
	dec := xml.NewDecoder(strings.NewReader(s))
	var got string
	depth := 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("%s is not well-formed XML: %w", what, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if depth == 0 {
				if got != "" {
					return fmt.Errorf("%s has more than one root element", what)
				}
				got = t.Name.Local
			}
			depth++
		case xml.EndElement:
			depth--
		}
	}
	if got != root {
		return fmt.Errorf("%s root element is %q, want %q", what, got, root)
	}
	return nil
}

// checkGradleCoord requires group:artifact with an optional version and
// classifier, every part free of characters that would break out of the
// Groovy string the mutator emits.
func checkGradleCoord(coord string) error {
	parts := strings.Split(coord, ":")
	if len(parts) < 2 || len(parts) > 4 {
		return fmt.Errorf("gradle coordinate %q must be group:artifact[:version[:classifier]]", coord)
	}
	for _, p := range parts {
		if err := checkMatch(gradleTokRe, "gradle coordinate part", p); err != nil {
			return fmt.Errorf("gradle coordinate %q: %w", coord, err)
		}
	}
	return nil
}
