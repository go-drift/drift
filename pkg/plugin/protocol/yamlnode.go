package protocol

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ResolveYAML returns a standalone deep copy of n: aliases replaced by
// copies of their anchored nodes, merge keys (`<<: *base`) folded into their
// mappings, anchors dropped, and a document node unwrapped to its content.
// Scalars keep their source text, tag and style, so a config value reaches
// its typed field exactly as written ("1.10" stays "1.10", "0123" stays
// "0123"); decoding through `any` would turn those into 1.1 and 83.
//
// Config travels from drift.yaml to the bridge on its own, so an alias to
// an anchor outside the config block would dangle without this.
func ResolveYAML(n *yaml.Node) (*yaml.Node, error) {
	return resolveYAML(n, map[*yaml.Node]bool{})
}

// resolveYAML copies n. expanding holds the anchors whose aliases are being
// expanded on the current path, so a self-referencing alias fails instead
// of recursing forever.
func resolveYAML(n *yaml.Node, expanding map[*yaml.Node]bool) (*yaml.Node, error) {
	switch n.Kind {
	case 0, yaml.DocumentNode: // 0: unmarshaled from empty input
		if len(n.Content) == 0 {
			return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}, nil
		}
		return resolveYAML(n.Content[0], expanding)
	case yaml.AliasNode:
		if expanding[n.Alias] {
			return nil, fmt.Errorf("line %d: alias *%s refers to itself", n.Line, n.Value)
		}
		expanding[n.Alias] = true
		defer delete(expanding, n.Alias)
		return resolveYAML(n.Alias, expanding)
	case yaml.MappingNode:
		return resolveMapping(n, expanding)
	}
	cp := *n
	cp.Anchor = ""
	cp.Content = nil
	for _, c := range n.Content {
		rc, err := resolveYAML(c, expanding)
		if err != nil {
			return nil, err
		}
		cp.Content = append(cp.Content, rc)
	}
	return &cp, nil
}

// resolveMapping copies a mapping, folding in merge keys with YAML's merge
// rules: keys written in the mapping win over merged ones, and within a
// list of merged mappings the earlier mapping wins.
func resolveMapping(n *yaml.Node, expanding map[*yaml.Node]bool) (*yaml.Node, error) {
	cp := *n
	cp.Anchor = ""
	cp.Content = nil
	seen := map[string]bool{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if k := n.Content[i]; !isMergeKey(k) {
			seen[k.Value] = true
		}
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, err := resolveYAML(n.Content[i], expanding)
		if err != nil {
			return nil, err
		}
		v, err := resolveYAML(n.Content[i+1], expanding)
		if err != nil {
			return nil, err
		}
		if !isMergeKey(n.Content[i]) {
			cp.Content = append(cp.Content, k, v)
			continue
		}
		sources := []*yaml.Node{v}
		if v.Kind == yaml.SequenceNode {
			sources = v.Content
		}
		for _, src := range sources {
			if src.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("line %d: a merge key (<<) needs a mapping or a list of mappings", n.Content[i].Line)
			}
			for j := 0; j+1 < len(src.Content); j += 2 {
				if mk := src.Content[j]; !seen[mk.Value] {
					seen[mk.Value] = true
					cp.Content = append(cp.Content, mk, src.Content[j+1])
				}
			}
		}
	}
	return &cp, nil
}

func isMergeKey(k *yaml.Node) bool {
	return k.Kind == yaml.ScalarNode && k.ShortTag() == "!!merge"
}
