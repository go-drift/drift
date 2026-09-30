package plugin

import (
	"errors"
	"strings"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

func mkBase(pkg string) protocol.Base {
	return protocol.Base{Pkg: pkg, Ident: pkg}
}

const firebaseSDK = "https://github.com/firebase/firebase-ios-sdk"

func spmOp(pkg, version string, products ...string) *protocol.OpIOSAddPackageDependency {
	return &protocol.OpIOSAddPackageDependency{
		Base:        mkBase(pkg),
		URL:         firebaseSDK,
		Requirement: driftplugin.SPMRequirementFrom(version),
		Products:    products,
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		ops  []protocol.Op
		// wantLen is the number of ops kept when there is no conflict.
		wantLen int
		// wantConflict, when set, is the contested target the error names.
		wantConflict string
		wantMixed    bool
	}{
		{
			name: "identical plist values collapse",
			ops: []protocol.Op{
				&protocol.OpInfoPlistSetString{Base: mkBase("a"), Key: "Foo", Value: "bar"},
				&protocol.OpInfoPlistSetString{Base: mkBase("b"), Key: "Foo", Value: "bar"},
			},
			wantLen: 1,
		},
		{
			name: "different plist values conflict",
			ops: []protocol.Op{
				&protocol.OpInfoPlistSetString{Base: mkBase("a"), Key: "Foo", Value: "x"},
				&protocol.OpInfoPlistSetString{Base: mkBase("b"), Key: "Foo", Value: "y"},
			},
			wantConflict: "plist:Foo",
		},
		{
			name: "string and bool on one plist key conflict",
			ops: []protocol.Op{
				&protocol.OpInfoPlistSetString{Base: mkBase("a"), Key: "Foo", Value: "true"},
				&protocol.OpInfoPlistSetBool{Base: mkBase("b"), Key: "Foo", Value: true},
			},
			wantConflict: "plist:Foo",
		},
		{
			name: "array items merge",
			ops: []protocol.Op{
				&protocol.OpInfoPlistAppendArrayItem{Base: mkBase("a"), Key: "Modes", Value: "fetch"},
				&protocol.OpInfoPlistAppendArrayItem{Base: mkBase("b"), Key: "Modes", Value: "fetch"},
				&protocol.OpInfoPlistAppendArrayItem{Base: mkBase("b"), Key: "Modes", Value: "remote-notification"},
			},
			wantLen: 2,
		},
		{
			name: "array item vs whole array conflict",
			ops: []protocol.Op{
				&protocol.OpInfoPlistAppendArrayItem{Base: mkBase("a"), Key: "Modes", Value: "fetch"},
				&protocol.OpInfoPlistSetStringArray{Base: mkBase("b"), Key: "Modes", Values: []string{"audio"}},
			},
			wantConflict: "plist:Modes",
			wantMixed:    true,
		},
		{
			name: "permissions merge",
			ops: []protocol.Op{
				&protocol.OpAndroidManifestAddPermission{Base: mkBase("a"), Name: "android.permission.CAMERA"},
				&protocol.OpAndroidManifestAddPermission{Base: mkBase("b"), Name: "android.permission.CAMERA"},
				&protocol.OpAndroidManifestAddPermission{Base: mkBase("c"), Name: "android.permission.INTERNET"},
			},
			wantLen: 2,
		},
		{
			name: "identical launch screens collapse",
			ops: []protocol.Op{
				&protocol.OpIOSReplaceLaunchScreen{Base: mkBase("a"), Content: "<document/>"},
				&protocol.OpIOSReplaceLaunchScreen{Base: mkBase("b"), Content: "<document/>"},
			},
			wantLen: 1,
		},
		{
			name: "different launch screens conflict",
			ops: []protocol.Op{
				&protocol.OpIOSReplaceLaunchScreen{Base: mkBase("a"), Content: "<document a=\"1\"/>"},
				&protocol.OpIOSReplaceLaunchScreen{Base: mkBase("b"), Content: "<document a=\"2\"/>"},
			},
			wantConflict: "ios:launch-screen",
		},
		{
			name: "drawable name ignores extension",
			ops: []protocol.Op{
				&protocol.OpAndroidWriteDrawable{Base: mkBase("a"), Name: "icon", Content: "AA=="},
				&protocol.OpAndroidWriteDrawable{Base: mkBase("b"), Name: "icon.webp", Content: "AA=="},
			},
			wantConflict: "android-res:drawable/icon",
		},
		{
			name: "drawable bitmap vs drawable XML with the same name",
			ops: []protocol.Op{
				&protocol.OpAndroidWriteDrawable{Base: mkBase("a"), Name: "splash", Content: "AA=="},
				&protocol.OpAndroidWriteResourceXML{Base: mkBase("b"), RelPath: "drawable/splash.xml", Content: "<layer-list/>"},
			},
			wantConflict: "android-res:drawable/splash",
		},
		{
			name: "night drawable does not clash with day drawable",
			ops: []protocol.Op{
				&protocol.OpAndroidWriteDrawable{Base: mkBase("a"), Name: "splash", Content: "AA=="},
				&protocol.OpAndroidWriteResourceXML{Base: mkBase("a"), RelPath: "drawable-night/splash.xml", Content: "<layer-list/>"},
			},
			wantLen: 2,
		},
		{
			name: "color vs color state list with the same name",
			ops: []protocol.Op{
				&protocol.OpAndroidColorSet{Base: mkBase("a"), Name: "brand", Value: "#FFFFFF"},
				&protocol.OpAndroidWriteResourceXML{Base: mkBase("b"), RelPath: "color/brand.xml", Content: "<selector/>"},
			},
			wantConflict: "android-res:color/brand",
		},
		{
			name: "Kotlin package and path aliasing",
			ops: []protocol.Op{
				&protocol.OpAddKotlinSource{Base: mkBase("a"), Package: "com.foo", RelPath: "Foo.kt", Content: "AA=="},
				&protocol.OpAddKotlinSource{Base: mkBase("b"), Package: "com", RelPath: "foo/Foo.kt", Content: "AQ=="},
			},
			wantConflict: "android-src:com/foo/Foo.kt",
		},
		{
			name: "Swift basename clash across groups",
			ops: []protocol.Op{
				&protocol.OpAddIOSSource{Base: mkBase("a"), Group: "A", RelPath: "Plugin.swift", Content: "AA=="},
				&protocol.OpAddIOSSource{Base: mkBase("b"), Group: "B", RelPath: "sub/Plugin.swift", Content: "AA=="},
			},
			wantConflict: "ios-swift-basename:Plugin.swift",
		},
		{
			name: "image set vs bundle PNG of the same name",
			ops: []protocol.Op{
				&protocol.OpIOSAssetsAddImageSet{Base: mkBase("a"), Name: "Logo", Image: "AA=="},
				&protocol.OpIOSAddBundleResource{Base: mkBase("b"), Path: "Logo.png", Content: "AA=="},
			},
			wantConflict: "ios-bundle:Logo.png",
		},
		{
			name: "shared SwiftPM package with different products merges",
			ops: []protocol.Op{
				spmOp("a", "11.0.0", "FirebaseCore"),
				spmOp("b", "11.0.0", "FirebaseCore", "FirebaseMessaging"),
			},
			wantLen: 2,
		},
		{
			name:    "identical SwiftPM requests collapse",
			ops:     []protocol.Op{spmOp("a", "11.0.0", "FirebaseCore"), spmOp("b", "11.0.0", "FirebaseCore")},
			wantLen: 1,
		},
		{
			name:         "shared SwiftPM package with different requirements conflicts",
			ops:          []protocol.Op{spmOp("a", "10.0.0", "FirebaseCore"), spmOp("b", "11.0.0", "FirebaseMessaging")},
			wantConflict: "spm:firebase-ios-sdk",
		},
		{
			name: "same Gradle plugin at different versions conflicts",
			ops: []protocol.Op{
				&protocol.OpAndroidGradleApplyPlugin{Base: mkBase("a"), ID: "com.google.gms.google-services", Version: "4.4.0"},
				&protocol.OpAndroidGradleApplyPlugin{Base: mkBase("b"), ID: "com.google.gms.google-services", Version: "4.3.15"},
			},
			wantConflict: "gradle-plugin:com.google.gms.google-services",
		},
		{
			name: "same Gradle plugin and version collapse",
			ops: []protocol.Op{
				&protocol.OpAndroidGradleApplyPlugin{Base: mkBase("a"), ID: "com.google.gms.google-services", Version: "4.4.0"},
				&protocol.OpAndroidGradleApplyPlugin{Base: mkBase("b"), ID: "com.google.gms.google-services", Version: "4.4.0"},
			},
			wantLen: 1,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := Validate(c.ops)
			if c.wantConflict == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(out) != c.wantLen {
					t.Errorf("kept %d ops, want %d", len(out), c.wantLen)
				}
				return
			}
			var ce *ConflictError
			if !errors.As(err, &ce) {
				t.Fatalf("expected ConflictError, got %v", err)
			}
			if !strings.HasPrefix(ce.Target, c.wantConflict) {
				t.Errorf("conflict target %q, want %q", ce.Target, c.wantConflict)
			}
			if ce.Mixed != c.wantMixed {
				t.Errorf("Mixed = %v, want %v", ce.Mixed, c.wantMixed)
			}
			if ce.First.PluginPackage() == ce.Second.PluginPackage() {
				t.Errorf("error should name both plugins: %v", ce)
			}
		})
	}
}

func TestValidateKeepsSourceOrder(t *testing.T) {
	ops := []protocol.Op{
		&protocol.OpRegistrantAndroid{Base: mkBase("z"), Symbol: "z.Z"},
		&protocol.OpRegistrantAndroid{Base: mkBase("a"), Symbol: "a.A"},
		&protocol.OpRegistrantAndroid{Base: mkBase("a"), Symbol: "z.Z"},
	}
	out, err := Validate(ops)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0] != ops[0] || out[1] != ops[1] {
		t.Errorf("want first two ops in source order, got %v", out)
	}
}
