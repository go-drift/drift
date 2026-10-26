package mutate

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/beevik/etree"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// ManifestOps are the AndroidManifest.xml ops of one build, by type.
type ManifestOps struct {
	Permissions   []*protocol.OpAndroidManifestAddPermission
	IntentFilters []*protocol.OpAndroidManifestAddIntentFilter
	ActivityAttrs []*protocol.OpAndroidManifestSetActivityAttr
	MetaData      []*protocol.OpAndroidManifestAddMetaData
	Components    []*protocol.OpAndroidManifestAddComponent
}

// Empty reports whether there is nothing to apply.
func (m ManifestOps) Empty() bool {
	return len(m.Permissions)+len(m.IntentFilters)+len(m.ActivityAttrs)+len(m.MetaData)+len(m.Components) == 0
}

// ApplyAndroidManifest plans manifest ops' edits to the file at path into
// cs. Existing nodes are preserved (etree round-trips comments and
// unrelated siblings). Returns changed=true iff the file's bytes change.
func ApplyAndroidManifest(cs *Changeset, path string, ops ManifestOps) (bool, error) {
	doc, err := loadXML(cs, path)
	if err != nil {
		return false, fmt.Errorf("read AndroidManifest: %w", err)
	}
	manifest := doc.SelectElement("manifest")
	if manifest == nil {
		return false, fmt.Errorf("AndroidManifest.xml has no <manifest> root")
	}

	for _, op := range ops.Permissions {
		ensurePermission(manifest, op.Name, op.MaxSDK)
	}

	app := manifest.SelectElement("application")
	if app == nil && len(ops.ActivityAttrs)+len(ops.IntentFilters)+len(ops.MetaData)+len(ops.Components) > 0 {
		return false, fmt.Errorf("AndroidManifest.xml has no <application>; cannot apply activity, intent, meta-data, service or receiver ops")
	}

	for _, op := range ops.ActivityAttrs {
		act := findActivity(app, op.Activity)
		if act == nil {
			return false, fmt.Errorf("AndroidManifest.xml: activity %q not found for SetActivityAttr", op.Activity)
		}
		setNSAttr(act, op.Attr, op.Value)
	}

	for _, op := range ops.IntentFilters {
		act := findActivity(app, op.Activity)
		if act == nil {
			return false, fmt.Errorf("AndroidManifest.xml: activity %q not found for AddIntentFilter", op.Activity)
		}
		if err := appendIntentFilter(act, op.XML); err != nil {
			return false, err
		}
	}

	for _, op := range ops.MetaData {
		parent := app
		if rest, ok := strings.CutPrefix(op.Parent, "activity:"); ok {
			parent = findActivity(app, rest)
			if parent == nil {
				return false, fmt.Errorf("AndroidManifest.xml: activity %q not found for AddMetaData", op.Parent)
			}
		}
		ensureMetaData(parent, op.Name, op.Value)
	}

	// Grouped by kind, services first, so the manifest's order does not
	// depend on the order plugins record them in.
	for _, tag := range protocol.ManifestComponents {
		for _, op := range ops.Components {
			if op.Tag() != tag {
				continue
			}
			if err := putComponent(app, tag, op.Name(), op.XML, op.PluginPackage()); err != nil {
				return false, err
			}
		}
	}

	doc.Indent(4)
	out, err := doc.WriteToBytes()
	if err != nil {
		return false, fmt.Errorf("serialize AndroidManifest: %w", err)
	}
	return cs.WriteFile(path, out)
}

