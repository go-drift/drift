package plugin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/cache"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// bridgeTemplateVersion versions the generated bridge layout; bumping it
// invalidates every cached bridge binary.
const bridgeTemplateVersion = "1"

// BridgeFilePath is the relative path of the generated bridge tool.
const BridgeFilePath = "tools/drift-plugins/main.go"

// Bridge is the resolved bridge binary.
type Bridge struct {
	BinaryPath string
}

// EnsureBridge generates tools/drift-plugins/main.go from the plugin list,
// builds the bridge binary if needed, and returns the path of the runnable
// binary.
//
// The build never edits go.mod or go.sum (-mod=readonly); missing entries
// fail with a `go mod tidy` hint. Binaries are cached by a key over
// everything that feeds the build, except when any source in the build is
// local (a plugin in the main module, a directory replace such as a local
// github.com/go-drift/drift checkout, or a multi-module go.work): then the
// key cannot see source edits, so the bridge is rebuilt every time into a
// fixed per-project path, which the Go build cache keeps cheap.
func EnsureBridge(projectRoot, cliVersion string, plugins []ConfiguredPlugin, infos []*PackageInfo) (*Bridge, error) {
	if len(plugins) != len(infos) {
		return nil, fmt.Errorf("EnsureBridge: plugin/info length mismatch")
	}
	src, err := writeBridgeSource(projectRoot, plugins)
	if err != nil {
		return nil, err
	}

	env, err := readGoToolchain(projectRoot)
	if err != nil {
		return nil, err
	}
	local, err := hasLocalSources(projectRoot, infos)
	if err != nil {
		return nil, err
	}

	binName := "bridge"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	var binPath string
	if local {
		binPath = filepath.Join(cacheBridgesRoot(), "local", sha256hex([]byte(projectRoot))[:12], binName)
	} else {
		key, err := bridgeCacheKey(projectRoot, cliVersion, env, plugins, infos, src)
		if err != nil {
			return nil, err
		}
		binPath = filepath.Join(cacheBridgesRoot(), key, binName)
		if info, err := os.Stat(binPath); err == nil && info.Mode().IsRegular() {
			return &Bridge{BinaryPath: binPath}, nil
		}
	}

	fmt.Fprintln(os.Stderr, "Building Drift plugin bridge…")
	if err := buildBridge(projectRoot, binPath); err != nil {
		return nil, err
	}
	return &Bridge{BinaryPath: binPath}, nil
}

// writeBridgeSource writes tools/drift-plugins/main.go for plugins (only
// when its content changed) and returns the source.
func writeBridgeSource(projectRoot string, plugins []ConfiguredPlugin) ([]byte, error) {
	src, err := GenerateBridgeSource(plugins)
	if err != nil {
		return nil, err
	}
	if err := writeBridgeFileAtomic(filepath.Join(projectRoot, BridgeFilePath), src); err != nil {
		return nil, err
	}
	return src, nil
}

// buildBridge compiles the bridge tool to binPath atomically: the binary is
// linked under a temporary name in the same directory and renamed into
// place, so an interrupted or concurrent build never leaves a truncated
// binary at binPath.
func buildBridge(projectRoot, binPath string) error {
	dir := filepath.Dir(binPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create bridge dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".bridge-*.tmp")
	if err != nil {
		return fmt.Errorf("temp bridge binary: %w", err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name()) // no-op after a successful rename

	cmd := exec.Command("go", "build", "-mod=readonly", "-tags", "drift_tool", "-o", tmp.Name(), "./"+filepath.ToSlash(filepath.Dir(BridgeFilePath)))
	cmd.Dir = projectRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = os.Stderr
	if err := cmd.Run(); err != nil {
		return decorateBuildError(projectRoot, stderr.String(), err)
	}
	if err := os.Rename(tmp.Name(), binPath); err != nil {
		return fmt.Errorf("install bridge binary: %w", err)
	}
	return nil
}

func cacheBridgesRoot() string {
	root, err := cache.Root()
	if err != nil || root == "" {
		root = filepath.Join(os.TempDir(), "drift")
	}
	return filepath.Join(root, "cache", "bridges")
}

// goToolchain is the part of `go env` that changes what `go build`
// produces for the same sources.
type goToolchain struct {
	Version string `json:"GOVERSION"`
	Flags   string `json:"GOFLAGS"`
	// Work is the active go.work file, or "" / "off".
	Work string `json:"GOWORK"`
}

func readGoToolchain(projectRoot string) (goToolchain, error) {
	cmd := exec.Command("go", "env", "-json", "GOVERSION", "GOFLAGS", "GOWORK")
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return goToolchain{}, fmt.Errorf("go env: %w", commandError(err))
	}
	var env goToolchain
	if err := json.Unmarshal(out, &env); err != nil {
		return goToolchain{}, fmt.Errorf("decode go env: %w", err)
	}
	return env, nil
}

