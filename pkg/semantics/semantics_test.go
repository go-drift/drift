//go:build android || darwin || ios
// +build android darwin ios

package semantics

import (
	"testing"

	"github.com/go-drift/drift/pkg/rendering"
)

func TestSemanticsNode_Creation(t *testing.T) {
	node := NewSemanticsNode()

	if node.ID <= 0 {
		t.Errorf("NewSemanticsNode should generate positive ID, got %d", node.ID)
	}

	if !node.IsDirty() {
		t.Error("New node should be dirty")
	}
}

func TestSemanticsNode_UniqueIDs(t *testing.T) {
	node1 := NewSemanticsNode()
	node2 := NewSemanticsNode()

	if node1.ID == node2.ID {
		t.Errorf("Nodes should have unique IDs, both have %d", node1.ID)
	}
}

func TestSemanticsNode_AddChild(t *testing.T) {
	parent := NewSemanticsNode()
	child := NewSemanticsNode()

	parent.AddChild(child)

	if len(parent.Children) != 1 {
		t.Errorf("Expected 1 child, got %d", len(parent.Children))
	}

	if child.Parent != parent {
		t.Error("Child's parent should be set")
	}
}

func TestSemanticsNode_RemoveChild(t *testing.T) {
	parent := NewSemanticsNode()
	child := NewSemanticsNode()

	parent.AddChild(child)
	parent.RemoveChild(child)

	if len(parent.Children) != 0 {
		t.Errorf("Expected 0 children after removal, got %d", len(parent.Children))
	}

	if child.Parent != nil {
		t.Error("Child's parent should be nil after removal")
	}
}

func TestSemanticsNode_ClearChildren(t *testing.T) {
	parent := NewSemanticsNode()
	child1 := NewSemanticsNode()
	child2 := NewSemanticsNode()

	parent.AddChild(child1)
	parent.AddChild(child2)
	parent.ClearChildren()

	if len(parent.Children) != 0 {
		t.Errorf("Expected 0 children after clear, got %d", len(parent.Children))
	}
}

func TestSemanticsNode_FindNodeByID(t *testing.T) {
	root := NewSemanticsNode()
	child1 := NewSemanticsNode()
	child2 := NewSemanticsNode()
	grandchild := NewSemanticsNode()

	root.AddChild(child1)
	root.AddChild(child2)
	child1.AddChild(grandchild)

	// Find root
	found := root.FindNodeByID(root.ID)
	if found != root {
		t.Error("Should find root node")
	}

	// Find grandchild
	found = root.FindNodeByID(grandchild.ID)
	if found != grandchild {
		t.Error("Should find grandchild node")
	}

	// Find non-existent
	found = root.FindNodeByID(-999)
	if found != nil {
		t.Error("Should return nil for non-existent ID")
	}
}

func TestSemanticsNode_Visit(t *testing.T) {
	root := NewSemanticsNode()
	child1 := NewSemanticsNode()
	child2 := NewSemanticsNode()

	root.AddChild(child1)
	root.AddChild(child2)

	var visited []int64
	root.Visit(func(node *SemanticsNode) bool {
		visited = append(visited, node.ID)
		return true
	})

	if len(visited) != 3 {
		t.Errorf("Expected 3 visited nodes, got %d", len(visited))
	}
}

func TestSemanticsNode_VisitStopEarly(t *testing.T) {
	root := NewSemanticsNode()
	child1 := NewSemanticsNode()
	child2 := NewSemanticsNode()

	root.AddChild(child1)
	root.AddChild(child2)

	count := 0
	root.Visit(func(node *SemanticsNode) bool {
		count++
		return count < 2 // Stop after 2 nodes
	})

	if count != 2 {
		t.Errorf("Visit should have stopped after 2 nodes, got %d", count)
	}
}

func TestSemanticsOwner_SetRoot(t *testing.T) {
	owner := NewSemanticsOwner()
	root := NewSemanticsNode()
	child := NewSemanticsNode()
	root.AddChild(child)

	owner.SetRoot(root)

	if owner.Root != root {
		t.Error("Root should be set")
	}

	// Verify index is built
	found := owner.FindNodeByID(child.ID)
	if found != child {
		t.Error("Index should be built on SetRoot")
	}
}

func TestSemanticsOwner_MarkDirty(t *testing.T) {
	owner := NewSemanticsOwner()
	node := NewSemanticsNode()
	node.ClearDirty()

	owner.MarkDirty(node)

	if !node.IsDirty() {
		t.Error("Node should be marked dirty")
	}

	if !owner.HasDirtyNodes() {
		t.Error("Owner should report dirty nodes")
	}
}

