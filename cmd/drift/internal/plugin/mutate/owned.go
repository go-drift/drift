package mutate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// OwnedFile is a whole file a plugin op writes into the project, with the
// bytes it writes. Drift creates and owns these files, or, for the launch
// screen, takes over the template's file whole, unlike edits inside files
// the scaffold or the user owns (Info.plist, AndroidManifest.xml,
// build.gradle). Apply writes exactly these files (WriteOwned) and the
// ejected-project lock records them, so the lock records what was written.
type OwnedFile struct {
	Path    string
	Content []byte
}

// ImageSetFiles are the image and Contents.json an OpIOSAssetsAddImageSet
// writes under <assetsRoot>/<Name>.imageset/.
func ImageSetFiles(assetsRoot string, op *protocol.OpIOSAssetsAddImageSet) ([]OwnedFile, error) {
	image, err := protocol.DecodeContent(op.Image)
	if err != nil {
		return nil, fmt.Errorf("decode image %s: %w", op.Name, err)
	}
	contents, err := imagesetContentsJSON(op.Name + ".png")
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(assetsRoot, op.Name+".imageset")
	return []OwnedFile{
		{Path: filepath.Join(dir, op.Name+".png"), Content: image},
		{Path: filepath.Join(dir, "Contents.json"), Content: contents},
	}, nil
}

// LaunchScreenFile is the storyboard an OpIOSReplaceLaunchScreen writes at
// path, in place of the project template's.
func LaunchScreenFile(path string, op *protocol.OpIOSReplaceLaunchScreen) OwnedFile {
	return OwnedFile{Path: path, Content: []byte(op.Content)}
}

// KotlinSourceFile is the source an OpAddKotlinSource writes under
// <javaRoot>/<package as path>/<rel>.
func KotlinSourceFile(javaRoot string, op *protocol.OpAddKotlinSource) (OwnedFile, error) {
	content, err := protocol.DecodeContent(op.Content)
	if err != nil {
		return OwnedFile{}, fmt.Errorf("decode Kotlin source %s: %w", op.RelPath, err)
	}
	parts := append([]string{javaRoot}, strings.Split(op.Package, ".")...)
	parts = append(parts, filepath.FromSlash(op.RelPath))
	return OwnedFile{Path: filepath.Join(parts...), Content: content}, nil
}

// DrawableFile is the bitmap an OpAndroidWriteDrawable writes into
// drawableDir; a name without an extension gets .png.
func DrawableFile(drawableDir string, op *protocol.OpAndroidWriteDrawable) (OwnedFile, error) {
	content, err := protocol.DecodeContent(op.Content)
	if err != nil {
		return OwnedFile{}, fmt.Errorf("decode drawable %s: %w", op.Name, err)
	}
	name := op.Name
	if filepath.Ext(name) == "" {
		name += ".png"
	}
	return OwnedFile{Path: filepath.Join(drawableDir, name), Content: content}, nil
}

// ResourceXMLFile is the file an OpAndroidWriteResourceXML writes under
// resRoot.
func ResourceXMLFile(resRoot string, op *protocol.OpAndroidWriteResourceXML) OwnedFile {
	return OwnedFile{Path: filepath.Join(resRoot, filepath.FromSlash(op.RelPath)), Content: []byte(op.Content)}
}

// AppModuleFile is the file an OpAndroidAddAppModuleFile writes into the
// app module directory.
func AppModuleFile(appDir string, op *protocol.OpAndroidAddAppModuleFile) (OwnedFile, error) {
	content, err := protocol.DecodeContent(op.Content)
	if err != nil {
		return OwnedFile{}, fmt.Errorf("Android app module file %s: decode content: %w", op.Name, err)
	}
	return OwnedFile{Path: filepath.Join(appDir, op.Name), Content: content}, nil
}

// WriteOwned plans each file and returns the paths whose bytes change.
func WriteOwned(cs *Changeset, files ...OwnedFile) ([]string, error) {
	var changed []string
	for _, f := range files {
		ch, err := cs.WriteFile(f.Path, f.Content)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, f.Path)
		}
	}
	return changed, nil
}
