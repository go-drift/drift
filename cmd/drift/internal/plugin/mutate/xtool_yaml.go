package mutate

import (
	"bytes"
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// SetXtoolResources makes the top-level `resources:` list of xtool.yml
// exactly the given paths (relative to the xtool project root). xtool copies
// each listed file into the root of the app bundle, where Bundle.main and
// UIImage(named:) find it. SwiftPM target resources would instead land in a
// separate Runner_Runner.bundle that neither can see, which is why plugin
// bundle resources go through xtool.yml and not Package.swift.
//
// The key is owned by Drift: the managed xtool dir is re-scaffolded on every
// build and xtool projects cannot be ejected, so there is no user-authored
// list to merge with. Setting (not appending) keeps reruns convergent. An
// empty list removes the key.
//
// FUTURE(xtool#219): once xtool compiles asset catalogs, this mutator also
// sets `assetCatalogs:` (one catalog per product) to the Drift-owned
// Assets.xcassets. https://github.com/xtool-org/xtool/pull/219
func SetXtoolResources(xtoolYmlPath string, resources []string) (bool, error) {
	data, err := os.ReadFile(xtoolYmlPath)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", xtoolYmlPath, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false, fmt.Errorf("parse %s: %w", xtoolYmlPath, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return false, fmt.Errorf("%s: expected a top-level mapping", xtoolYmlPath)
	}
	root := doc.Content[0]

	sorted := append([]string(nil), resources...)
	sort.Strings(sorted)
	idx := mappingKeyIndex(root, "resources")
	switch {
	case len(sorted) == 0 && idx < 0:
		// Nothing to do; leave the scaffolded file byte-for-byte untouched.
		return false, nil
	case len(sorted) == 0:
		root.Content = append(root.Content[:idx], root.Content[idx+2:]...)
	default:
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, r := range sorted {
			seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: r})
		}
		if idx < 0 {
			root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "resources"}, seq)
		} else {
			root.Content[idx+1] = seq
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return false, fmt.Errorf("encode %s: %w", xtoolYmlPath, err)
	}
	if err := enc.Close(); err != nil {
		return false, fmt.Errorf("encode %s: %w", xtoolYmlPath, err)
	}
	return writeIfDifferent(xtoolYmlPath, buf.Bytes())
}

// mappingKeyIndex returns the index of key's key node in mapping m's
// Content, or -1.
func mappingKeyIndex(m *yaml.Node, key string) int {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return i
		}
	}
	return -1
}
