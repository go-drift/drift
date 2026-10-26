package plugin

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/plugin/mutate"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// opKind is everything the CLI knows about one op type: what it writes
// into a project and how. Apply, the ejected lock and the ejected-project
// checks all read it, so a new op type needs its protocol struct and one
// entry here (and a mutator, if it writes a new kind of file). Every field
// is optional.
type opKind struct {
	// edits returns the shared files the op edits, which the template and
	// the user write too (Info.plist, the manifest, build.gradle).
	edits func(p project, op protocol.Op) []string
	// edit names what the op changes inside those files.
	edit func(op protocol.Op) string
	// owns returns the whole files the op writes (mutate.OwnedFile). Apply
	// writes exactly these, and the ejected lock records them.
	owns func(p project, op protocol.Op) ([]mutate.OwnedFile, error)
	// line returns the exact line a versioned Gradle op writes, or "".
	line func(op protocol.Op) string
	// stage hands the op to the mutators of shared files and of output
	// Drift regenerates every build.
	stage func(b *batch, op protocol.Op)
}

// kind is an opKind for op type T, its functions typed.
type kind[T protocol.Op] struct {
	edits func(project, T) []string
	edit  func(T) string
	owns  func(project, T) ([]mutate.OwnedFile, error)
	line  func(T) string
	stage func(*batch, T)
}

// opKinds holds the opKind of every op type, keyed by its Go type.
var opKinds = map[reflect.Type]opKind{}

// register records k as T's opKind.
func register[T protocol.Op](k kind[T]) {
	t := reflect.TypeFor[T]()
	if _, dup := opKinds[t]; dup {
		panic(fmt.Sprintf("plugin: op type %v registered twice", t))
	}
	if (k.edits == nil) != (k.edit == nil) {
		panic(fmt.Sprintf("plugin: op type %v needs both edits and edit, or neither", t))
	}
	var e opKind
	if k.edits != nil {
		e.edits = func(p project, op protocol.Op) []string { return k.edits(p, op.(T)) }
		e.edit = func(op protocol.Op) string { return k.edit(op.(T)) }
	}
	if k.owns != nil {
		e.owns = func(p project, op protocol.Op) ([]mutate.OwnedFile, error) { return k.owns(p, op.(T)) }
	}
	if k.line != nil {
		e.line = func(op protocol.Op) string { return k.line(op.(T)) }
	}
	if k.stage != nil {
		e.stage = func(b *batch, op protocol.Op) { k.stage(b, op.(T)) }
	}
	opKinds[t] = e
}

// kindOf returns op's opKind. An op type without one is a CLI bug (a new
// op added to pkg/plugin/protocol without its entry), so it panics.
func kindOf(op protocol.Op) opKind {
	k, ok := opKinds[reflect.TypeOf(op)]
	if !ok {
		panic(fmt.Sprintf("plugin: op type %T (%s) has no opKind", op, op.Type()))
	}
	return k
}

// ownedFiles returns the whole files op writes into p.
func ownedFiles(op protocol.Op, p project) ([]mutate.OwnedFile, error) {
	k := kindOf(op)
	if k.owns == nil {
		return nil, nil
	}
	return k.owns(p, op)
}

// describeEdit names the edit op makes inside shared files of p, as
// "<files>: <what>", or "" if it makes none.
func describeEdit(op protocol.Op, p project) string {
	k := kindOf(op)
	if k.edits == nil {
		return ""
	}
	return strings.Join(k.edits(p, op), " and ") + ": " + k.edit(op)
}

// edits reports whether op edits the shared file rel of p.
func edits(op protocol.Op, p project, rel string) bool {
	k := kindOf(op)
	return k.edits != nil && slices.Contains(k.edits(p, op), rel)
}

// gradleLine is the line a versioned Gradle op writes, or "".
func gradleLine(op protocol.Op) string {
	k := kindOf(op)
	if k.line == nil {
		return ""
	}
	return k.line(op)
}

