package tree

import "slices"

// Children is the interface that wraps the basic methods of a tree model.
type Children interface {
	// At returns the content item of the given index.
	At(index int) Node

	// Length returns the number of children in the tree.
	Length() int
}

// NodeChildren is the implementation of the Children interface with tree Nodes.
type NodeChildren []Node

// Append appends a child to the list of children.
func (n NodeChildren) Append(child Node) NodeChildren {
	n = append(n, child)
	return n
}

// Remove removes a child from the list at the given index.
func (n NodeChildren) Remove(index int) NodeChildren {
	if index < 0 || len(n) < index+1 {
		return n
	}
	n = slices.Delete(n, index, index+1)
	return n
}

// Length returns the number of children in the list.
func (n NodeChildren) Length() int {
	return len(n)
}

// At returns the child at the given index.
func (n NodeChildren) At(i int) Node {
	if i >= 0 && i < len(n) {
		return n[i]
	}
	return nil
}

// NewStringData returns a Data of strings.
func NewStringData(data ...string) Children {
	result := make([]Node, 0, len(data))
	for _, d := range data {
		s := Leaf{value: d}
		result = append(result, &s)
	}
	return NodeChildren(result)
}

var _ Children = NewFilter(nil)

// Filter applies a filter on some data. You could use this to create a new
// tree whose values all satisfy the condition provided in the Filter() function.
type Filter struct {
	data   Children
	filter func(index int) bool
}

// NewFilter initializes a new Filter.
func NewFilter(data Children) *Filter {
	return &Filter{data: data}
}

// At returns the item at the given index.
// The index is relative to the filtered results.
func (m *Filter) At(index int) Node {
	j := 0
	for i := range m.data.Length() {
		if m.filter(i) {
			if j == index {
				return m.data.At(i)
			}
			j++
		}
	}

	return nil
}

// Filter uses a filter function to set a condition that all the data must satisfy to be in the Tree.
func (m *Filter) Filter(f func(index int) bool) *Filter {
	m.filter = f
	return m
}

// Length returns the number of children in the tree.
func (m *Filter) Length() int {
	j := 0
	for i := range m.data.Length() {
		if m.filter(i) {
			j++
		}
	}
	return j
}
