package testing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// TestingT is the subset of *testing.T used by MatchesFile, allowing
// test doubles to intercept failures.
type TestingT interface {
	Helper()
	Fatalf(format string, args ...any)
	Errorf(format string, args ...any)
	Name() string
}

// Snapshot captures the render tree structure and display operations.
type Snapshot struct {
	RenderTree *RenderNode `json:"renderTree"`
	DisplayOps []DisplayOp `json:"displayOps,omitempty"`
}

// RenderNode represents a node in the serialized render tree.
type RenderNode struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Size       [2]float64     `json:"size"`
	Offset     [2]float64     `json:"offset"`
	Properties map[string]any `json:"props,omitempty"`
	Children   []*RenderNode  `json:"children,omitempty"`
}

// propertyWhitelist defines which properties to serialize per render type.
// Types not listed here are serialized with size/offset only.
var propertyWhitelist = map[string][]string{
	"RenderFlex":           {"direction", "alignment", "crossAlignment"},
	"RenderPadding":        {"padding"},
	"RenderContainer":      {"painter.color", "padding", "width", "height"},
	"RenderText":           {"text", "maxLines"},
	"RenderRichText":       {"text", "maxLines", "baseStyle"},
	"RenderConstrainedBox": {"minWidth", "maxWidth", "minHeight", "maxHeight"},
	"RenderSizedBox":       {"width", "height"},
	"RenderOpacity":        {"opacity"},
	"RenderClipRRect":      {"radius"},
}

// CaptureSnapshot captures the current render tree and display operations.
func (t *WidgetTester) CaptureSnapshot() *Snapshot {
	snap := &Snapshot{}
	if t.rootRender != nil {
		counter := &typeCounter{}
		snap.RenderTree = captureRenderNode(t.rootRender, counter)

		// Record paint operations via serializing canvas
		recorder := &graphics.PictureRecorder{}
		canvas := recorder.BeginRecording(t.size)
		ctx := &layout.PaintContext{Canvas: canvas}
		t.rootRender.Paint(ctx)
		dl := recorder.EndRecording()
		snap.DisplayOps = serializeDisplayList(dl)
	}
	return snap
}

// MatchesFile compares this snapshot against a golden file. On mismatch it
// reports a diff and instructions for updating. When DRIFT_UPDATE_SNAPSHOTS=1
// is set, the file is silently updated instead.
func (s *Snapshot) MatchesFile(t TestingT, path string) {
	t.Helper()

	if os.Getenv("DRIFT_UPDATE_SNAPSHOTS") == "1" {
		if err := s.UpdateFile(path); err != nil {
			t.Fatalf("failed to update snapshot: %v", err)
		}
		return
	}

	expected, err := loadSnapshot(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("snapshot file missing: %s\n\nTo create: DRIFT_UPDATE_SNAPSHOTS=1 go test -run %s", path, t.Name())
			return
		}
		t.Fatalf("failed to load snapshot: %v", err)
		return
	}

	if diff := s.Diff(expected); diff != "" {
		t.Errorf("snapshot mismatch: %s\n%s\n\nTo update: DRIFT_UPDATE_SNAPSHOTS=1 go test -run %s", path, diff, t.Name())
	}
}

