package testing

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/widgets"
)

// Finder locates elements in the widget tree.
type Finder interface {
	// Evaluate returns all matching elements under root (depth-first pre-order).
	Evaluate(root core.Element) []core.Element
	// Description returns a human-readable description for error messages.
	Description() string
}

// FinderResult wraps finder results with convenient accessors.
type FinderResult struct {
	elements []core.Element
	finder   Finder
}

// First returns the first match. Panics if no matches.
func (r FinderResult) First() core.Element {
	if len(r.elements) == 0 {
		desc := "unknown"
		if r.finder != nil {
			desc = r.finder.Description()
		}
		panic(fmt.Sprintf("Finder found no elements: %s", desc))
	}
	return r.elements[0]
}

// FirstOrNil returns the first match, or nil if none.
func (r FinderResult) FirstOrNil() core.Element {
	if len(r.elements) == 0 {
		return nil
	}
	return r.elements[0]
}

// At returns the match at index. Panics if out of range.
func (r FinderResult) At(index int) core.Element {
	if index < 0 || index >= len(r.elements) {
		desc := "unknown"
		if r.finder != nil {
			desc = r.finder.Description()
		}
		panic(fmt.Sprintf("Finder index %d out of range (found %d): %s", index, len(r.elements), desc))
	}
	return r.elements[index]
}

// All returns all matches in traversal order.
func (r FinderResult) All() []core.Element {
	return r.elements
}

// Count returns the number of matches.
func (r FinderResult) Count() int {
	return len(r.elements)
}

// Exists returns true if at least one match was found.
func (r FinderResult) Exists() bool {
	return len(r.elements) > 0
}

// Widget returns the widget of the first matched element. Panics if no matches.
func (r FinderResult) Widget() core.Widget {
	return r.First().Widget()
}

// RenderObject returns the render object of the first matched element.
// Returns nil if the element has no associated render object.
func (r FinderResult) RenderObject() layout.RenderObject {
	return extractRenderObject(r.First())
}

// --- Concrete finders ---

// typeFinder matches elements whose widget is of the specified type.
type typeFinder struct {
	widgetType reflect.Type
	typeName   string
}

func (f *typeFinder) Evaluate(root core.Element) []core.Element {
	return collectMatches(root, func(e core.Element) bool {
		return reflect.TypeOf(e.Widget()) == f.widgetType
	})
}

func (f *typeFinder) Description() string {
	return fmt.Sprintf("ByType(%s)", f.typeName)
}

// ByType returns a finder that matches elements whose widget is type T.
func ByType[T core.Widget]() Finder {
	t := reflect.TypeFor[T]()
	return &typeFinder{widgetType: t, typeName: t.String()}
}

// keyFinder matches elements whose widget key equals the given key.
type keyFinder struct {
	key any
}

func (f *keyFinder) Evaluate(root core.Element) []core.Element {
	return collectMatches(root, func(e core.Element) bool {
		k := e.Widget().Key()
		if k == nil && f.key == nil {
			return true
		}
		if k == nil || f.key == nil {
			return false
		}
		// Guard against non-comparable types (slices, maps, funcs).
		if !reflect.TypeOf(k).Comparable() || !reflect.TypeOf(f.key).Comparable() {
			return reflect.DeepEqual(k, f.key)
		}
		return k == f.key
	})
}

func (f *keyFinder) Description() string {
	return fmt.Sprintf("ByKey(%v)", f.key)
}

// ByKey returns a finder that matches elements whose widget key equals key.
func ByKey(key any) Finder {
	return &keyFinder{key: key}
}

// textFinder matches widgets.Text elements by exact content.
type textFinder struct {
	text string
}

func (f *textFinder) Evaluate(root core.Element) []core.Element {
	return collectMatches(root, func(e core.Element) bool {
		if t, ok := e.Widget().(widgets.Text); ok {
			return t.Content == f.text
		}
		if rt, ok := e.Widget().(widgets.RichText); ok {
			return rt.Content.PlainText() == f.text
		}
		return false
	})
}

func (f *textFinder) Description() string {
	return fmt.Sprintf("ByText(%q)", f.text)
}

// ByText returns a finder that matches [widgets.Text] or [widgets.RichText]
// with exact content. For RichText, the match is against the concatenated
// plain text of all spans.
func ByText(text string) Finder {
	return &textFinder{text: text}
}

// textContainingFinder matches widgets.Text elements containing substring.
type textContainingFinder struct {
	substring string
}

