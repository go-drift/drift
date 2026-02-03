package navigation

import (
	"reflect"
	"testing"
)

func TestNewPathPattern_Static(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"/", "/", true},
		{"/", "/foo", false},
		{"/products", "/products", true},
		{"/products", "/products/", true}, // trailing slash stripped by default
		{"/products", "/Products", false}, // case sensitive by default
		{"/products/list", "/products/list", true},
		{"/products/list", "/products", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			p := NewPathPattern(tt.pattern)
			_, ok := p.Match(tt.path)
			if ok != tt.want {
				t.Errorf("NewPathPattern(%q).Match(%q) = %v, want %v", tt.pattern, tt.path, ok, tt.want)
			}
		})
	}
}

func TestNewPathPattern_Params(t *testing.T) {
	tests := []struct {
		pattern    string
		path       string
		wantOK     bool
		wantParams map[string]string
	}{
		{
			pattern:    "/products/:id",
			path:       "/products/123",
			wantOK:     true,
			wantParams: map[string]string{"id": "123"},
		},
		{
			pattern:    "/products/:id",
			path:       "/products/",
			wantOK:     false,
			wantParams: nil,
		},
		{
			pattern:    "/users/:userId/posts/:postId",
			path:       "/users/42/posts/99",
			wantOK:     true,
			wantParams: map[string]string{"userId": "42", "postId": "99"},
		},
		{
			pattern:    "/products/:id",
			path:       "/products/hello%20world",
			wantOK:     true,
			wantParams: map[string]string{"id": "hello world"}, // percent decoded
		},
		{
			pattern:    "/products/:id",
			path:       "/users/123",
			wantOK:     false,
			wantParams: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			p := NewPathPattern(tt.pattern)
			params, ok := p.Match(tt.path)
			if ok != tt.wantOK {
				t.Errorf("Match() ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK && !reflect.DeepEqual(params, tt.wantParams) {
				t.Errorf("Match() params = %v, want %v", params, tt.wantParams)
			}
		})
	}
}

func TestNewPathPattern_Wildcard(t *testing.T) {
	tests := []struct {
		pattern    string
		path       string
		wantOK     bool
		wantParams map[string]string
	}{
		{
			pattern:    "/files/*path",
			path:       "/files/docs/readme.md",
			wantOK:     true,
			wantParams: map[string]string{"path": "docs/readme.md"},
		},
		{
			pattern:    "/files/*path",
			path:       "/files/",
			wantOK:     true,
			wantParams: map[string]string{"path": ""},
		},
		{
			pattern:    "/files/*path",
			path:       "/files",
			wantOK:     true,
			wantParams: map[string]string{"path": ""},
		},
		{
			pattern:    "/api/*",
			path:       "/api/v1/users",
			wantOK:     true,
			wantParams: map[string]string{"wildcard": "v1/users"},
		},
		{
			pattern:    "/files/*path",
			path:       "/other/docs",
			wantOK:     false,
			wantParams: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			p := NewPathPattern(tt.pattern)
			params, ok := p.Match(tt.path)
			if ok != tt.wantOK {
				t.Errorf("Match() ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK && !reflect.DeepEqual(params, tt.wantParams) {
				t.Errorf("Match() params = %v, want %v", params, tt.wantParams)
			}
		})
	}
}

func TestNewPathPattern_TrailingSlash(t *testing.T) {
	// Default: strip trailing slash
	p := NewPathPattern("/products/:id")
	_, ok := p.Match("/products/123/")
	if !ok {
		t.Error("TrailingSlashStrip should match paths with trailing slash")
	}

	// Strict: require exact match
	p = NewPathPattern("/products/:id", WithTrailingSlash(TrailingSlashStrict))
	_, ok = p.Match("/products/123/")
	if ok {
		t.Error("TrailingSlashStrict should not match paths with trailing slash")
	}
	_, ok = p.Match("/products/123")
	if !ok {
		t.Error("TrailingSlashStrict should match paths without trailing slash")
	}
}

func TestNewPathPattern_TrailingSlash_RequiresTrailingSlash(t *testing.T) {
	// Pattern that requires trailing slash
	p := NewPathPattern("/products/:id/", WithTrailingSlash(TrailingSlashStrict))

	// Should match paths WITH trailing slash
	_, ok := p.Match("/products/123/")
	if !ok {
		t.Error("Pattern /products/:id/ should match /products/123/ with trailing slash")
	}

	// Should NOT match paths WITHOUT trailing slash
	_, ok = p.Match("/products/123")
	if ok {
		t.Error("Pattern /products/:id/ should NOT match /products/123 without trailing slash")
	}
}

