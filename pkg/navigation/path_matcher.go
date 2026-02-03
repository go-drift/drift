package navigation

import (
	"net/url"
	"strings"
)

// TrailingSlashBehavior controls how trailing slashes are handled during
// path matching.
type TrailingSlashBehavior int

const (
	// TrailingSlashStrip removes trailing slashes before matching.
	// This is the default behavior, making "/products/1/" match "/products/:id".
	TrailingSlashStrip TrailingSlashBehavior = iota

	// TrailingSlashStrict requires exact trailing slash match.
	// With this setting, "/products/1/" does NOT match "/products/:id".
	TrailingSlashStrict
)

// CaseSensitivity controls whether path matching is case-sensitive.
type CaseSensitivity int

const (
	// CaseSensitive requires exact case match in path segments.
	// This is the default behavior: "/Products" does NOT match "/products".
	CaseSensitive CaseSensitivity = iota

	// CaseInsensitive ignores case when matching path segments.
	// With this setting, "/Products" matches "/products".
	CaseInsensitive
)

// PathPattern represents a compiled URL pattern for route matching.
//
// Patterns support three types of segments:
//   - Static: "/products", "/users/list" - must match exactly
//   - Parameter: ":id", ":name" - captures a single path segment
//   - Wildcard: "*path" - captures all remaining segments (must be last)
//
// Create patterns using [NewPathPattern]:
//
//	pattern := navigation.NewPathPattern("/products/:id")
//	params, ok := pattern.Match("/products/123")
//	// params = {"id": "123"}, ok = true
type PathPattern struct {
	pattern               string
	segments              []segment
	trailingSlash         TrailingSlashBehavior
	caseSensitivity       CaseSensitivity
	hasWildcard           bool
	wildcardParamName     string
	requiresTrailingSlash bool // true if pattern ends with / in strict mode
}

type segment struct {
	value   string
	isParam bool
	isWild  bool // * catches rest of path
}

// PathPatternOption configures a [PathPattern] during creation.
type PathPatternOption func(*PathPattern)

// WithTrailingSlash sets how trailing slashes are handled during matching.
//
//	// Strict mode - trailing slash must match exactly
//	pattern := navigation.NewPathPattern("/products/:id",
//	    navigation.WithTrailingSlash(navigation.TrailingSlashStrict),
//	)
func WithTrailingSlash(behavior TrailingSlashBehavior) PathPatternOption {
	return func(p *PathPattern) {
		p.trailingSlash = behavior
	}
}

// WithCaseSensitivity sets whether matching is case-sensitive.
//
//	// Case-insensitive matching
//	pattern := navigation.NewPathPattern("/Products/:id",
//	    navigation.WithCaseSensitivity(navigation.CaseInsensitive),
//	)
func WithCaseSensitivity(sensitivity CaseSensitivity) PathPatternOption {
	return func(p *PathPattern) {
		p.caseSensitivity = sensitivity
	}
}

// NewPathPattern compiles a pattern string into a PathPattern.
// Pattern syntax:
//   - Static segments: /products, /users
//   - Parameters: :id, :name (captures a single segment)
//   - Wildcards: *path (captures remaining path, must be last)
//
// Examples:
//   - "/products/:id" matches "/products/123" -> {"id": "123"}
//   - "/files/*path" matches "/files/a/b/c" -> {"path": "a/b/c"}
func NewPathPattern(pattern string, opts ...PathPatternOption) *PathPattern {
	p := &PathPattern{
		pattern:         pattern,
		trailingSlash:   TrailingSlashStrip,
		caseSensitivity: CaseSensitive,
	}

	for _, opt := range opts {
		opt(p)
	}

	// Track if pattern requires trailing slash (only meaningful in strict mode)
	hasTrailingSlash := strings.HasSuffix(pattern, "/") && pattern != "/"
	if p.trailingSlash == TrailingSlashStrict && hasTrailingSlash {
		p.requiresTrailingSlash = true
	}

	// Normalize pattern for parsing (but we've already captured trailing slash requirement)
	pattern = strings.TrimSuffix(pattern, "/")
	if pattern == "" {
		pattern = "/"
	}

	// Parse segments
	if pattern == "/" {
		p.segments = nil
		return p
	}

	parts := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	p.segments = make([]segment, 0, len(parts))

	for i, part := range parts {
		if strings.HasPrefix(part, "*") {
			// Wildcard - captures rest of path
			p.hasWildcard = true
			p.wildcardParamName = strings.TrimPrefix(part, "*")
			if p.wildcardParamName == "" {
				p.wildcardParamName = "wildcard"
			}
			p.segments = append(p.segments, segment{
				value:  p.wildcardParamName,
				isWild: true,
			})
			// Wildcard must be last
			if i < len(parts)-1 {
				// Invalid pattern, but we'll just ignore trailing parts
				break
			}
		} else if strings.HasPrefix(part, ":") {
			// Parameter
			p.segments = append(p.segments, segment{
				value:   strings.TrimPrefix(part, ":"),
				isParam: true,
			})
		} else {
			// Static segment
			p.segments = append(p.segments, segment{
				value: part,
			})
		}
	}

	return p
}

