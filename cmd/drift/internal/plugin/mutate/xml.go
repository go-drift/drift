package mutate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/beevik/etree"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// ApplyAndroidManifest applies manifest ops to the file at path. Existing
// nodes are preserved (etree round-trips comments and unrelated siblings).
// Returns changed=true iff the file's bytes actually changed.
func ApplyAndroidManifest(
	path string,
	addPerms []*driftplugin.OpAndroidManifestAddPermission,
	addIntents []*driftplugin.OpAndroidManifestAddIntentFilter,
	setAttrs []*driftplugin.OpAndroidManifestSetActivityAttr,
	addMeta []*driftplugin.OpAndroidManifestAddMetaData,
) (bool, error) {
	doc, original, err := loadXML(path)
	if err != nil {
		return false, fmt.Errorf("read AndroidManifest: %w", err)
	}
	manifest := doc.SelectElement("manifest")
	if manifest == nil {
		return false, fmt.Errorf("AndroidManifest.xml has no <manifest> root")
	}

	for _, op := range addPerms {
		ensurePermission(manifest, op.Name)
	}

	app := manifest.SelectElement("application")
	if app == nil && len(setAttrs)+len(addIntents)+len(addMeta) > 0 {
		return false, fmt.Errorf("AndroidManifest.xml has no <application>; cannot apply activity/intent ops")
	}

	for _, op := range setAttrs {
		act := findActivity(app, op.Activity)
		if act == nil {
			return false, fmt.Errorf("AndroidManifest.xml: activity %q not found for SetActivityAttr", op.Activity)
		}
		setNSAttr(act, op.Attr, op.Value)
	}

	for _, op := range addIntents {
		act := findActivity(app, op.Activity)
		if act == nil {
			return false, fmt.Errorf("AndroidManifest.xml: activity %q not found for AddIntentFilter", op.Activity)
		}
		if err := appendIntentFilter(act, op.XML); err != nil {
			return false, err
		}
	}

	for _, op := range addMeta {
		parent := app
		if rest, ok := strings.CutPrefix(op.Parent, "activity:"); ok {
			parent = findActivity(app, rest)
			if parent == nil {
				return false, fmt.Errorf("AndroidManifest.xml: activity %q not found for AddMetaData", op.Parent)
			}
		}
		ensureMetaData(parent, op.Name, op.Value)
	}

	doc.Indent(4)
	out, err := doc.WriteToBytes()
	if err != nil {
		return false, fmt.Errorf("serialize AndroidManifest: %w", err)
	}
	if bytes.Equal(out, original) {
		return false, nil
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return false, fmt.Errorf("write AndroidManifest: %w", err)
	}
	return true, nil
}

func ensurePermission(manifest *etree.Element, name string) {
	for _, el := range manifest.SelectElements("uses-permission") {
		if attr := el.SelectAttr("android:name"); attr != nil && attr.Value == name {
			return
		}
	}
	perm := etree.NewElement("uses-permission")
	perm.CreateAttr("android:name", name)
	// Place at the end of the permissions block (immediately before <application>)
	// or at the end of <manifest> if there's no <application>.
	app := manifest.SelectElement("application")
	if app != nil {
		manifest.InsertChildAt(app.Index(), perm)
	} else {
		manifest.AddChild(perm)
	}
}

func findActivity(app *etree.Element, name string) *etree.Element {
	if app == nil {
		return nil
	}
	for _, el := range app.SelectElements("activity") {
		if attr := el.SelectAttr("android:name"); attr != nil && attr.Value == name {
			return el
		}
	}
	return nil
}

func setNSAttr(el *etree.Element, attr, value string) {
	if !strings.Contains(attr, ":") {
		attr = "android:" + attr
	}
	existing := el.SelectAttr(attr)
	if existing != nil && existing.Value == value {
		return
	}
	if existing != nil {
		existing.Value = value
		return
	}
	el.CreateAttr(attr, value)
}

func appendIntentFilter(activity *etree.Element, snippet string) error {
	sub := etree.NewDocument()
	if err := sub.ReadFromString(snippet); err != nil {
		return fmt.Errorf("parse intent-filter snippet: %w", err)
	}
	root := sub.Root()
	if root == nil || root.Tag != "intent-filter" {
		return fmt.Errorf("intent-filter snippet must be a single <intent-filter> element")
	}
	// Skip if an equivalent filter is already present (same canonical form).
	canonical := canonicalIntent(root)
	for _, existing := range activity.SelectElements("intent-filter") {
		if canonicalIntent(existing) == canonical {
			return nil
		}
	}
	activity.AddChild(root.Copy())
	return nil
}

func canonicalIntent(el *etree.Element) string {
	var parts []string
	for _, child := range el.ChildElements() {
		var bits []string
		bits = append(bits, child.Tag)
		var attrs []string
		for _, a := range child.Attr {
			attrs = append(attrs, a.FullKey()+"="+a.Value)
		}
		sort.Strings(attrs)
		bits = append(bits, attrs...)
		parts = append(parts, strings.Join(bits, "|"))
	}
	sort.Strings(parts)
	return strings.Join(parts, "##")
}

func ensureMetaData(parent *etree.Element, name, value string) {
	for _, el := range parent.SelectElements("meta-data") {
		nameAttr := el.SelectAttr("android:name")
		if nameAttr == nil || nameAttr.Value != name {
			continue
		}
		setNSAttr(el, "android:value", value)
		return
	}
	meta := etree.NewElement("meta-data")
	meta.CreateAttr("android:name", name)
	meta.CreateAttr("android:value", value)
	parent.AddChild(meta)
}