func TestNewPathPattern_TrailingSlash_StripMode_IgnoresPatternTrailingSlash(t *testing.T) {
	// In strip mode, pattern's trailing slash doesn't matter
	p := NewPathPattern("/products/:id/", WithTrailingSlash(TrailingSlashStrip))

	// Both with and without trailing slash should match
	_, ok := p.Match("/products/123/")
	if !ok {
		t.Error("Strip mode should match with trailing slash")
	}

	_, ok = p.Match("/products/123")
	if !ok {
		t.Error("Strip mode should match without trailing slash")
	}
}

func TestNewPathPattern_CaseSensitivity(t *testing.T) {
	// Default: case sensitive
	p := NewPathPattern("/Products/:id")
	_, ok := p.Match("/products/123")
	if ok {
		t.Error("CaseSensitive should not match different case")
	}
	_, ok = p.Match("/Products/123")
	if !ok {
		t.Error("CaseSensitive should match same case")
	}

	// Case insensitive
	p = NewPathPattern("/Products/:id", WithCaseSensitivity(CaseInsensitive))
	_, ok = p.Match("/products/123")
	if !ok {
		t.Error("CaseInsensitive should match different case")
	}
	_, ok = p.Match("/PRODUCTS/123")
	if !ok {
		t.Error("CaseInsensitive should match uppercase")
	}
}

func TestNewPathPattern_QueryAndFragment(t *testing.T) {
	p := NewPathPattern("/products/:id")

	// Query string should be stripped for matching
	params, ok := p.Match("/products/123?color=red")
	if !ok {
		t.Error("Should match path with query string")
	}
	if params["id"] != "123" {
		t.Errorf("Should extract param without query, got %q", params["id"])
	}

	// Fragment should be stripped for matching
	params, ok = p.Match("/products/456#section")
	if !ok {
		t.Error("Should match path with fragment")
	}
	if params["id"] != "456" {
		t.Errorf("Should extract param without fragment, got %q", params["id"])
	}
}

func TestPathPattern_Pattern(t *testing.T) {
	p := NewPathPattern("/products/:id")
	if p.Pattern() != "/products/:id" {
		t.Errorf("Pattern() = %q, want %q", p.Pattern(), "/products/:id")
	}
}

func TestParsePath(t *testing.T) {
	tests := []struct {
		fullPath  string
		wantPath  string
		wantQuery map[string][]string
	}{
		{
			fullPath:  "/search",
			wantPath:  "/search",
			wantQuery: map[string][]string{},
		},
		{
			fullPath:  "/search?q=hello",
			wantPath:  "/search",
			wantQuery: map[string][]string{"q": {"hello"}},
		},
		{
			fullPath:  "/search?q=hello&page=2",
			wantPath:  "/search",
			wantQuery: map[string][]string{"q": {"hello"}, "page": {"2"}},
		},
		{
			fullPath:  "/search?tag=a&tag=b",
			wantPath:  "/search",
			wantQuery: map[string][]string{"tag": {"a", "b"}},
		},
		{
			fullPath:  "/search?q=hello%20world",
			wantPath:  "/search",
			wantQuery: map[string][]string{"q": {"hello world"}},
		},
		{
			fullPath:  "/search#results",
			wantPath:  "/search",
			wantQuery: map[string][]string{},
		},
		{
			fullPath:  "/search?q=test#results",
			wantPath:  "/search",
			wantQuery: map[string][]string{"q": {"test"}},
		},
		{
			fullPath:  "/path/",
			wantPath:  "/path",
			wantQuery: map[string][]string{},
		},
		{
			fullPath:  "/",
			wantPath:  "/",
			wantQuery: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.fullPath, func(t *testing.T) {
			path, query := ParsePath(tt.fullPath)
			if path != tt.wantPath {
				t.Errorf("ParsePath(%q) path = %q, want %q", tt.fullPath, path, tt.wantPath)
			}
			if !reflect.DeepEqual(query, tt.wantQuery) {
				t.Errorf("ParsePath(%q) query = %v, want %v", tt.fullPath, query, tt.wantQuery)
			}
		})
	}
}

func TestMatchPath(t *testing.T) {
	pattern := NewPathPattern("/products/:id")

	settings, ok := MatchPath(pattern, "/products/123?color=red")
	if !ok {
		t.Fatal("MatchPath should match")
	}
	if settings.Name != "/products/123?color=red" {
		t.Errorf("Name = %q, want %q", settings.Name, "/products/123?color=red")
	}
	if settings.Params["id"] != "123" {
		t.Errorf("Params[id] = %q, want %q", settings.Params["id"], "123")
	}
	if settings.Query["color"][0] != "red" {
		t.Errorf("Query[color] = %v, want [red]", settings.Query["color"])
	}
}

