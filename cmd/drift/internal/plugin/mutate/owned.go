package mutate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// OwnedFile is a whole file a plugin op writes into the project, with the
// bytes it writes. Drift creates and owns these files, unlike edits inside
// files the scaffold or the user owns (Info.plist, AndroidManifest.xml,
// build.gradle). The writers below and the ejected-project lock both use
// these functions, so the lock records exactly what was written.
type OwnedFile struct {
	Path    string
	Content []byte
}

// IOSSourceFile is the Swift source an OpAddIOSSource writes under
// <pluginsRoot>/<group>/<rel>.
func IOSSourceFile(pluginsRoot string, op *protocol.OpAddIOSSource) (OwnedFile, error) {
	content, err := protocol.DecodeContent(op.Content)
	if err != nil {
		return OwnedFile{}, fmt.Errorf("decode iOS source %s: %w", op.RelPath, err)
	}
	return OwnedFile{Path: filepath.Join(pluginsRoot, op.Group, filepath.FromSlash(op.RelPath)), Content: content}, nil
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

// AndroidAssetFile is the file an OpAndroidAddAsset writes under
// assetsRoot.
func AndroidAssetFile(assetsRoot string, op *protocol.OpAndroidAddAsset) (OwnedFile, error) {
	content, err := protocol.DecodeContent(op.Content)
	if err != nil {
		return OwnedFile{}, fmt.Errorf("Android asset %s: decode content: %w", op.Path, err)
	}
	return OwnedFile{Path: filepath.Join(assetsRoot, filepath.FromSlash(op.Path)), Content: content}, nil
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

// writeOwned writes each file whose bytes differ and returns the paths it
// changed.
func writeOwned(files ...OwnedFile) ([]string, error) {
	var changed []string
	for _, f := range files {
		ch, err := writeIfDifferent(f.Path, f.Content)
		if err != nil {
			return changed, fmt.Errorf("write %s: %w", f.Path, err)
		}
		if ch {
			changed = append(changed, f.Path)
		}
	}
	return changed, nil
}

// writeEach writes the OwnedFile each op produces.
func writeEach[O any](ops []O, file func(O) (OwnedFile, error)) ([]string, error) {
	var changed []string
	for _, op := range ops {
		f, err := file(op)
		if err != nil {
			return changed, err
		}
		paths, err := writeOwned(f)
		changed = append(changed, paths...)
		if err != nil {
			return changed, err
		}
	}
	return changed, nil
}
