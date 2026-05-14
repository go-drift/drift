package plugin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/cache"
	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// BridgeSource is the canonical content of tools/drift-plugins/main.go for
// a given plugin list. Exported for tests.
const bridgeTemplateVersion = "1"

// BridgeFilePath is the relative path of the generated bridge tool.
const BridgeFilePath = "tools/drift-plugins/main.go"

// Bridge is the resolved bridge binary.
type Bridge struct {
	BinaryPath string
}

// EnsureBridge generates tools/drift-plugins/main.go from the plugin list,
// builds the cached bridge binary if needed, and returns the path of the
// runnable binary.
func EnsureBridge(projectRoot, cliVersion string, plugins []ConfiguredPlugin, infos []*PackageInfo) (*Bridge, error) {
	if len(plugins) == 0 {
		return nil, fmt.Errorf("EnsureBridge: no plugins")
	}
	if len(plugins) != len(infos) {
		return nil, fmt.Errorf("EnsureBridge: plugin/info length mismatch")
	}

	src, err := GenerateBridgeSource(plugins)
	if err != nil {
		return nil, err
	}

	if err := writeBridgeFileAtomic(filepath.Join(projectRoot, BridgeFilePath), src); err != nil {
		return nil, err
	}

	// Bypass the shared bridge cache whenever a plugin's source is local:
	//   - Module.Replace != nil: explicit `replace` directive in go.mod
	//   - Module.Version == "": main module (Module.Main=true is reported
	//     with empty Version) or a `go.work use` workspace member
	// In all these cases the (package, module-version) tuple captured in the
	// cache key does not reflect source-level edits, so reusing a cached
	// bridge would silently miss the plugin changes. A per-build temp dir
	// forces a fresh `go build` against the current sources.
	bypass := false
	for _, info := range infos {
		if info == nil || info.Module == nil {
			continue
		}
		if info.Module.Replace != nil || info.Module.Version == "" {
			bypass = true
			break
		}
	}

	var outDir string
	if bypass {
		// Per-build temp dir
		tmp, err := os.MkdirTemp("", "drift-bridge-")
		if err != nil {
			return nil, fmt.Errorf("temp bridge dir: %w", err)
		}
		outDir = tmp
	} else {
		key, err := BridgeCacheKey(projectRoot, cliVersion, plugins, infos, src)
		if err != nil {
			return nil, err
		}
		outDir = filepath.Join(cacheBridgesRoot(), key)
	}
	binPath := filepath.Join(outDir, "bridge")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	if _, err := os.Stat(binPath); err == nil && !bypass {
		return &Bridge{BinaryPath: binPath}, nil
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("create bridge dir: %w", err)
	}
	fmt.Fprintln(os.Stderr, "Building Drift plugin bridge…")

	cmd := exec.Command("go", "build", "-tags", "drift_tool", "-o", binPath, "./"+filepath.ToSlash(filepath.Dir(BridgeFilePath)))
	cmd.Dir = projectRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, decorateBuildError(stderr.String(), err)
	}
	return &Bridge{BinaryPath: binPath}, nil
}

func cacheBridgesRoot() string {
	root, err := cache.Root()
	if err != nil || root == "" {
		root = filepath.Join(os.TempDir(), "drift")
	}
	return filepath.Join(root, "cache", "bridges")
}