func TestRouteSettings_Param(t *testing.T) {
	s := RouteSettings{
		Params: map[string]string{"id": "123", "name": "test"},
	}

	if s.Param("id") != "123" {
		t.Errorf("Param(id) = %q, want %q", s.Param("id"), "123")
	}
	if s.Param("name") != "test" {
		t.Errorf("Param(name) = %q, want %q", s.Param("name"), "test")
	}
	if s.Param("missing") != "" {
		t.Errorf("Param(missing) = %q, want empty string", s.Param("missing"))
	}

	// Nil params map
	s2 := RouteSettings{}
	if s2.Param("id") != "" {
		t.Errorf("Param on nil map = %q, want empty string", s2.Param("id"))
	}
}

func TestRouteSettings_QueryValue(t *testing.T) {
	s := RouteSettings{
		Query: map[string][]string{
			"q":    {"hello"},
			"tags": {"a", "b", "c"},
		},
	}

	if s.QueryValue("q") != "hello" {
		t.Errorf("QueryValue(q) = %q, want %q", s.QueryValue("q"), "hello")
	}
	if s.QueryValue("tags") != "a" {
		t.Errorf("QueryValue(tags) = %q, want %q", s.QueryValue("tags"), "a")
	}
	if s.QueryValue("missing") != "" {
		t.Errorf("QueryValue(missing) = %q, want empty string", s.QueryValue("missing"))
	}

	// Nil query map
	s2 := RouteSettings{}
	if s2.QueryValue("q") != "" {
		t.Errorf("QueryValue on nil map = %q, want empty string", s2.QueryValue("q"))
	}
}

func TestRouteSettings_QueryValues(t *testing.T) {
	s := RouteSettings{
		Query: map[string][]string{
			"tags": {"a", "b", "c"},
		},
	}

	vals := s.QueryValues("tags")
	if !reflect.DeepEqual(vals, []string{"a", "b", "c"}) {
		t.Errorf("QueryValues(tags) = %v, want [a b c]", vals)
	}
	if s.QueryValues("missing") != nil {
		t.Errorf("QueryValues(missing) = %v, want nil", s.QueryValues("missing"))
	}

	// Nil query map
	s2 := RouteSettings{}
	if s2.QueryValues("tags") != nil {
		t.Errorf("QueryValues on nil map = %v, want nil", s2.QueryValues("tags"))
	}
}

func TestMatchPath_PreservesTrailingSlash(t *testing.T) {
	// Pattern requires trailing slash
	patternWithSlash := NewPathPattern("/products/:id/", WithTrailingSlash(TrailingSlashStrict))

	// Should match path WITH trailing slash
	settings, ok := MatchPath(patternWithSlash, "/products/123/")
	if !ok {
		t.Error("MatchPath should match /products/123/ with trailing slash pattern")
	}
	if settings.Params["id"] != "123" {
		t.Errorf("Params[id] = %q, want 123", settings.Params["id"])
	}

	// Should NOT match path WITHOUT trailing slash
	_, ok = MatchPath(patternWithSlash, "/products/123")
	if ok {
		t.Error("MatchPath should NOT match /products/123 when pattern requires trailing slash")
	}

	// Pattern without trailing slash
	patternWithoutSlash := NewPathPattern("/products/:id", WithTrailingSlash(TrailingSlashStrict))

	// Should match path WITHOUT trailing slash
	settings, ok = MatchPath(patternWithoutSlash, "/products/456")
	if !ok {
		t.Error("MatchPath should match /products/456 without trailing slash")
	}
	if settings.Params["id"] != "456" {
		t.Errorf("Params[id] = %q, want 456", settings.Params["id"])
	}

	// Should NOT match path WITH trailing slash
	_, ok = MatchPath(patternWithoutSlash, "/products/456/")
	if ok {
		t.Error("MatchPath should NOT match /products/456/ when pattern doesn't have trailing slash")
	}
}

func TestMatchPath_WithQueryAndTrailingSlash(t *testing.T) {
	pattern := NewPathPattern("/products/:id/", WithTrailingSlash(TrailingSlashStrict))

	// Should match with query string
	settings, ok := MatchPath(pattern, "/products/123/?color=red")
	if !ok {
		t.Error("MatchPath should match /products/123/?color=red")
	}
	if settings.Params["id"] != "123" {
		t.Errorf("Params[id] = %q, want 123", settings.Params["id"])
	}
	if settings.QueryValue("color") != "red" {
		t.Errorf("QueryValue(color) = %q, want red", settings.QueryValue("color"))
	}
}