// ensurePermission declares name, up to API level maxSDK when it is set.
// An existing declaration is kept as is.
func ensurePermission(manifest *etree.Element, name string, maxSDK int) {
	for _, el := range manifest.SelectElements("uses-permission") {
		if attr := el.SelectAttr("android:name"); attr != nil && attr.Value == name {
			return
		}
	}
	perm := etree.NewElement("uses-permission")
	perm.CreateAttr("android:name", name)
	if maxSDK > 0 {
		perm.CreateAttr("android:maxSdkVersion", strconv.Itoa(maxSDK))
	}
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

// putComponent adds a <tag> element (service, receiver) from pkg's xml,
// replacing one with the same android:name in place (the op owns that
// component), so reruns converge.
func putComponent(app *etree.Element, tag, name, xml, pkg string) error {
	sub := etree.NewDocument()
	if err := sub.ReadFromString(xml); err != nil {
		return fmt.Errorf("parse %s from %s: %w", tag, pkg, err)
	}
	component := sub.Root().Copy()
	for _, el := range app.SelectElements(tag) {
		if attr := el.SelectAttr("android:name"); attr != nil && attr.Value == name {
			app.InsertChildAt(el.Index(), component)
			app.RemoveChild(el)
			return nil
		}
	}
	app.AddChild(component)
	return nil
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

// WriteAndroidColors plans Drift's plugin colours file at path from ops,
// replacing any previous content (the file is Drift-owned), or removes it
// when there are no ops so a dropped plugin's colours do not linger.
// Values are Drift hex colours (alpha last), written in Android's format
// (alpha first).
func WriteAndroidColors(cs *Changeset, path string, ops []*protocol.OpAndroidColorSet) (string, bool, error) {
	values := make([]string, len(ops))
	for i, op := range ops {
		c, err := protocol.ParseColor(op.Value)
		if err != nil {
			return "", false, fmt.Errorf("color %s: %w", op.Name, err)
		}
		values[i] = c.AndroidHex()
	}
	return writeValuesXML(cs, path, "color", len(ops), func(root *etree.Element) {
		for i, op := range ops {
			setValueEntry(root, "color", op.Name, values[i])
		}
	})
}

// WriteAndroidStrings is WriteAndroidColors for Drift's plugin strings file.
// Values are literal text, written in Android's string resource syntax (see
// androidStringText) so each compiled resource is exactly its value. A
// value containing % is marked formatted="false": it is not a format
// string, and aapt2 rejects some literal text, such as "%s and %d", as an
// unnumbered format otherwise.
func WriteAndroidStrings(cs *Changeset, path string, ops []*protocol.OpAndroidStringSet) (string, bool, error) {
	return writeValuesXML(cs, path, "string", len(ops), func(root *etree.Element) {
		for _, op := range ops {
			el := setValueEntry(root, "string", op.Name, androidStringText(op.Value))
			if strings.Contains(op.Value, "%") {
				el.CreateAttr("formatted", "false")
			} else {
				el.RemoveAttr("formatted")
			}
		}
	})
}

// androidStringText returns the text of a <string> resource (before XML
// escaping, which etree does) that aapt2 compiles to exactly value.
// Android string syntax goes beyond XML: a backslash starts an escape,
// double quotes delimit, an unescaped apostrophe is an error, a leading @
// or ? makes a reference, and outside quotes ASCII whitespace runs collapse
// to one space and leading or trailing whitespace is trimmed. So
// backslashes, quotes and a leading @ or ? are backslash-escaped, newline
// and tab become \n and \t, other control characters \uXXXX, and a value
// whose spaces would otherwise change is wrapped in double quotes.
func androidStringText(value string) string {
	var b strings.Builder
	for i, r := range value {
		switch {
		case r == '\\' || r == '"' || r == '\'' || (i == 0 && (r == '@' || r == '?')):
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	if strings.HasPrefix(value, " ") || strings.HasSuffix(value, " ") || strings.Contains(value, "  ") {
		return `"` + b.String() + `"`
	}
	return b.String()
}

// WriteAndroidStyles is WriteAndroidColors for Drift's plugin styles file.
func WriteAndroidStyles(cs *Changeset, path string, ops []*protocol.OpAndroidStyleSet) (string, bool, error) {
	return writeValuesXML(cs, path, "style", len(ops), func(root *etree.Element) {
		for _, op := range ops {
			setStyleEntry(root, op)
		}
	})
}

func writeValuesXML(cs *Changeset, path, kind string, entries int, fill func(root *etree.Element)) (string, bool, error) {
	if entries == 0 {
		existed, err := cs.Remove(path)
		if err != nil {
			return path, false, fmt.Errorf("remove %s: %w", kind, err)
		}
		return path, existed, nil
	}
	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="utf-8"`)
	fill(doc.CreateElement("resources"))
	doc.Indent(4)
	out, err := doc.WriteToBytes()
	if err != nil {
		return path, false, fmt.Errorf("serialize %s: %w", kind, err)
	}
	ch, err := cs.WriteFile(path, out)
	if err != nil {
		return path, false, fmt.Errorf("%s: %w", kind, err)
	}
	return path, ch, nil
}

// setValueEntry sets the text of root's <tag name="name"> entry, creating
// it if absent, and returns the entry.
func setValueEntry(root *etree.Element, tag, name, value string) *etree.Element {
	for _, el := range root.SelectElements(tag) {
		if attr := el.SelectAttr("name"); attr != nil && attr.Value == name {
			el.SetText(value)
			return el
		}
	}
	entry := root.CreateElement(tag)
	entry.CreateAttr("name", name)
	entry.SetText(value)
	return entry
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

func loadXML(cs *Changeset, path string) (*etree.Document, error) {
	data, err := cs.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(data); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return doc, nil
}
