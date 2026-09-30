package plugin

import (
	"errors"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

func mkBase(pkg string) driftplugin.Base {
	return driftplugin.Base{Pkg: pkg, Ident: pkg}
}

func TestValidateCollapsesIdenticalIdempotent(t *testing.T) {
	ops := []driftplugin.Op{
		&driftplugin.OpInfoPlistSetString{Base: mkBase("a"), Key: "Foo", Value: "bar"},
		&driftplugin.OpInfoPlistSetString{Base: mkBase("b"), Key: "Foo", Value: "bar"},
	}
	out, err := Validate(ops)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("expected 1 op after collapse, got %d", len(out))
	}
}

func TestValidateDivergentIdempotentFails(t *testing.T) {
	ops := []driftplugin.Op{
		&driftplugin.OpInfoPlistSetString{Base: mkBase("a"), Key: "Foo", Value: "x"},
		&driftplugin.OpInfoPlistSetString{Base: mkBase("b"), Key: "Foo", Value: "y"},
	}
	_, err := Validate(ops)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	var ce *ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("expected ConflictError, got %T", err)
	}
	if len(ce.Plugins) != 2 {
		t.Errorf("expected both plugins named in error, got %v", ce.Plugins)
	}
}

func TestValidateAdditiveMerges(t *testing.T) {
	ops := []driftplugin.Op{
		&driftplugin.OpAndroidManifestAddPermission{Base: mkBase("a"), Name: "android.permission.CAMERA"},
		&driftplugin.OpAndroidManifestAddPermission{Base: mkBase("b"), Name: "android.permission.CAMERA"},
		&driftplugin.OpAndroidManifestAddPermission{Base: mkBase("c"), Name: "android.permission.INTERNET"},
	}
	out, err := Validate(ops)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Errorf("expected 2 unique permissions, got %d", len(out))
	}
}

func TestValidateExclusiveSameContentCollapses(t *testing.T) {
	ops := []driftplugin.Op{
		&driftplugin.OpIOSReplaceLaunchScreen{Base: mkBase("a"), Content: "<x/>"},
		&driftplugin.OpIOSReplaceLaunchScreen{Base: mkBase("b"), Content: "<x/>"},
	}
	out, err := Validate(ops)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("expected collapse to 1 op, got %d", len(out))
	}
}

func TestValidateExclusiveDivergentFails(t *testing.T) {
	ops := []driftplugin.Op{
		&driftplugin.OpIOSReplaceLaunchScreen{Base: mkBase("a"), Content: "<a/>"},
		&driftplugin.OpIOSReplaceLaunchScreen{Base: mkBase("b"), Content: "<b/>"},
	}
	_, err := Validate(ops)
	var ce *ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("expected ConflictError, got %v", err)
	}
}

// Two plugins requesting the same SwiftPM URL with different version
// requirements must surface as a ConflictError, not silently pick one.
// SPM cannot reconcile two version constraints inside a single package
// graph; the user has to choose.
func TestValidateSPMDivergentRequirementsFail(t *testing.T) {
	ops := []driftplugin.Op{
		&driftplugin.OpIOSAddPackageDependency{
			Base:        mkBase("a"),
			URL:         "https://github.com/firebase/firebase-ios-sdk",
			Requirement: driftplugin.SPMRequirementFrom("10.0.0"),
			Products:    []string{"FirebaseAnalytics"},
		},
		&driftplugin.OpIOSAddPackageDependency{
			Base:        mkBase("b"),
			URL:         "https://github.com/firebase/firebase-ios-sdk",
			Requirement: driftplugin.SPMRequirementFrom("11.0.0"),
			Products:    []string{"FirebaseAnalytics"},
		},
	}
	_, err := Validate(ops)
	var ce *ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("expected ConflictError on divergent SPM requirements, got %v", err)
	}
}

// Two plugins on the same SwiftPM URL with identical payloads collapse
// silently (cooperating plugins both depending on Firebase is normal).
func TestValidateSPMIdenticalRequirementsCollapse(t *testing.T) {
	ops := []driftplugin.Op{
		&driftplugin.OpIOSAddPackageDependency{
			Base:        mkBase("a"),
			URL:         "https://github.com/firebase/firebase-ios-sdk",
			Requirement: driftplugin.SPMRequirementFrom("10.0.0"),
			Products:    []string{"FirebaseAnalytics"},
		},
		&driftplugin.OpIOSAddPackageDependency{
			Base:        mkBase("b"),
			URL:         "https://github.com/firebase/firebase-ios-sdk",
			Requirement: driftplugin.SPMRequirementFrom("10.0.0"),
			Products:    []string{"FirebaseAnalytics"},
		},
	}
	out, err := Validate(ops)
	if err != nil {
		t.Fatalf("identical requirements must collapse, got error: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("expected collapse to 1 op, got %d", len(out))
	}
}

// Same Gradle plugin id with divergent Version
// must conflict. Gradle cannot apply one plugin id at two versions inside
// a single build.
func TestValidateGradleApplyPluginDivergentClasspathFails(t *testing.T) {
	ops := []driftplugin.Op{
		&driftplugin.OpAndroidGradleApplyPlugin{
			Base:    mkBase("a"),
			ID:      "com.google.gms.google-services",
			Version: "4.4.0",
		},
		&driftplugin.OpAndroidGradleApplyPlugin{
			Base:    mkBase("b"),
			ID:      "com.google.gms.google-services",
			Version: "4.3.15",
		},
	}
	_, err := Validate(ops)
	var ce *ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("expected ConflictError on divergent Gradle plugin versions, got %v", err)
	}
}

// Same Gradle plugin id + same classpath_artifact collapse silently.
func TestValidateGradleApplyPluginIdenticalCollapse(t *testing.T) {
	ops := []driftplugin.Op{
		&driftplugin.OpAndroidGradleApplyPlugin{
			Base:    mkBase("a"),
			ID:      "com.google.gms.google-services",
			Version: "4.4.0",
		},
		&driftplugin.OpAndroidGradleApplyPlugin{
			Base:    mkBase("b"),
			ID:      "com.google.gms.google-services",
			Version: "4.4.0",
		},
	}
	out, err := Validate(ops)
	if err != nil {
		t.Fatalf("identical apply-plugin ops must collapse, got error: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("expected collapse to 1 op, got %d", len(out))
	}
}
