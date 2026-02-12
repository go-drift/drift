package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
)

// debugServer manages the HTTP server for render tree inspection.
type debugServer struct {
	server   *http.Server
	listener net.Listener
	mu       sync.Mutex
}

var debugSrv debugServer

// RenderTreeNode represents a node in the serialized render tree.
// Uses SafeFloat for dimensions that may contain Inf/NaN from layout issues.
type RenderTreeNode struct {
	Type              string           `json:"type"`
	Size              SafeSize         `json:"size"`
	Constraints       *SafeConstraints `json:"constraints,omitempty"`
	Offset            SafeOffset       `json:"offset,omitempty"`
	Depth             int              `json:"depth"`
	NeedsLayout       bool             `json:"needsLayout"`
	NeedsPaint        bool             `json:"needsPaint"`
	IsRepaintBoundary bool             `json:"isRepaintBoundary"`
	Children          []RenderTreeNode `json:"children,omitempty"`
}

// SafeFloat wraps a float64 to handle Inf/NaN in JSON encoding.
type SafeFloat float64

func (f SafeFloat) MarshalJSON() ([]byte, error) {
	v := float64(f)
	if math.IsInf(v, 1) {
		return []byte(`"Infinity"`), nil
	}
	if math.IsInf(v, -1) {
		return []byte(`"-Infinity"`), nil
	}
	if math.IsNaN(v) {
		return []byte(`"NaN"`), nil
	}
	return json.Marshal(v)
}

// SafeSize is a JSON-safe version of graphics.Size.
type SafeSize struct {
	Width  SafeFloat `json:"width"`
	Height SafeFloat `json:"height"`
}

// SafeOffset is a JSON-safe version of graphics.Offset.
type SafeOffset struct {
	X SafeFloat `json:"x"`
	Y SafeFloat `json:"y"`
}

// SafeConstraints is a JSON-safe version of layout.Constraints.
type SafeConstraints struct {
	MinWidth  SafeFloat `json:"minWidth"`
	MaxWidth  SafeFloat `json:"maxWidth"`
	MinHeight SafeFloat `json:"minHeight"`
	MaxHeight SafeFloat `json:"maxHeight"`
}

// WidgetTreeNode represents a node in the serialized widget/element tree.
type WidgetTreeNode struct {
	WidgetType  string           `json:"widgetType"`
	ElementType string           `json:"elementType"`
	Key         any              `json:"key,omitempty"`
	Depth       int              `json:"depth"`
	NeedsBuild  bool             `json:"needsBuild"`
	HasState    bool             `json:"hasState,omitempty"`
	Children    []WidgetTreeNode `json:"children,omitempty"`
}

// startDebugServer starts the HTTP debug server on the specified port.
// Returns the actual port (useful when port=0 for ephemeral allocation).
func startDebugServer(port int) (int, error) {
	debugSrv.mu.Lock()
	defer debugSrv.mu.Unlock()

	if debugSrv.server != nil {
		// Already running - return current port
		if debugSrv.listener != nil {
			return debugSrv.listener.Addr().(*net.TCPAddr).Port, nil
		}
		return port, nil
	}

	// Bind listener first to fail fast on port conflicts
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return 0, fmt.Errorf("debug server listen: %w", err)
	}

	actualPort := listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/render-tree", handleRenderTree)
	mux.HandleFunc("/widget-tree", handleWidgetTree)
	mux.HandleFunc("/frames", handleFrameTimeline)
	mux.HandleFunc("/runtime", handleRuntime)
	mux.HandleFunc("/jank", handleJankSnapshot)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/debug", handleDebug)

	server := &http.Server{Handler: mux}
	debugSrv.server = server
	debugSrv.listener = listener

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			// Server failed - clear state so it can be restarted
			debugSrv.mu.Lock()
			debugSrv.server = nil
			debugSrv.listener = nil
			debugSrv.mu.Unlock()
			fmt.Printf("debug server error: %v\n", err)
		}
	}()

	return actualPort, nil
}