// batch collects, by mutator, the ops whose output is not whole files of
// their own: edits to shared files, and output Drift regenerates every
// build. Apply runs each mutator once with its ops.
type batch struct {
	plists      map[protocol.PlistFile][]protocol.PlistOp
	imageSets   []*protocol.OpIOSAssetsAddImageSet
	iosSources  []*protocol.OpAddIOSSource
	iosPlugins  []*protocol.OpIOSPlugin
	spm         []*protocol.OpIOSAddPackageDependency
	files       map[protocol.FileLocation][]*protocol.OpAddFile // syncedFiles only
	manifest    mutate.ManifestOps
	colors      []*protocol.OpAndroidColorSet
	strings     []*protocol.OpAndroidStringSet
	styles      []*protocol.OpAndroidStyleSet
	gradleDeps  []*protocol.OpAndroidGradleAddDependency
	gradlePlugs []*protocol.OpAndroidGradleApplyPlugin
}

// syncedFiles are the OpAddFile locations in a directory Drift owns and
// syncs every build, removing files no op adds (the bundle resources, the
// plugin config assets). A file anywhere else is owned on its own: written
// as it is and recorded by the ejected lock.
var syncedFiles = map[protocol.FileLocation]bool{
	protocol.FileIOSBundle:           true,
	protocol.FileIOSPluginConfig:     true,
	protocol.FileAndroidPluginConfig: true,
}

// one wraps a single owned file.
func one(f mutate.OwnedFile, err error) ([]mutate.OwnedFile, error) {
	if err != nil {
		return nil, err
	}
	return []mutate.OwnedFile{f}, nil
}

func plistKind[T protocol.PlistOp](edit func(T) string) kind[T] {
	return kind[T]{
		edits: func(p project, o T) []string { return []string{p.plist(o.PlistFile())} },
		edit:  edit,
		stage: func(b *batch, o T) {
			if b.plists == nil {
				b.plists = map[protocol.PlistFile][]protocol.PlistOp{}
			}
			b.plists[o.PlistFile()] = append(b.plists[o.PlistFile()], o)
		},
	}
}

// manifestKind edits the AndroidManifest; edit names the element.
func manifestKind[T protocol.Op](edit func(T) string, stage func(*mutate.ManifestOps, T)) kind[T] {
	return kind[T]{
		edits: func(p project, _ T) []string { return []string{p.manifest()} },
		edit:  edit,
		stage: func(b *batch, o T) { stage(&b.manifest, o) },
	}
}

