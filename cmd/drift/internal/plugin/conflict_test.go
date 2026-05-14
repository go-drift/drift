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
