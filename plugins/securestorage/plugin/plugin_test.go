package plugin

import (
	"slices"
	"strings"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

func build(t *testing.T, config string) []protocol.Op {
	t.Helper()
	ctx := driftplugin.NewTestCtx()
	if err := driftplugin.Bind("github.com/go-drift/drift/plugins/securestorage/plugin", Plugin).Build(ctx, []byte(config)); err != nil {
		t.Fatal(err)
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("ctx.Err: %v", err)
	}
	return ctx.Ops()
}

type summary struct {
	deps       []string
	faceID     string
	biometrics string // which Biometrics.kt ships
}

func summarize(t *testing.T, ops []protocol.Op) summary {
	t.Helper()
	var s summary
	for _, op := range ops {
		switch o := op.(type) {
		case *protocol.OpAndroidGradleAddDependency:
			s.deps = append(s.deps, o.Coord)
		case *protocol.OpPlistSetString:
			if o.Key == "NSFaceIDUsageDescription" {
				s.faceID = o.Value
			}
		case *protocol.OpAddKotlinSource:
			if o.RelPath == "Biometrics.kt" {
				content, _ := protocol.DecodeContent(o.Content)
				if s.biometrics != "" {
					t.Fatal("two Biometrics.kt shipped")
				}
				s.biometrics = string(content)
			}
		}
	}
	return s
}

func TestBuildWithoutBiometrics(t *testing.T) {
	s := summarize(t, build(t, ""))
	if !slices.Equal(s.deps, []string{securityCrypto}) {
		t.Errorf("deps = %v, want security-crypto only", s.deps)
	}
	if s.faceID != "" {
		t.Error("Face ID usage string set without biometrics:")
	}
	if !strings.Contains(s.biometrics, "biometrics_not_configured") {
		t.Error("the refusing Biometrics.kt is not shipped")
	}
}

func TestBuildWithBiometrics(t *testing.T) {
	s := summarize(t, build(t, "biometrics:\n  face_id_description: Unlocks your vault.\n"))
	if !slices.Equal(s.deps, []string{securityCrypto, biometric}) {
		t.Errorf("deps = %v", s.deps)
	}
	if s.faceID != "Unlocks your vault." {
		t.Errorf("Face ID usage string = %q", s.faceID)
	}
	if !strings.Contains(s.biometrics, "BiometricPrompt") {
		t.Error("the BiometricPrompt Biometrics.kt is not shipped")
	}
}

func TestBuildRejectsEmptyFaceIDDescription(t *testing.T) {
	err := driftplugin.Bind("github.com/go-drift/drift/plugins/securestorage/plugin", Plugin).
		Build(driftplugin.NewTestCtx(), []byte("biometrics:\n  face_id_description: \" \"\n"))
	if err == nil || !strings.Contains(err.Error(), "biometrics.face_id_description: must not be empty") {
		t.Errorf("err = %v, want biometrics.face_id_description: must not be empty", err)
	}
}
