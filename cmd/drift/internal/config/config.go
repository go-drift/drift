package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"gopkg.in/yaml.v3"
)

// Config represents the optional drift.yaml configuration.
type Config struct {
	App    AppConfig    `yaml:"app"`
	Engine EngineConfig `yaml:"engine"`
}

// AppConfig contains application metadata.
type AppConfig struct {
	Name string `yaml:"name,omitempty"`
	ID   string `yaml:"id,omitempty"`
}

// EngineConfig contains engine settings.
type EngineConfig struct {
	Version string `yaml:"version,omitempty"`
}

// Resolved contains resolved configuration values.
type Resolved struct {
	Root          string
	ModulePath    string
	AppName       string
	AppID         string
	EngineVersion string
}

// LoadOptional reads drift.yaml if present.
func LoadOptional(dir string) (*Config, error) {
	path := filepath.Join(dir, "drift.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to read drift.yaml: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse drift.yaml: %w", err)
	}

	return &cfg, nil
}

// Resolve loads drift.yaml (if present) and resolves defaults.
func Resolve(dir string) (*Resolved, error) {
	modulePath, err := modulePath(dir)
	if err != nil {
		return nil, err
	}

	cfg, err := LoadOptional(dir)
	if err != nil {
		return nil, err
	}

	appName := strings.TrimSpace(cfg.App.Name)
	if appName == "" {
		appName = defaultAppName(modulePath, dir)
	}

	appID := strings.TrimSpace(cfg.App.ID)
	if appID == "" {
		appID = defaultAppID(modulePath, appName)
	}

	engineVersion := strings.TrimSpace(cfg.Engine.Version)
	if engineVersion == "" {
		engineVersion = "latest"
	}

	if err := validateAppID(appID); err != nil {
		return nil, err
	}

	return &Resolved{
		Root:          dir,
		ModulePath:    modulePath,
		AppName:       appName,
		AppID:         appID,
		EngineVersion: engineVersion,
	}, nil
}

// FindProjectRoot walks up from the current directory to find go.mod.
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not in a Go module (no go.mod found)")
		}
		dir = parent
	}
}

func modulePath(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}
	path := modfile.ModulePath(data)
	if path == "" {
		return "", fmt.Errorf("could not determine module path from go.mod")
	}
	return path, nil
}

func defaultAppName(modulePath, dir string) string {
	base := filepath.Base(dir)
	modName, _, ok := module.SplitPathVersion(modulePath)
	if ok {
		parts := strings.Split(modName, "/")
		if len(parts) > 0 {
			base = parts[len(parts)-1]
		}
	}
	if base == "" {
		return "drift_app"
	}
	return base
}

func defaultAppID(modulePath, appName string) string {
	parts := strings.Split(modulePath, "/")
	if len(parts) < 2 || !strings.Contains(parts[0], ".") {
		return fmt.Sprintf("com.example.%s", sanitizeSegment(appName, true))
	}

	host := strings.Split(parts[0], ".")
	for i, j := 0, len(host)-1; i < j; i, j = i+1, j-1 {
		host[i], host[j] = host[j], host[i]
	}

	var pathParts []string
	for _, p := range parts[1:] {
		if p == "" {
			continue
		}
		pathParts = append(pathParts, p)
	}

	segments := append(host, pathParts...)
	for i, segment := range segments {
		segments[i] = sanitizeSegment(segment, i > 0)
	}

	return strings.Join(segments, ".")
}

func sanitizeSegment(segment string, allowLeadingDigit bool) string {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		segment = "app"
	}

	var out []rune
	for _, r := range segment {
		switch {
		case r >= 'a' && r <= 'z':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		case r >= '0' && r <= '9':
			out = append(out, r)
		case r == '_' || r == '-':
			out = append(out, '_')
		default:
			out = append(out, '_')
		}
	}

	if len(out) == 0 {
		out = []rune("app")
	}

	if !allowLeadingDigit && out[0] >= '0' && out[0] <= '9' {
		out = append([]rune{'a'}, out...)
	}
	if out[0] == '_' {
		out = append([]rune{'a'}, out...)
	}

	return string(out)
}

func validateAppID(appID string) error {
	if !strings.Contains(appID, ".") {
		return fmt.Errorf("app.id must contain at least one '.' (got %q)", appID)
	}
	segments := strings.Split(appID, ".")
	for _, segment := range segments {
		if segment == "" {
			return fmt.Errorf("app.id contains an empty segment (%q)", appID)
		}
		if segment[0] >= '0' && segment[0] <= '9' {
			return fmt.Errorf("app.id segments cannot start with a digit (%q)", appID)
		}
		if segment[0] == '_' {
			return fmt.Errorf("app.id segments cannot start with '_' (%q)", appID)
		}
		for _, r := range segment {
			if !(r == '_' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
				return fmt.Errorf("app.id contains invalid character %q in %q", r, appID)
			}
		}
	}
	return nil
}