func init() {
	// iOS
	register(plistKind(func(o *protocol.OpPlistSet) string { return "key " + o.Key }))
	register(plistKind(func(o *protocol.OpPlistAppendArrayItem) string {
		return fmt.Sprintf("%q in array %s", o.Value, o.Key)
	}))
	register(kind[*protocol.OpIOSAssetsAddImageSet]{
		owns: func(p project, o *protocol.OpIOSAssetsAddImageSet) ([]mutate.OwnedFile, error) {
			if p.xtool() {
				return nil, nil // a loose bundle PNG, pruned with the bundle resources
			}
			return mutate.ImageSetFiles(p.abs(p.assetCatalog()), o)
		},
		stage: func(b *batch, o *protocol.OpIOSAssetsAddImageSet) { b.imageSets = append(b.imageSets, o) },
	})
	register(kind[*protocol.OpIOSReplaceLaunchScreen]{
		owns: func(p project, o *protocol.OpIOSReplaceLaunchScreen) ([]mutate.OwnedFile, error) {
			return one(mutate.LaunchScreenFile(p.abs(p.launchScreen()), o), nil)
		},
	})
	register(kind[*protocol.OpAddIOSSource]{
		stage: func(b *batch, o *protocol.OpAddIOSSource) { b.iosSources = append(b.iosSources, o) },
	})
	register(kind[*protocol.OpIOSPlugin]{
		// Rendered by WriteRegistrant; Apply only checks it has a module.
		stage: func(b *batch, o *protocol.OpIOSPlugin) { b.iosPlugins = append(b.iosPlugins, o) },
	})
	register(kind[*protocol.OpIOSAddPackageDependency]{
		stage: func(b *batch, o *protocol.OpIOSAddPackageDependency) { b.spm = append(b.spm, o) },
	})
	register(kind[*protocol.OpAddFile]{
		owns: func(p project, o *protocol.OpAddFile) ([]mutate.OwnedFile, error) {
			if syncedFiles[o.Location] {
				return nil, nil
			}
			return one(mutate.AddedFile(p.abs(p.fileDir(o.Location)), o))
		},
		stage: func(b *batch, o *protocol.OpAddFile) {
			if !syncedFiles[o.Location] {
				return
			}
			if b.files == nil {
				b.files = map[protocol.FileLocation][]*protocol.OpAddFile{}
			}
			b.files[o.Location] = append(b.files[o.Location], o)
		},
	})

	// Android
	register(manifestKind(func(o *protocol.OpAndroidManifestAddPermission) string {
		if o.MaxSDK > 0 {
			return fmt.Sprintf("<uses-permission android:name=%q android:maxSdkVersion=\"%d\">", o.Name, o.MaxSDK)
		}
		return fmt.Sprintf("<uses-permission android:name=%q>", o.Name)
	}, func(m *mutate.ManifestOps, o *protocol.OpAndroidManifestAddPermission) {
		m.Permissions = append(m.Permissions, o)
	}))
	register(manifestKind(func(o *protocol.OpAndroidManifestAddIntentFilter) string {
		return fmt.Sprintf("%s on activity %s", strings.Join(strings.Fields(o.XML), " "), o.Activity)
	}, func(m *mutate.ManifestOps, o *protocol.OpAndroidManifestAddIntentFilter) {
		m.IntentFilters = append(m.IntentFilters, o)
	}))
	register(manifestKind(func(o *protocol.OpAndroidManifestSetActivityAttr) string {
		return fmt.Sprintf("%s=%q on activity %s", o.Attr, o.Value, o.Activity)
	}, func(m *mutate.ManifestOps, o *protocol.OpAndroidManifestSetActivityAttr) {
		m.ActivityAttrs = append(m.ActivityAttrs, o)
	}))
	register(manifestKind(func(o *protocol.OpAndroidManifestAddMetaData) string {
		return fmt.Sprintf("<meta-data android:name=%q> under %s", o.Name, o.Parent)
	}, func(m *mutate.ManifestOps, o *protocol.OpAndroidManifestAddMetaData) {
		m.MetaData = append(m.MetaData, o)
	}))
	register(manifestKind(func(o *protocol.OpAndroidManifestAddComponent) string {
		return fmt.Sprintf("<%s android:name=%q>", o.Tag(), o.Name())
	}, func(m *mutate.ManifestOps, o *protocol.OpAndroidManifestAddComponent) {
		m.Components = append(m.Components, o)
	}))
	register(kind[*protocol.OpAndroidColorSet]{
		stage: func(b *batch, o *protocol.OpAndroidColorSet) { b.colors = append(b.colors, o) },
	})
	register(kind[*protocol.OpAndroidStringSet]{
		stage: func(b *batch, o *protocol.OpAndroidStringSet) { b.strings = append(b.strings, o) },
	})
	register(kind[*protocol.OpAndroidStyleSet]{
		stage: func(b *batch, o *protocol.OpAndroidStyleSet) { b.styles = append(b.styles, o) },
	})
	register(kind[*protocol.OpAddKotlinSource]{
		owns: func(p project, o *protocol.OpAddKotlinSource) ([]mutate.OwnedFile, error) {
			return one(mutate.KotlinSourceFile(p.abs(p.javaRoot()), o))
		},
	})
	register(kind[*protocol.OpAndroidPlugin]{}) // rendered by WriteRegistrant
	register(kind[*protocol.OpAndroidGradleAddDependency]{
		edits: func(p project, _ *protocol.OpAndroidGradleAddDependency) []string { return []string{p.appGradle()} },
		edit: func(o *protocol.OpAndroidGradleAddDependency) string {
			return fmt.Sprintf("%s %q", o.Configuration, o.Coord)
		},
		line:  mutate.GradleDependencyLine,
		stage: func(b *batch, o *protocol.OpAndroidGradleAddDependency) { b.gradleDeps = append(b.gradleDeps, o) },
	})
	register(kind[*protocol.OpAndroidGradleApplyPlugin]{
		// A versioned plugin is also declared in the project build.gradle.
		edits: func(p project, o *protocol.OpAndroidGradleApplyPlugin) []string {
			if o.Version != "" {
				return []string{p.appGradle(), p.projectGradle()}
			}
			return []string{p.appGradle()}
		},
		edit: func(o *protocol.OpAndroidGradleApplyPlugin) string { return "Gradle plugin " + o.ID },
		line: func(o *protocol.OpAndroidGradleApplyPlugin) string {
			if o.Version == "" {
				return ""
			}
			return mutate.GradlePluginDeclaration(o)
		},
		stage: func(b *batch, o *protocol.OpAndroidGradleApplyPlugin) { b.gradlePlugs = append(b.gradlePlugs, o) },
	})
}