func TestSemanticsConfiguration_IsEmpty(t *testing.T) {
	empty := SemanticsConfiguration{}
	if !empty.IsEmpty() {
		t.Error("Default configuration should be empty")
	}

	withLabel := SemanticsConfiguration{
		Properties: SemanticsProperties{Label: "test"},
	}
	if withLabel.IsEmpty() {
		t.Error("Configuration with label should not be empty")
	}

	withBoundary := SemanticsConfiguration{
		IsSemanticBoundary: true,
	}
	if withBoundary.IsEmpty() {
		t.Error("Configuration with boundary should not be empty")
	}
}

func TestSemanticsConfiguration_Merge(t *testing.T) {
	config1 := SemanticsConfiguration{
		Properties: SemanticsProperties{
			Label: "original",
			Flags: SemanticsIsButton,
		},
	}

	config2 := SemanticsConfiguration{
		IsSemanticBoundary: true,
		Properties: SemanticsProperties{
			Value: "value",
			Flags: SemanticsIsEnabled,
		},
	}

	config1.Merge(config2)

	if !config1.IsSemanticBoundary {
		t.Error("Boundary should be merged")
	}

	if config1.Properties.Label != "original" {
		t.Error("Original label should be preserved")
	}

	if config1.Properties.Value != "value" {
		t.Error("Value should be merged")
	}

	// Flags should be OR'd
	if !config1.Properties.Flags.Has(SemanticsIsButton) || !config1.Properties.Flags.Has(SemanticsIsEnabled) {
		t.Error("Flags should be combined")
	}
}

func TestComputeDiff_Empty(t *testing.T) {
	diff := ComputeDiff(nil, nil)
	if !diff.IsEmpty() {
		t.Error("Diff of nil trees should be empty")
	}
}

func TestComputeDiff_Addition(t *testing.T) {
	newRoot := NewSemanticsNode()
	newRoot.Config.Properties.Label = "test"

	diff := ComputeDiff(nil, newRoot)

	if len(diff.Updates) != 1 {
		t.Errorf("Expected 1 update for new node, got %d", len(diff.Updates))
	}

	if len(diff.Removals) != 0 {
		t.Errorf("Expected 0 removals, got %d", len(diff.Removals))
	}
}

func TestComputeDiff_Removal(t *testing.T) {
	oldRoot := NewSemanticsNode()

	diff := ComputeDiff(oldRoot, nil)

	if len(diff.Removals) != 1 {
		t.Errorf("Expected 1 removal, got %d", len(diff.Removals))
	}

	if diff.Removals[0] != oldRoot.ID {
		t.Errorf("Expected removal ID %d, got %d", oldRoot.ID, diff.Removals[0])
	}
}

func TestComputeDiff_Modification(t *testing.T) {
	// Create old tree
	oldRoot := NewSemanticsNode()
	oldRoot.Config.Properties.Label = "old"

	// Create new tree with same ID but different content
	newRoot := &SemanticsNode{
		ID:    oldRoot.ID,
		dirty: true,
	}
	newRoot.Config.Properties.Label = "new"

	diff := ComputeDiff(oldRoot, newRoot)

	if len(diff.Updates) != 1 {
		t.Errorf("Expected 1 update for modified node, got %d", len(diff.Updates))
	}
}

func TestBuildSemanticsTree(t *testing.T) {
	config := SemanticsConfiguration{
		IsSemanticBoundary: true,
		Properties: SemanticsProperties{
			Label: "test button",
			Role:  SemanticsRoleButton,
		},
	}

	rect := rendering.RectFromLTWH(0, 0, 100, 50)
	node := BuildSemanticsTree(config, rect)

	if node == nil {
		t.Fatal("BuildSemanticsTree should return a node")
	}

	if node.Config.Properties.Label != "test button" {
		t.Errorf("Expected label 'test button', got %q", node.Config.Properties.Label)
	}

	if node.Rect != rect {
		t.Error("Rect should be set")
	}
}

func TestFlattenTree(t *testing.T) {
	root := NewSemanticsNode()
	child1 := NewSemanticsNode()
	child2 := NewSemanticsNode()
	grandchild := NewSemanticsNode()

	root.AddChild(child1)
	root.AddChild(child2)
	child1.AddChild(grandchild)

	nodes := FlattenTree(root)

	if len(nodes) != 4 {
		t.Errorf("Expected 4 nodes, got %d", len(nodes))
	}
}

func TestFlattenTree_Nil(t *testing.T) {
	nodes := FlattenTree(nil)
	if nodes != nil {
		t.Error("FlattenTree(nil) should return nil")
	}
}
