package mutate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/beevik/etree"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// ApplyAndroidManifest applies manifest ops to the file at path. Existing
// nodes are preserved (etree round-trips comments and unrelated siblings).
// Returns changed=true iff the file's bytes actually changed.
func ApplyAndroidManifest(
	path string,
	addPerms []*protocol.OpAndroidManifestAddPermission,
	addIntents []*protocol.OpAndroidManifestAddIntentFilter,
	setAttrs []*protocol.OpAndroidManifestSetActivityAttr,
	addMeta []*protocol.OpAndroidManifestAddMetaData,
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

// WriteAndroidColors writes Drift's plugin colours file at path from ops,
// replacing any previous content (the file is Drift-owned), or removes it
// when there are no ops so a dropped plugin's colours do not linger.
func WriteAndroidColors(path string, ops []*protocol.OpAndroidColorSet) (string, bool, error) {
	return writeValuesXML(path, "color", len(ops), func(root *etree.Element) {
		for _, op := range ops {
			setValueEntry(root, "color", op.Name, op.Value)
		}
	})
}

// WriteAndroidStrings is WriteAndroidColors for Drift's plugin strings file.
func WriteAndroidStrings(path string, ops []*protocol.OpAndroidStringSet) (string, bool, error) {
	return writeValuesXML(path, "string", len(ops), func(root *etree.Element) {
		for _, op := range ops {
			setValueEntry(root, "string", op.Name, op.Value)
		}
	})
}

// WriteAndroidStyles is WriteAndroidColors for Drift's plugin styles file.
func WriteAndroidStyles(path string, ops []*protocol.OpAndroidStyleSet) (string, bool, error) {
	return writeValuesXML(path, "style", len(ops), func(root *etree.Element) {
		for _, op := range ops {
			setStyleEntry(root, op)
		}
	})
}

func writeValuesXML(path, kind string, entries int, fill func(root *etree.Element)) (string, bool, error) {
	if entries == 0 {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return path, false, nil
		}
		if err != nil {
			return path, false, fmt.Errorf("remove %s: %w", kind, err)
		}
		return path, true, nil
	}
	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="utf-8"`)
	fill(doc.CreateElement("resources"))
	doc.Indent(4)
	out, err := doc.WriteToBytes()
	if err != nil {
		return path, false, fmt.Errorf("serialize %s: %w", kind, err)
	}
	ch, err := writeIfDifferent(path, out)
	if err != nil {
		return path, false, fmt.Errorf("write %s: %w", kind, err)
	}
	return path, ch, nil
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

func setStyleEntry(root *etree.Element, op *protocol.OpAndroidStyleSet) {
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
func WriteAndroidDrawables(drawableDir string, ops []*protocol.OpAndroidWriteDrawable) ([]string, error) {
	return writeEach(ops, func(op *protocol.OpAndroidWriteDrawable) (OwnedFile, error) {
		return DrawableFile(drawableDir, op)
	})
}

// WriteAndroidResourceXML writes arbitrary res/<relPath> XML files.
func WriteAndroidResourceXML(resRoot string, ops []*protocol.OpAndroidWriteResourceXML) ([]string, error) {
	return writeEach(ops, func(op *protocol.OpAndroidWriteResourceXML) (OwnedFile, error) {
		return ResourceXMLFile(resRoot, op), nil
	})
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