// stopDebugServer gracefully shuts down the debug server.
func stopDebugServer() {
	debugSrv.mu.Lock()
	server := debugSrv.server
	debugSrv.server = nil
	debugSrv.listener = nil
	debugSrv.mu.Unlock()

	if server == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

// maxTreeDepth limits recursion depth to prevent stack overflow from malformed trees.
const maxTreeDepth = 500

// handleRenderTree returns the render tree as JSON.
//
// Locking contract: frameLock is held briefly to read the rootRender pointer.
// Tree serialization runs after releasing frameLock — the render tree data is
// safe to read because the HTTP handler runs between frames (no concurrent
// mutation). Frame-trace and runtime-sample buffers use their own sync.RWMutex
// and never require frameLock for snapshot reads.
func handleRenderTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Recover from panics during serialization
	defer func() {
		if rec := recover(); rec != nil {
			http.Error(w, fmt.Sprintf("panic: %v", rec), http.StatusInternalServerError)
		}
	}()

	frameLock.Lock()
	root := app.rootRender
	if root == nil {
		frameLock.Unlock()
		http.Error(w, "no render tree", http.StatusServiceUnavailable)
		return
	}
	tree := serializeRenderTreeWithDepth(root, 0)
	frameLock.Unlock()

	// Encode to buffer first so we can catch errors
	data, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		http.Error(w, fmt.Sprintf("json encode error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// handleHealth returns a simple health check response.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// handleDebug returns diagnostic info about the render tree state.
func handleDebug(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	frameLock.Lock()
	root := app.rootRender
	var info struct {
		HasRoot  bool   `json:"hasRoot"`
		RootType string `json:"rootType,omitempty"`
		RootSize string `json:"rootSize,omitempty"`
	}
	info.HasRoot = root != nil
	if root != nil {
		info.RootType = reflect.TypeOf(root).String()
		size := root.Size()
		info.RootSize = fmt.Sprintf("%.2fx%.2f", size.Width, size.Height)
	}
	frameLock.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// handleWidgetTree returns the widget/element tree as JSON.
func handleWidgetTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Recover from panics during serialization
	defer func() {
		if rec := recover(); rec != nil {
			http.Error(w, fmt.Sprintf("panic: %v", rec), http.StatusInternalServerError)
		}
	}()

	frameLock.Lock()
	root := app.root
	if root == nil {
		frameLock.Unlock()
		http.Error(w, "no widget tree", http.StatusServiceUnavailable)
		return
	}
	tree := serializeWidgetTree(root, 0)
	frameLock.Unlock()

	// Encode to buffer first so we can catch errors
	data, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		http.Error(w, fmt.Sprintf("json encode error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// handleFrameTimeline returns recent frame timing samples as JSON.
func handleFrameTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	frameLock.Lock()
	trace := app.frameTrace
	frameLock.Unlock()
	if trace == nil {
		http.Error(w, "frame tracing disabled", http.StatusServiceUnavailable)
		return
	}

	resp := trace.Snapshot()

	applyFrameFilters(r, &resp)

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		http.Error(w, fmt.Sprintf("json encode error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// handleRuntime returns recent runtime/GC samples as JSON.
func handleRuntime(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	frameLock.Lock()
	buffer := app.runtimeSamples
	frameLock.Unlock()
	if buffer == nil {
		http.Error(w, "runtime sampling disabled", http.StatusServiceUnavailable)
		return
	}

	samples := applyRuntimeFilters(r, buffer.Snapshot())

	resp := struct {
		Samples []RuntimeSample `json:"samples"`
	}{
		Samples: samples,
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		http.Error(w, fmt.Sprintf("json encode error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func applyFrameFilters(r *http.Request, resp *FrameTimeline) {
	limit := 0
	if value := r.URL.Query().Get("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	var filters []func(FrameSample) bool

	if v := parseFloatQuery(r, "min_ms"); v > 0 {
		filters = append(filters, func(s FrameSample) bool { return s.FrameMs >= v })
	}
	if v := parseFloatQuery(r, "dispatch_ms"); v > 0 {
		filters = append(filters, func(s FrameSample) bool { return s.Phases.DispatchMs >= v })
	}
	if v := parseFloatQuery(r, "animate_ms"); v > 0 {
		filters = append(filters, func(s FrameSample) bool { return s.Phases.AnimateMs >= v })
	}
	if v := parseFloatQuery(r, "build_ms"); v > 0 {
		filters = append(filters, func(s FrameSample) bool { return s.Phases.BuildMs >= v })
	}
	if v := parseFloatQuery(r, "layout_ms"); v > 0 {
		filters = append(filters, func(s FrameSample) bool { return s.Phases.LayoutMs >= v })
	}
	if v := parseFloatQuery(r, "record_ms"); v > 0 {
		filters = append(filters, func(s FrameSample) bool { return s.Phases.RecordMs >= v })
	}
	if v := parseFloatQuery(r, "geometry_ms"); v > 0 {
		filters = append(filters, func(s FrameSample) bool { return s.Phases.GeometryMs >= v })
	}
	if v := parseFloatQuery(r, "semantics_ms"); v > 0 {
		filters = append(filters, func(s FrameSample) bool { return s.Phases.SemanticsMs >= v })
	}
	if v := parseFloatQuery(r, "trace_overhead_ms"); v > 0 {
		filters = append(filters, func(s FrameSample) bool { return s.Phases.TraceOverheadMs >= v })
	}
	if value := r.URL.Query().Get("resumed"); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil && parsed {
			filters = append(filters, func(s FrameSample) bool { return s.Flags.ResumedThisFrame })
		}
	}

	if len(filters) > 0 {
		filtered := make([]FrameSample, 0, len(resp.Samples))
	outer:
		for _, sample := range resp.Samples {
			for _, f := range filters {
				if !f(sample) {
					continue outer
				}
			}
			filtered = append(filtered, sample)
		}
		resp.Samples = filtered
	}

	if limit > 0 && len(resp.Samples) > limit {
		resp.Samples = resp.Samples[len(resp.Samples)-limit:]
	}
}

func applyRuntimeFilters(r *http.Request, samples []RuntimeSample) []RuntimeSample {
	windowSeconds := parseFloatQuery(r, "window")
	if windowSeconds > 0 {
		cutoff := time.Now().Add(-time.Duration(windowSeconds * float64(time.Second))).UnixMilli()
		filtered := make([]RuntimeSample, 0, len(samples))
		for _, sample := range samples {
			if sample.Timestamp >= cutoff {
				filtered = append(filtered, sample)
			}
		}
		samples = filtered
	}

	limit := 0
	if value := r.URL.Query().Get("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 0 && len(samples) > limit {
		samples = samples[len(samples)-limit:]
	}
	return samples
}

// handleJankSnapshot returns a combined frames/runtime snapshot.
func handleJankSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	frameLock.Lock()
	trace := app.frameTrace
	runtimeBuffer := app.runtimeSamples
	frameLock.Unlock()

	if trace == nil {
		http.Error(w, "frame tracing disabled", http.StatusServiceUnavailable)
		return
	}
	if runtimeBuffer == nil {
		http.Error(w, "runtime sampling disabled", http.StatusServiceUnavailable)
		return
	}

	frames := trace.Snapshot()
	applyFrameFilters(r, &frames)

	runtimeSamples := runtimeBuffer.Snapshot()
	runtimeSamples = applyRuntimeFilters(r, runtimeSamples)

	resp := struct {
		Frames  FrameTimeline   `json:"frames"`
		Runtime []RuntimeSample `json:"runtime"`
	}{
		Frames:  frames,
		Runtime: runtimeSamples,
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		http.Error(w, fmt.Sprintf("json encode error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func parseFloatQuery(r *http.Request, key string) float64 {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

// serializeWidgetTree recursively converts an element tree to JSON-serializable form.
// The depth parameter limits recursion to prevent stack overflow.
func serializeWidgetTree(elem core.Element, depth int) WidgetTreeNode {
	if elem == nil {
		return WidgetTreeNode{ElementType: "<nil>"}
	}

	widget := elem.Widget()
	node := WidgetTreeNode{
		ElementType: reflect.TypeOf(elem).String(),
		Depth:       elem.Depth(),
		NeedsBuild:  getNeedsBuild(elem),
	}

	if widget != nil {
		node.WidgetType = reflect.TypeOf(widget).String()
		node.Key = safeKey(widget.Key())
	}

	// Check if this is a stateful element
	if _, ok := elem.(*core.StatefulElement); ok {
		node.HasState = true
	}

	// Recurse into children (with depth limit)
	if depth < maxTreeDepth {
		elem.VisitChildren(func(child core.Element) bool {
			node.Children = append(node.Children, serializeWidgetTree(child, depth+1))
			return true
		})
	}

	return node
}

// safeKey converts a widget key to a JSON-safe value.
// Non-serializable types (funcs, chans, etc.) are converted to their string representation.
func safeKey(key any) any {
	if key == nil {
		return nil
	}
	switch key.(type) {
	case string, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool:
		return key
	default:
		// For complex types, use string representation to avoid JSON errors
		return fmt.Sprintf("%v", key)
	}
}

// getNeedsBuild safely retrieves the dirty/needsBuild flag from an element.
func getNeedsBuild(elem core.Element) bool {
	if elem == nil {
		return false
	}
	// The dirty field is unexported, but we can check via type assertion
	// to a common interface if available. For now, we use reflection.
	v := reflect.ValueOf(elem)
	if !v.IsValid() {
		return false
	}
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false
	}
	if dirty := v.FieldByName("dirty"); dirty.IsValid() && dirty.Kind() == reflect.Bool {
		return dirty.Bool()
	}
	return false
}

// serializeRenderTreeWithDepth recursively converts a render object tree to JSON-serializable form.
// The depth parameter limits recursion to prevent stack overflow.
func serializeRenderTreeWithDepth(obj layout.RenderObject, depth int) RenderTreeNode {
	size := obj.Size()
	node := RenderTreeNode{
		Type: reflect.TypeOf(obj).String(),
		Size: SafeSize{
			Width:  SafeFloat(size.Width),
			Height: SafeFloat(size.Height),
		},
		NeedsLayout:       getNeedsLayout(obj),
		NeedsPaint:        getNeedsPaint(obj),
		IsRepaintBoundary: obj.IsRepaintBoundary(),
	}

	// Get constraints if available
	if getter, ok := obj.(interface{ Constraints() layout.Constraints }); ok {
		c := getter.Constraints()
		node.Constraints = &SafeConstraints{
			MinWidth:  SafeFloat(c.MinWidth),
			MaxWidth:  SafeFloat(c.MaxWidth),
			MinHeight: SafeFloat(c.MinHeight),
			MaxHeight: SafeFloat(c.MaxHeight),
		}
	}

	// Get depth if available
	if getter, ok := obj.(interface{ Depth() int }); ok {
		node.Depth = getter.Depth()
	}

	// Get offset from parent data if available
	if pd, ok := obj.ParentData().(*layout.BoxParentData); ok {
		node.Offset = SafeOffset{
			X: SafeFloat(pd.Offset.X),
			Y: SafeFloat(pd.Offset.Y),
		}
	}

	// Recurse into children (with depth limit)
	if depth < maxTreeDepth {
		if cv, ok := obj.(layout.ChildVisitor); ok {
			cv.VisitChildren(func(child layout.RenderObject) {
				node.Children = append(node.Children, serializeRenderTreeWithDepth(child, depth+1))
			})
		}
	}

	return node
}

// getNeedsLayout safely retrieves the NeedsLayout flag.
func getNeedsLayout(obj layout.RenderObject) bool {
	if getter, ok := obj.(interface{ NeedsLayout() bool }); ok {
		return getter.NeedsLayout()
	}
	return false
}

// getNeedsPaint safely retrieves the NeedsPaint flag.
func getNeedsPaint(obj layout.RenderObject) bool {
	if getter, ok := obj.(interface{ NeedsPaint() bool }); ok {
		return getter.NeedsPaint()
	}
	return false
}