// BridgeCacheKey returns the hash key for a given plugin set and project state.
func BridgeCacheKey(projectRoot, cliVersion string, plugins []ConfiguredPlugin, infos []*PackageInfo, src []byte) (string, error) {
	type pluginPin struct {
		Package string `json:"package"`
		Module  string `json:"module"`
		Version string `json:"version"`
	}
	pins := make([]pluginPin, len(plugins))
	for i, p := range plugins {
		pin := pluginPin{Package: p.Package}
		if i < len(infos) && infos[i] != nil && infos[i].Module != nil {
			pin.Module = infos[i].Module.Path
			pin.Version = infos[i].Module.Version
		}
		pins[i] = pin
	}
	sort.Slice(pins, func(i, j int) bool { return pins[i].Package < pins[j].Package })

	goSumHash, err := hashFile(filepath.Join(projectRoot, "go.sum"))
	if err != nil {
		return "", err
	}

	desc := struct {
		CLIVersion       string      `json:"cli_version"`
		APIVersion       int         `json:"api_version"`
		GoVersion        string      `json:"go_version"`
		GOOS             string      `json:"goos"`
		GOARCH           string      `json:"goarch"`
		Plugins          []pluginPin `json:"plugins"`
		BridgeTemplate   string      `json:"bridge_template"`
		BridgeSourceHash string      `json:"bridge_source_hash"`
		GoSumHash        string      `json:"go_sum_hash"`
	}{
		CLIVersion:       cliVersion,
		APIVersion:       driftplugin.APIVersion,
		GoVersion:        runtime.Version(),
		GOOS:             runtime.GOOS,
		GOARCH:           runtime.GOARCH,
		Plugins:          pins,
		BridgeTemplate:   bridgeTemplateVersion,
		BridgeSourceHash: sha256hex(src),
		GoSumHash:        goSumHash,
	}
	enc, err := json.Marshal(desc)
	if err != nil {
		return "", err
	}
	return sha256hex(enc), nil
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return sha256hex([]byte("")), nil
		}
		return "", err
	}
	return sha256hex(data), nil
}

func sha256hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// GenerateBridgeSource returns the canonical content of
// tools/drift-plugins/main.go for the given plugin list.
func GenerateBridgeSource(plugins []ConfiguredPlugin) ([]byte, error) {
	if len(plugins) == 0 {
		return nil, fmt.Errorf("generate bridge: empty plugin list")
	}
	aliases := assignAliases(plugins)

	var b bytes.Buffer
	b.WriteString("//go:build drift_tool\n\n")
	b.WriteString("// Code generated by drift CLI from drift.yaml. DO NOT EDIT.\n")
	b.WriteString("// Regenerated by drift build/run/plugin sync.\n\n")
	b.WriteString("package main\n\n")
	b.WriteString("import (\n")
	b.WriteString("\tdriftplugin \"github.com/go-drift/drift/pkg/plugin\"\n\n")
	for _, p := range plugins {
		fmt.Fprintf(&b, "\t%s \"%s\"\n", aliases[p.Package], p.Package)
	}
	b.WriteString(")\n\n")
	b.WriteString("func main() {\n")
	b.WriteString("\tdriftplugin.Main(\n")
	for _, p := range plugins {
		fmt.Fprintf(&b, "\t\tdriftplugin.Bind(%q, %s.Plugin),\n", p.Package, aliases[p.Package])
	}
	b.WriteString("\t)\n")
	b.WriteString("}\n")
	return b.Bytes(), nil
}

func assignAliases(plugins []ConfiguredPlugin) map[string]string {
	used := make(map[string]bool, len(plugins))
	out := make(map[string]string, len(plugins))
	for _, p := range plugins {
		base := lastSegment(p.Package)
		if base == "" {
			base = "plugin"
		}
		base = sanitizeAlias(base)
		candidate := base
		for i := 2; used[candidate]; i++ {
			candidate = fmt.Sprintf("%s_%d", base, i)
		}
		used[candidate] = true
		out[p.Package] = candidate
	}
	return out
}

func lastSegment(pkg string) string {
	// Strip a trailing "/plugin" segment, then take the last segment of the
	// remaining path so a package path like github.com/foo/drift-splash/plugin
	// aliases as `splash` rather than `plugin` (which would collide for every
	// plugin module shipping under <module>/plugin).
	trimmed, _ := strings.CutSuffix(pkg, "/plugin")
	parts := strings.Split(trimmed, "/")
	last := parts[len(parts)-1]
	// Strip a common drift- prefix so drift-splash → splash.
	last = strings.TrimPrefix(last, "drift-")
	return last
}