func (f *textContainingFinder) Evaluate(root core.Element) []core.Element {
	return collectMatches(root, func(e core.Element) bool {
		if t, ok := e.Widget().(widgets.Text); ok {
			return strings.Contains(t.Content, f.substring)
		}
		if rt, ok := e.Widget().(widgets.RichText); ok {
			return strings.Contains(rt.Content.PlainText(), f.substring)
		}
		return false
	})
}

func (f *textContainingFinder) Description() string {
	return fmt.Sprintf("ByTextContaining(%q)", f.substring)
}

// ByTextContaining returns a finder that matches [widgets.Text] or
// [widgets.RichText] containing the given substring. For RichText, the match
// is against the concatenated plain text of all spans.
func ByTextContaining(substring string) Finder {
	return &textContainingFinder{substring: substring}
}

// predicateFinder matches elements satisfying a predicate.
type predicateFinder struct {
	fn   func(core.Element) bool
	desc string
}

func (f *predicateFinder) Evaluate(root core.Element) []core.Element {
	return collectMatches(root, f.fn)
}

func (f *predicateFinder) Description() string {
	return f.desc
}

// ByPredicate returns a finder that matches elements satisfying fn.
func ByPredicate(fn func(core.Element) bool) Finder {
	return &predicateFinder{fn: fn, desc: "ByPredicate(...)"}
}

// descendantFinder finds elements matching 'matching' that are descendants
// of elements matching 'of'.
type descendantFinder struct {
	of       Finder
	matching Finder
}

func (f *descendantFinder) Evaluate(root core.Element) []core.Element {
	ancestors := f.of.Evaluate(root)
	if len(ancestors) == 0 {
		return nil
	}
	var results []core.Element
	seen := make(map[core.Element]bool)
	for _, ancestor := range ancestors {
		// Search within each ancestor's subtree (skip the ancestor itself)
		ancestor.VisitChildren(func(child core.Element) bool {
			for _, match := range f.matching.Evaluate(child) {
				if !seen[match] {
					seen[match] = true
					results = append(results, match)
				}
			}
			return true
		})
	}
	return results
}

func (f *descendantFinder) Description() string {
	return fmt.Sprintf("Descendant(of: %s, matching: %s)", f.of.Description(), f.matching.Description())
}

// Descendant returns a finder that matches elements satisfying 'matching'
// that are descendants of elements matching 'of'.
func Descendant(of, matching Finder) Finder {
	return &descendantFinder{of: of, matching: matching}
}

// ancestorFinder finds elements matching 'matching' that are ancestors
// of elements matching 'of'.
type ancestorFinder struct {
	of       Finder
	matching Finder
}

func (f *ancestorFinder) Evaluate(root core.Element) []core.Element {
	descendants := f.of.Evaluate(root)
	if len(descendants) == 0 {
		return nil
	}
	// Collect all ancestors of matched elements, then filter by matching finder
	allElements := f.matching.Evaluate(root)
	if len(allElements) == 0 {
		return nil
	}
	ancestorSet := make(map[core.Element]bool)
	for _, e := range allElements {
		ancestorSet[e] = true
	}
	// For each descendant, walk up looking for matching ancestors
	var results []core.Element
	seen := make(map[core.Element]bool)
	for _, desc := range descendants {
		// Walk all elements and check if desc is a descendant
		for _, candidate := range allElements {
			if !seen[candidate] && isAncestorOf(candidate, desc) {
				seen[candidate] = true
				results = append(results, candidate)
			}
		}
	}
	return results
}

func (f *ancestorFinder) Description() string {
	return fmt.Sprintf("Ancestor(of: %s, matching: %s)", f.of.Description(), f.matching.Description())
}

// Ancestor returns a finder that matches elements satisfying 'matching'
// that are ancestors of elements matching 'of'.
func Ancestor(of, matching Finder) Finder {
	return &ancestorFinder{of: of, matching: matching}
}

// isAncestorOf returns true if ancestor contains descendant in its subtree.
func isAncestorOf(ancestor, descendant core.Element) bool {
	found := false
	walkTree(ancestor, func(e core.Element) bool {
		if e == descendant {
			found = true
			return false // stop
		}
		return true
	})
	return found
}

// collectMatches performs depth-first pre-order traversal, collecting
// elements that satisfy the predicate.
func collectMatches(root core.Element, predicate func(core.Element) bool) []core.Element {
	var results []core.Element
	walkTree(root, func(e core.Element) bool {
		if predicate(e) {
			results = append(results, e)
		}
		return true
	})
	return results
}

// walkTree performs a depth-first pre-order traversal of the element tree.
// The visitor returns false to stop traversal.
func walkTree(root core.Element, visitor func(core.Element) bool) {
	if !visitor(root) {
		return
	}
	root.VisitChildren(func(child core.Element) bool {
		walkTree(child, visitor)
		return true
	})
}
