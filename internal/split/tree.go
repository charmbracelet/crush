// Package split implements a binary tree layout engine for terminal split panes.
// Each leaf is a pane, each internal node is a horizontal or vertical split
// with a configurable ratio.
package split

import "fmt"

// Direction defines the split orientation.
type Direction int

const (
	Horizontal Direction = iota // split left/right
	Vertical                    // split top/bottom
)

func (d Direction) String() string {
	if d == Horizontal {
		return "H"
	}
	return "V"
}

// Node is a node in the split tree — either a Split (internal) or a Leaf (terminal pane).
type Node struct {
	// If Leaf is non-nil, this is a leaf node (a pane).
	Leaf *Leaf

	// If Split is non-nil, this is an internal split node.
	Split *SplitNode
}

// IsLeaf returns true if this node is a leaf (pane).
func (n *Node) IsLeaf() bool {
	return n.Leaf != nil
}

// IsSplit returns true if this node is an internal split.
func (n *Node) IsSplit() bool {
	return n.Split != nil
}

// Leaf represents a terminal pane (the actual content area).
type Leaf struct {
	// PaneID uniquely identifies this pane.
	PaneID string
}

// SplitNode represents an internal split dividing space between two children.
type SplitNode struct {
	// Dir is the split direction.
	Dir Direction

	// Ratio is the fraction of space allocated to child A (0.0-1.0).
	Ratio float64

	// A is the first child (left or top).
	A *Node

	// B is the second child (right or bottom).
	B *Node
}

// NewLeaf creates a new leaf node with the given pane ID.
func NewLeaf(paneID string) *Node {
	return &Node{Leaf: &Leaf{PaneID: paneID}}
}

// NewSplit creates a new split node.
func NewSplit(dir Direction, ratio float64, a, b *Node) *Node {
	if ratio <= 0 || ratio >= 1 {
		ratio = 0.5
	}
	return &Node{Split: &SplitNode{Dir: dir, Ratio: ratio, A: a, B: b}}
}

// SplitLeaf finds the leaf with the given paneID and splits it into two panes.
// The original pane becomes child A, and a new pane with newPaneID becomes child B.
// Returns an error if the pane is not found.
func SplitLeaf(root *Node, paneID string, dir Direction, newPaneID string) error {
	return splitLeafRecursive(root, paneID, dir, newPaneID)
}

func splitLeafRecursive(node *Node, paneID string, dir Direction, newPaneID string) error {
	if node.IsLeaf() {
		if node.Leaf.PaneID == paneID {
			// Convert this leaf into a split with original + new pane.
			original := &Leaf{PaneID: paneID}
			newLeaf := &Leaf{PaneID: newPaneID}
			node.Leaf = nil
			node.Split = &SplitNode{
				Dir:   dir,
				Ratio: 0.5,
				A:     &Node{Leaf: original},
				B:     &Node{Leaf: newLeaf},
			}
			return nil
		}
		return fmt.Errorf("pane %q not found", paneID)
	}

	if node.IsSplit() {
		if err := splitLeafRecursive(node.Split.A, paneID, dir, newPaneID); err == nil {
			return nil
		}
		return splitLeafRecursive(node.Split.B, paneID, dir, newPaneID)
	}

	return fmt.Errorf("pane %q not found", paneID)
}

// RemoveLeaf removes a pane from the tree by its ID. The parent split collapses,
// and the sibling takes over the parent's space. Returns an error if the pane
// is not found or is the only pane (root leaf).
func RemoveLeaf(root *Node, paneID string) error {
	if root.IsLeaf() {
		if root.Leaf.PaneID == paneID {
			return fmt.Errorf("cannot remove the only pane")
		}
		return fmt.Errorf("pane %q not found", paneID)
	}
	return removeLeafRecursive(root, paneID)
}

func removeLeafRecursive(node *Node, paneID string) error {
	if !node.IsSplit() {
		return fmt.Errorf("pane %q not found", paneID)
	}

	s := node.Split

	// Check if A is the target leaf.
	if s.A.IsLeaf() && s.A.Leaf.PaneID == paneID {
		// Promote B to this node's position.
		*node = *s.B
		return nil
	}

	// Check if B is the target leaf.
	if s.B.IsLeaf() && s.B.Leaf.PaneID == paneID {
		// Promote A to this node's position.
		*node = *s.A
		return nil
	}

	// Recurse into children.
	if err := removeLeafRecursive(s.A, paneID); err == nil {
		return nil
	}
	return removeLeafRecursive(s.B, paneID)
}

// FindLeaf returns the leaf node with the given paneID, or nil.
func FindLeaf(root *Node, paneID string) *Leaf {
	if root.IsLeaf() {
		if root.Leaf.PaneID == paneID {
			return root.Leaf
		}
		return nil
	}
	if root.IsSplit() {
		if l := FindLeaf(root.Split.A, paneID); l != nil {
			return l
		}
		return FindLeaf(root.Split.B, paneID)
	}
	return nil
}

// AllLeaves returns all leaf pane IDs in depth-first order.
func AllLeaves(root *Node) []string {
	if root.IsLeaf() {
		return []string{root.Leaf.PaneID}
	}
	if root.IsSplit() {
		a := AllLeaves(root.Split.A)
		b := AllLeaves(root.Split.B)
		return append(a, b...)
	}
	return nil
}

// LeafCount returns the number of leaf panes in the tree. Zero allocations.
func LeafCount(root *Node) int {
	if root == nil {
		return 0
	}
	if root.IsLeaf() {
		return 1
	}
	if root.IsSplit() {
		return LeafCount(root.Split.A) + LeafCount(root.Split.B)
	}
	return 0
}

// SetRatio sets the split ratio for the split that contains the given paneID
// as a direct child. Returns an error if not found.
func SetRatio(root *Node, paneID string, ratio float64) error {
	if ratio <= 0 || ratio >= 1 {
		return fmt.Errorf("ratio must be between 0 and 1 exclusive, got %f", ratio)
	}
	return setRatioRecursive(root, paneID, ratio)
}

func setRatioRecursive(node *Node, paneID string, ratio float64) error {
	if !node.IsSplit() {
		return fmt.Errorf("pane %q not found in any split", paneID)
	}
	s := node.Split
	if (s.A.IsLeaf() && s.A.Leaf.PaneID == paneID) ||
		(s.B.IsLeaf() && s.B.Leaf.PaneID == paneID) {
		s.Ratio = ratio
		return nil
	}
	if err := setRatioRecursive(s.A, paneID, ratio); err == nil {
		return nil
	}
	return setRatioRecursive(s.B, paneID, ratio)
}