// ApplyAndroidColors writes/updates a colors.xml at path with the given
// OpAndroidColorSet entries.
func ApplyAndroidColors(path string, ops []*driftplugin.OpAndroidColorSet) (string, bool, error) {
	return applyValuesXML(path, "color", func(d *etree.Document) {
		root := ensureResourcesRoot(d)
		for _, op := range ops {
			setValueEntry(root, "color", op.Name, op.Value)
		}
	})
}

// ApplyAndroidStrings writes/updates a strings.xml at path.
func ApplyAndroidStrings(path string, ops []*driftplugin.OpAndroidStringSet) (string, bool, error) {
	return applyValuesXML(path, "string", func(d *etree.Document) {
		root := ensureResourcesRoot(d)
		for _, op := range ops {
			setValueEntry(root, "string", op.Name, op.Value)
		}
	})
}

// ApplyAndroidStyles writes/updates a styles.xml at path.
func ApplyAndroidStyles(path string, ops []*driftplugin.OpAndroidStyleSet) (string, bool, error) {
	return applyValuesXML(path, "style", func(d *etree.Document) {
		root := ensureResourcesRoot(d)
		for _, op := range ops {
			setStyleEntry(root, op)
		}
	})
}

func applyValuesXML(path, kind string, mutate func(*etree.Document)) (string, bool, error) {
	doc, original, err := loadOrCreate(path)
	if err != nil {
		return path, false, fmt.Errorf("load %s: %w", kind, err)
	}
	mutate(doc)
	doc.Indent(4)
	out, err := doc.WriteToBytes()
	if err != nil {
		return path, false, fmt.Errorf("serialize %s: %w", kind, err)
	}
	if original != nil && bytes.Equal(out, original) {
		return path, false, nil
	}
	if err := EnsureDir(path); err != nil {
		return path, false, err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return path, false, fmt.Errorf("write %s: %w", kind, err)
	}
	return path, true, nil
}

func ensureResourcesRoot(doc *etree.Document) *etree.Element {
	root := doc.SelectElement("resources")
	if root != nil {
		return root
	}
	root = doc.CreateElement("resources")
	return root
}

func setValueEntry(root *etree.Element, tag, name, value string) {
	for _, el := range root.SelectElements(tag) {
		if attr := el.SelectAttr("name"); attr != nil && attr.Value == name {
			el.SetText(value)
			return
		}
	}
	entry := root.CreateElement(tag)
	entry.CreateAttr("name", name)
	entry.SetText(value)
}

func setStyleEntry(root *etree.Element, op *driftplugin.OpAndroidStyleSet) {
	var style *etree.Element
	for _, el := range root.SelectElements("style") {
		if attr := el.SelectAttr("name"); attr != nil && attr.Value == op.Name {
			style = el
			break
		}
	}
	if style == nil {
		style = root.CreateElement("style")
		style.CreateAttr("name", op.Name)
	} else {
		// Clear existing children so we don't accumulate orphan items.
		for _, c := range style.ChildElements() {
			style.RemoveChild(c)
		}
	}
	if op.Parent != "" {
		if attr := style.SelectAttr("parent"); attr != nil {
			attr.Value = op.Parent
		} else {
			style.CreateAttr("parent", op.Parent)
		}
	}
	for _, item := range op.Items {
		it := style.CreateElement("item")
		it.CreateAttr("name", item.Name)
		it.SetText(item.Value)
	}
}

// WriteAndroidDrawables writes raw bitmap files under drawableDir. Returns
// the paths that actually changed.
func WriteAndroidDrawables(drawableDir string, ops []*driftplugin.OpAndroidWriteDrawable) ([]string, error) {
	var changed []string
	if err := os.MkdirAll(drawableDir, 0o755); err != nil {
		return changed, fmt.Errorf("mkdir drawable: %w", err)
	}
	for _, op := range ops {
		dest := filepath.Join(drawableDir, op.Name+drawableExtension(op.Name))
		content, err := driftplugin.DecodeContent(op.Content)
		if err != nil {
			return changed, fmt.Errorf("decode drawable %s: %w", op.Name, err)
		}
		ch, err := writeIfDifferent(dest, content)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, dest)
		}
	}
	return changed, nil
}

// drawableExtension returns the appropriate extension based on the magic bytes
// but we keep it simple: default to .png. Authors who need .webp/.jpg pass
// a name that already includes the extension.
func drawableExtension(name string) string {
	if filepath.Ext(name) != "" {
		return ""
	}
	return ".png"
}

// WriteAndroidResourceXML writes arbitrary res/<relPath> XML files.
func WriteAndroidResourceXML(resRoot string, ops []*driftplugin.OpAndroidWriteResourceXML) ([]string, error) {
	var changed []string
	for _, op := range ops {
		dest := filepath.Join(resRoot, filepath.FromSlash(op.RelPath))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return changed, fmt.Errorf("mkdir %s: %w", dest, err)
		}
		ch, err := writeIfDifferent(dest, []byte(op.Content))
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, dest)
		}
	}
	return changed, nil
}

func loadXML(path string) (*etree.Document, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(data); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return doc, data, nil
}

func loadOrCreate(path string) (*etree.Document, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			doc := etree.NewDocument()
			doc.CreateProcInst("xml", `version="1.0" encoding="utf-8"`)
			return doc, nil, nil
		}
		return nil, nil, err
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(data); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return doc, data, nil
}

func writeIfDifferent(path string, content []byte) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return false, err
	}
	return true, nil
}