// bridgeCacheKey hashes every input of the bridge build: the CLI and
// protocol versions, the go toolchain, the plugin pins, the bridge source,
// and the project's go.mod, go.sum and active go.work.
func bridgeCacheKey(projectRoot, cliVersion string, env goToolchain, plugins []ConfiguredPlugin, infos []*PackageInfo, src []byte) (string, error) {
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

	files := map[string]string{}
	paths := []string{filepath.Join(projectRoot, "go.mod"), filepath.Join(projectRoot, "go.sum")}
	if env.Work != "" && env.Work != "off" {
		paths = append(paths, env.Work, env.Work+".sum")
	}
	for _, p := range paths {
		h, err := hashFile(p)
		if err != nil {
			return "", err
		}
		files[filepath.Base(p)] = h
	}

	desc := struct {
		CLIVersion       string            `json:"cli_version"`
		APIVersion       int               `json:"api_version"`
		Go               goToolchain       `json:"go"`
		GOOS             string            `json:"goos"`
		GOARCH           string            `json:"goarch"`
		Plugins          []pluginPin       `json:"plugins"`
		BridgeTemplate   string            `json:"bridge_template"`
		BridgeSourceHash string            `json:"bridge_source_hash"`
		Files            map[string]string `json:"files"`
	}{
		CLIVersion:       cliVersion,
		APIVersion:       protocol.APIVersion,
		Go:               env,
		GOOS:             runtime.GOOS,
		GOARCH:           runtime.GOARCH,
		Plugins:          pins,
		BridgeTemplate:   bridgeTemplateVersion,
		BridgeSourceHash: sha256hex(src),
		Files:            files,
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
	// The generated file already imports driftplugin.
	used := map[string]bool{"driftplugin": true}
	out := make(map[string]string, len(plugins))
	for _, p := range plugins {
		base := sanitizeAlias(lastSegment(p.Package))
		if reservedAlias(base) {
			base += "_plugin"
		}
		candidate := base
		for i := 2; used[candidate]; i++ {
			candidate = fmt.Sprintf("%s_%d", base, i)
		}
		used[candidate] = true
		out[p.Package] = candidate
	}
	return out
}

// reservedAlias reports whether s cannot (keywords, "_", "init", "main")
// or should not (predeclared identifiers such as "string" or "len") name
// an import in the generated main package.
func reservedAlias(s string) bool {
	switch s {
	case "_", "init", "main":
		return true
	}
	return token.IsKeyword(s) || types.Universe.Lookup(s) != nil
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

func decorateBuildError(projectRoot, stderr string, err error) error {
	if stderr == "" {
		return fmt.Errorf("build plugin bridge: %w", err)
	}
	hint := ""
	switch {
	case strings.Contains(stderr, ".Plugin") && (strings.Contains(stderr, "undefined") || strings.Contains(stderr, "cannot infer")):
		hint = "\n\nHint: each plugin package must export a typed Plugin value:\n" +
			"    var Plugin driftplugin.Plugin[Config] = MyType{}\n" +
			"The concrete-struct form (var Plugin = MyType{}) does not work because\n" +
			"generic inference cannot reach the Config type parameter from a method\n" +
			"signature."
	case needsTidy(stderr):
		hint = tidyHint(projectRoot)
	}
	return fmt.Errorf("build plugin bridge: %w\n%s%s", err, strings.TrimRight(stderr, "\n"), hint)
}

// needsTidy reports whether go output says go.mod or go.sum lacks entries,
// which Drift never adds itself.
func needsTidy(stderr string) bool {
	return strings.Contains(stderr, "missing go.sum entry") ||
		strings.Contains(stderr, "updates to go.mod needed") ||
		strings.Contains(stderr, "no required module provides package")
}

func tidyHint(projectRoot string) string {
	return "\n\nHint: go.mod/go.sum are missing entries the plugin bridge needs. Drift does not edit them itself; run:\n" +
		"    cd " + projectRoot + " && go mod tidy\n" +
		"(or `drift plugin sync --tidy`), then commit go.mod, go.sum and tools/drift-plugins/main.go together."
}

// RunBridge executes the bridge binary with a build envelope and returns the
// decoded response.
func RunBridge(b *Bridge, env protocol.Envelope, logDir string) (*protocol.Response, error) {
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

	var resp protocol.Response
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if resp.APIVersion != protocol.APIVersion {
		return nil, fmt.Errorf("bridge API version %d does not match CLI %d", resp.APIVersion, protocol.APIVersion)
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