func sanitizeAlias(s string) string {
	if s == "" {
		return "plugin"
	}
	var b strings.Builder
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			if i == 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r)
		case r == '-' || r == '.':
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "plugin"
	}
	return strings.ToLower(out)
}

func writeBridgeFileAtomic(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create bridge tool dir: %w", err)
	}
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".bridge-*.go.tmp")
	if err != nil {
		return fmt.Errorf("temp bridge file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp bridge: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp bridge: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("rename temp bridge: %w", err)
	}
	return nil
}

func decorateBuildError(stderr string, err error) error {
	if stderr == "" {
		return fmt.Errorf("build plugin bridge: %w", err)
	}
	hint := ""
	if strings.Contains(stderr, ".Plugin") && (strings.Contains(stderr, "undefined") || strings.Contains(stderr, "cannot infer")) {
		hint = "\n\nHint: each plugin package must export a typed Plugin value:\n" +
			"    var Plugin driftplugin.Plugin[Config] = MyType{}\n" +
			"The concrete-struct form (var Plugin = MyType{}) does not work because\n" +
			"generic inference cannot reach the Config type parameter from a method\n" +
			"signature."
	}
	return fmt.Errorf("build plugin bridge: %w\n%s%s", err, strings.TrimRight(stderr, "\n"), hint)
}

// RunBridge executes the bridge binary with a build envelope and returns the
// decoded response.
func RunBridge(b *Bridge, env driftplugin.Envelope, logDir string) (*driftplugin.Response, error) {
	if b == nil || b.BinaryPath == "" {
		return nil, fmt.Errorf("RunBridge: bridge binary missing")
	}

	respFile, err := os.CreateTemp("", "drift-bridge-resp-*.json")
	if err != nil {
		return nil, fmt.Errorf("temp response file: %w", err)
	}
	respFile.Close()
	defer os.Remove(respFile.Name())

	cmd := exec.Command(b.BinaryPath, "--response-file="+respFile.Name())
	envBytes, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("encode envelope: %w", err)
	}
	cmd.Stdin = bytes.NewReader(envBytes)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	runErr := cmd.Run()

	if logDir != "" {
		_ = os.MkdirAll(logDir, 0o755)
		logPath := filepath.Join(logDir, "plugin-bridge.log")
		var logBuf bytes.Buffer
		logBuf.WriteString("=== plugin bridge run ===\n")
		logBuf.WriteString("stdout:\n")
		logBuf.Write(stdout.Bytes())
		logBuf.WriteString("\nstderr:\n")
		logBuf.Write(stderr.Bytes())
		_ = os.WriteFile(logPath, logBuf.Bytes(), 0o644)
	}

	respData, readErr := os.ReadFile(respFile.Name())
	if runErr != nil && (readErr != nil || len(respData) == 0) {
		return nil, fmt.Errorf("plugin bridge crashed during Build: %s", tailStderr(stderr.String()))
	}
	if runErr != nil {
		return nil, fmt.Errorf("plugin bridge exited non-zero: %s", tailStderr(stderr.String()))
	}
	if readErr != nil {
		return nil, fmt.Errorf("read response: %w", readErr)
	}

	var resp driftplugin.Response
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if resp.APIVersion != driftplugin.APIVersion {
		return nil, fmt.Errorf("bridge API version %d does not match CLI %d", resp.APIVersion, driftplugin.APIVersion)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("plugin bridge: %s", resp.Error)
	}
	return &resp, nil
}

func tailStderr(s string) string {
	s = strings.TrimRight(s, "\n")
	const max = 2_000
	if len(s) <= max {
		if s == "" {
			return "(no stderr)"
		}
		return s
	}
	return "…" + s[len(s)-max:]
}