// Match checks if a path matches this pattern and extracts parameters.
//
// Returns the extracted parameters and true if the path matches, or nil and
// false if it doesn't match. Percent-encoded values (like %20 for space) are
// automatically decoded in the returned parameters.
//
// Examples:
//
//	pattern := navigation.NewPathPattern("/products/:id")
//
//	params, ok := pattern.Match("/products/123")
//	// params = {"id": "123"}, ok = true
//
//	params, ok := pattern.Match("/products/hello%20world")
//	// params = {"id": "hello world"}, ok = true
//
//	params, ok := pattern.Match("/users/123")
//	// params = nil, ok = false
func (p *PathPattern) Match(path string) (params map[string]string, ok bool) {
	// Strip query string and fragment if present
	if idx := strings.IndexAny(path, "?#"); idx >= 0 {
		path = path[:idx]
	}

	// Check trailing slash requirement in strict mode
	pathHasTrailingSlash := strings.HasSuffix(path, "/") && path != "/"
	if p.trailingSlash == TrailingSlashStrict {
		if p.requiresTrailingSlash != pathHasTrailingSlash {
			return nil, false
		}
	}

	// Handle trailing slash for matching
	if p.trailingSlash == TrailingSlashStrip || pathHasTrailingSlash {
		path = strings.TrimSuffix(path, "/")
		if path == "" {
			path = "/"
		}
	}

	// Handle case sensitivity
	matchPath := path
	if p.caseSensitivity == CaseInsensitive {
		matchPath = strings.ToLower(path)
	}

	// Root path special case
	if len(p.segments) == 0 {
		if matchPath == "/" {
			return map[string]string{}, true
		}
		return nil, false
	}

	// Split path into segments
	pathParts := strings.Split(strings.TrimPrefix(matchPath, "/"), "/")
	originalParts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	params = make(map[string]string)

	segIdx := 0
	for i := 0; i < len(pathParts); i++ {
		if segIdx >= len(p.segments) {
			// More path segments than pattern segments
			return nil, false
		}

		seg := p.segments[segIdx]

		if seg.isWild {
			// Wildcard captures remaining path
			remaining := strings.Join(originalParts[i:], "/")
			decoded, err := url.PathUnescape(remaining)
			if err != nil {
				decoded = remaining
			}
			params[seg.value] = decoded
			return params, true
		}

		if seg.isParam {
			// Parameter captures this segment
			decoded, err := url.PathUnescape(originalParts[i])
			if err != nil {
				decoded = originalParts[i]
			}
			params[seg.value] = decoded
			segIdx++
			continue
		}

		// Static segment must match exactly
		segValue := seg.value
		if p.caseSensitivity == CaseInsensitive {
			segValue = strings.ToLower(segValue)
		}
		if pathParts[i] != segValue {
			return nil, false
		}
		segIdx++
	}

	// Check if we consumed all pattern segments
	if segIdx < len(p.segments) {
		// Didn't match all segments - unless the remaining is a wildcard
		if p.segments[segIdx].isWild {
			params[p.segments[segIdx].value] = ""
			return params, true
		}
		return nil, false
	}

	return params, true
}

// Pattern returns the original pattern string used to create this PathPattern.
func (p *PathPattern) Pattern() string {
	return p.pattern
}

// ParsePath splits a URL into its path and query components.
//
// The path is normalized (trailing slash removed) and query parameters are
// parsed and percent-decoded. URL fragments (#...) are ignored since they
// are not sent to the server in HTTP requests.
//
// Example:
//
//	path, query := navigation.ParsePath("/search?q=hello%20world&page=2#results")
//	// path = "/search"
//	// query = {"q": ["hello world"], "page": ["2"]}
func ParsePath(fullPath string) (path string, query map[string][]string) {
	u, err := url.Parse(fullPath)
	if err != nil {
		return fullPath, nil
	}

	// Normalize: remove trailing slash
	path = strings.TrimSuffix(u.Path, "/")
	if path == "" {
		path = "/"
	}

	// Parse query with proper decoding
	// Fragment (u.Fragment) is intentionally ignored
	query = u.Query()
	return path, query
}

// MatchPath is a convenience function that combines path parsing and
// [PathPattern.Match] to extract complete [RouteSettings] from a URL.
//
// Unlike [ParsePath], this function preserves trailing slashes for matching,
// allowing [TrailingSlashStrict] patterns to work correctly.
//
// Returns RouteSettings with Name, Params, and Query populated if the path
// matches, or empty settings and false if it doesn't match.
//
// Example:
//
//	pattern := navigation.NewPathPattern("/products/:id")
//	settings, ok := navigation.MatchPath(pattern, "/products/123?color=red")
//	// settings.Name = "/products/123?color=red"
//	// settings.Params = {"id": "123"}
//	// settings.Query = {"color": ["red"]}
//	// ok = true
func MatchPath(pattern *PathPattern, fullPath string) (settings RouteSettings, ok bool) {
	// Extract query without normalizing path (preserves trailing slash)
	_, query := ParsePath(fullPath)

	// Get path portion preserving trailing slash for pattern matching
	pathOnly := fullPath
	if idx := strings.IndexAny(fullPath, "?#"); idx >= 0 {
		pathOnly = fullPath[:idx]
	}

	params, ok := pattern.Match(pathOnly)
	if !ok {
		return RouteSettings{}, false
	}

	return RouteSettings{
		Name:   fullPath,
		Params: params,
		Query:  query,
	}, true
}