// UpdateFile writes this snapshot to the given path, creating directories
// as needed.
func (s *Snapshot) UpdateFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := marshalSnapshot(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Diff returns a unified diff between this snapshot and other. Returns
// empty string if equal.
func (s *Snapshot) Diff(other *Snapshot) string {
	a, _ := marshalSnapshot(s)
	b, _ := marshalSnapshot(other)
	if bytes.Equal(a, b) {
		return ""
	}
	return unifiedDiff(string(b), string(a))
}

// --- Internal ---

// typeCounter assigns stable IDs like "RenderFlex#0", "RenderFlex#1".
type typeCounter struct {
	counts map[string]int
}

func (c *typeCounter) next(typeName string) string {
	if c.counts == nil {
		c.counts = make(map[string]int)
	}
	n := c.counts[typeName]
	c.counts[typeName] = n + 1
	return fmt.Sprintf("%s#%d", typeName, n)
}

func captureRenderNode(ro layout.RenderObject, counter *typeCounter) *RenderNode {
	typeName := renderTypeName(ro)
	size := ro.Size()

	// Get offset from parent data
	offset := graphics.Offset{}
	if pd, ok := ro.ParentData().(*layout.BoxParentData); ok {
		offset = pd.Offset
	}

	node := &RenderNode{
		ID:     counter.next(typeName),
		Type:   typeName,
		Size:   [2]float64{round2(size.Width), round2(size.Height)},
		Offset: [2]float64{round2(offset.X), round2(offset.Y)},
	}

	// Capture whitelisted properties
	if props := captureProperties(ro, typeName); len(props) > 0 {
		node.Properties = props
	}

	// Recurse into children
	if visitor, ok := ro.(layout.ChildVisitor); ok {
		visitor.VisitChildren(func(child layout.RenderObject) {
			childNode := captureRenderNode(child, counter)
			node.Children = append(node.Children, childNode)
		})
	}

	return node
}

func renderTypeName(ro layout.RenderObject) string {
	t := reflect.TypeOf(ro)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	name := t.Name()
	// Capitalize first letter so unexported types like renderFlex
	// match whitelist entries like RenderFlex.
	if len(name) > 0 {
		name = strings.ToUpper(name[:1]) + name[1:]
	}
	return name
}

// getFieldByPath navigates a dot-separated path like "painter.color" through
// nested structs. Returns an invalid Value if the path doesn't exist.
func getFieldByPath(v reflect.Value, path string) reflect.Value {
	parts := strings.Split(path, ".")
	for _, part := range parts {
		if part == "" {
			return reflect.Value{}
		}
		if v.Kind() == reflect.Ptr {
			if v.IsNil() {
				return reflect.Value{}
			}
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return reflect.Value{}
		}
		field := v.FieldByName(part)
		if !field.IsValid() {
			// Try exported version (capitalize first letter)
			exported := strings.ToUpper(part[:1]) + part[1:]
			field = v.FieldByName(exported)
		}
		if !field.IsValid() {
			return reflect.Value{}
		}
		v = field
	}
	return v
}

func captureProperties(ro layout.RenderObject, typeName string) map[string]any {
	whitelist, ok := propertyWhitelist[typeName]
	if !ok {
		return nil
	}

	props := make(map[string]any)
	v := reflect.ValueOf(ro)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	for _, fieldPath := range whitelist {
		field := getFieldByPath(v, fieldPath)
		if !field.IsValid() {
			continue
		}
		// Use the last component of the path as the property name
		propName := fieldPath
		if idx := strings.LastIndex(fieldPath, "."); idx >= 0 {
			propName = fieldPath[idx+1:]
		}
		if val := serializeFieldValue(field); val != nil {
			props[propName] = val
		}
	}

	if len(props) == 0 {
		return nil
	}
	return props
}

func serializeFieldValue(v reflect.Value) any {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.Type() == reflect.TypeOf(graphics.Color(0)) {
			return serializeColor(graphics.Color(v.Uint()))
		}
		return v.Uint()
	case reflect.Float32, reflect.Float64:
		return round2(v.Float())
	case reflect.String:
		return v.String()
	case reflect.Bool:
		return v.Bool()
	case reflect.Struct:
		if !v.CanInterface() {
			// Unexported struct field — serialize exported fields individually.
			return serializeStruct(v)
		}
		return fmt.Sprintf("%v", v.Interface())
	default:
		return nil
	}
}

// serializeStruct handles unexported struct fields by iterating exported
// sub-fields and collecting their values into a map.
func serializeStruct(v reflect.Value) any {
	t := v.Type()
	m := make(map[string]any)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		if val := serializeFieldValue(v.Field(i)); val != nil {
			m[f.Name] = val
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

func loadSnapshot(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("invalid snapshot JSON: %w", err)
	}
	return &snap, nil
}

func marshalSnapshot(s *Snapshot) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// unifiedDiff produces a simple line-oriented diff.
func unifiedDiff(expected, actual string) string {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	var buf strings.Builder
	buf.WriteString("--- expected\n+++ actual\n")

	maxLen := len(expectedLines)
	if len(actualLines) > maxLen {
		maxLen = len(actualLines)
	}

	for i := 0; i < maxLen; i++ {
		var e, a string
		if i < len(expectedLines) {
			e = expectedLines[i]
		}
		if i < len(actualLines) {
			a = actualLines[i]
		}
		if e != a {
			if i < len(expectedLines) {
				fmt.Fprintf(&buf, "-%s\n", e)
			}
			if i < len(actualLines) {
				fmt.Fprintf(&buf, "+%s\n", a)
			}
		}
	}

	return buf.String()
}
