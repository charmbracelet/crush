// Package tree allows you to build trees, as simple or complicated as you need.
//
// Define a tree with a root node, and children, set rendering properties (such
// as style, enumerators, etc...), and print it.
//
//	t := tree.New().
//		Child(
//			".git",
//			tree.Root("examples/").
//				Child(
//					tree.Root("list/").
//						Child("main.go").
//					tree.Root("table/").
//						Child("main.go").
//				).
//			tree.Root("list/").
//				Child("list.go", "list_test.go").
//			tree.New().
//				Root("table/").
//				Child("table.go", "table_test.go").
//			"align.go",
//			"align_test.go",
//			"join.go",
//			"join_test.go",
//		)
package tree

import (
	"fmt"
	"sync"

	"github.com/charmbracelet/lipgloss/v2"
)

// Node defines a node in a tree.
type Node interface {
	fmt.Stringer
	Value() string
	Children() Children
	Hidden() bool
	SetHidden(bool)
	SetValue(any)
}

// Leaf is a node without children.
type Leaf struct {
	value  string
	hidden bool
}

// NewLeaf returns a new Leaf.
func NewLeaf(value any, hidden bool) *Leaf {
	s := Leaf{}
	s.SetValue(value)
	s.SetHidden(hidden)
	return &s
}

// Children of a Leaf node are always empty.
func (Leaf) Children() Children {
	return NodeChildren(nil)
}

// Value returns the value of a Leaf node.
func (s Leaf) Value() string {
	return s.value
}

// SetValue sets the value of a Leaf node.
func (s *Leaf) SetValue(value any) {
	switch item := value.(type) {
	case Node, fmt.Stringer:
		s.value = item.(fmt.Stringer).String()
	case string, nil:
		s.value = item.(string)
	default:
		s.value = fmt.Sprintf("%v", item)
	}
}

// Hidden returns whether a Leaf node is hidden.
func (s Leaf) Hidden() bool {
	return s.hidden
}

// SetHidden hides a Leaf node.
func (s *Leaf) SetHidden(hidden bool) { s.hidden = hidden }

// String returns the string representation of a Leaf node.
func (s Leaf) String() string {
	return s.Value()
}

// Tree implements a Node.
type Tree struct {
	value    string
	hidden   bool
	offset   [2]int
	children Children

	r     *renderer
	ronce sync.Once
}

// Hidden returns whether a Tree node is hidden.
func (t *Tree) Hidden() bool {
	return t.hidden
}

// Hide sets whether to hide the Tree node. Use this when creating a new
// hidden Tree.
func (t *Tree) Hide(hide bool) *Tree {
	t.hidden = hide
	return t
}

// SetHidden hides a Tree node.
func (t *Tree) SetHidden(hidden bool) { t.Hide(hidden) }

// Offset sets the Tree children offsets.
func (t *Tree) Offset(start, end int) *Tree {
	if start > end {
		_start := start
		start = end
		end = _start
	}

	if start < 0 {
		start = 0
	}
	if end < 0 || end > t.children.Length() {
		end = t.children.Length()
	}

	t.offset[0] = start
	t.offset[1] = end
	return t
}

// Value returns the root name of this node.
func (t *Tree) Value() string {
	return t.value
}

// SetValue sets the value of a Tree node.
func (t *Tree) SetValue(value any) {
	t.Root(value)
}

// String returns the string representation of the Tree node.
func (t *Tree) String() string {
	return t.ensureRenderer().render(t, true, "")
}

// Child adds a child to this Tree.
//
// If a Child Tree is passed without a root, it will be parented to it's sibling
// child (auto-nesting).
//
//	tree.Root("Foo").Child("Bar", tree.New().Child("Baz"), "Qux")
//	tree.Root("Foo").Child(tree.Root("Bar").Child("Baz"), "Qux")
//
//	├── Foo
//	├── Bar
//	│   └── Baz
//	└── Qux
func (t *Tree) Child(children ...any) *Tree {
	for _, child := range children {
		switch item := child.(type) {
		case *Tree:
			newItem, rm := ensureParent(t.children, item)
			if rm >= 0 {
				t.children = t.children.(NodeChildren).Remove(rm)
			}
			t.children = t.children.(NodeChildren).Append(newItem)
		case Children:
			for i := range item.Length() {
				t.children = t.children.(NodeChildren).Append(item.At(i))
			}
		case Node:
			t.children = t.children.(NodeChildren).Append(item)
		case fmt.Stringer:
			s := Leaf{value: item.String()}
			t.children = t.children.(NodeChildren).Append(&s)
		case string:
			s := Leaf{value: item}
			t.children = t.children.(NodeChildren).Append(&s)
		case []any:
			return t.Child(item...)
		case []string:
			ss := make([]any, 0, len(item))
			for _, s := range item {
				ss = append(ss, s)
			}
			return t.Child(ss...)
		case nil:
			continue
		default:
			return t.Child(fmt.Sprintf("%v", item))
		}
	}
	return t
}

func ensureParent(nodes Children, item *Tree) (*Tree, int) {
	if item.Value() != "" || nodes.Length() == 0 {
		return item, -1
	}
	j := nodes.Length() - 1
	parent := nodes.At(j)
	switch parent := parent.(type) {
	case *Tree:
		for i := range item.Children().Length() {
			parent.Child(item.children.At(i))
		}
		return parent, j
	case *Leaf:
		item.value = parent.Value()
		return item, j
	}
	return item, -1
}

func (t *Tree) ensureRenderer() *renderer {
	t.ronce.Do(func() { t.r = newRenderer() })
	return t.r
}

// EnumeratorStyle sets a static style for all enumerators.
//
// Use EnumeratorStyleFunc to conditionally set styles based on the tree node.
func (t *Tree) EnumeratorStyle(style lipgloss.Style) *Tree {
	t.ensureRenderer().style.enumeratorFunc = func(Children, int) lipgloss.Style {
		return style
	}
	return t
}

// EnumeratorStyleFunc sets the enumeration style function. Use this function
// for conditional styling.
//
//	t := tree.New().
//		EnumeratorStyleFunc(func(_ tree.Children, i int) lipgloss.Style {
//		    if selected == i {
//		        return lipgloss.NewStyle().Foreground(hightlightColor)
//		    }
//		    return lipgloss.NewStyle().Foreground(dimColor)
//		})
func (t *Tree) EnumeratorStyleFunc(fn StyleFunc) *Tree {
	if fn == nil {
		fn = func(Children, int) lipgloss.Style { return lipgloss.NewStyle() }
	}
	t.ensureRenderer().style.enumeratorFunc = fn
	return t
}

// RootStyle sets a style for the root element.
func (t *Tree) RootStyle(style lipgloss.Style) *Tree {
	t.ensureRenderer().style.root = style
	return t
}

// ItemStyle sets a static style for all items.
//
// Use ItemStyleFunc to conditionally set styles based on the tree node.
func (t *Tree) ItemStyle(style lipgloss.Style) *Tree {
	t.ensureRenderer().style.itemFunc = func(Children, int) lipgloss.Style { return style }
	return t
}

// ItemStyleFunc sets the item style function. Use this for conditional styling.
// For example:
//
//	t := tree.New().
//		ItemStyleFunc(func(_ tree.Data, i int) lipgloss.Style {
//			if selected == i {
//				return lipgloss.NewStyle().Foreground(hightlightColor)
//			}
//			return lipgloss.NewStyle().Foreground(dimColor)
//		})
func (t *Tree) ItemStyleFunc(fn StyleFunc) *Tree {
	if fn == nil {
		fn = func(Children, int) lipgloss.Style { return lipgloss.NewStyle() }
	}
	t.ensureRenderer().style.itemFunc = fn
	return t
}

// Enumerator sets the enumerator implementation. This can be used to change the
// way the branches indicators look.  Lipgloss includes predefined enumerators
// for a classic or rounded tree. For example, you can have a rounded tree:
//
//	tree.New().
//		Enumerator(RoundedEnumerator)
func (t *Tree) Enumerator(enum Enumerator) *Tree {
	t.ensureRenderer().enumerator = enum
	return t
}

// Indenter sets the indenter implementation. This is used to change the way
// the tree is indented. The default indentor places a border connecting sibling
// elements and no border for the last child.
//
//	└── Foo
//	    └── Bar
//	        └── Baz
//	            └── Qux
//	                └── Quux
//
// You can define your own indenter.
//
//	func ArrowIndenter(children tree.Children, index int) string {
//		return "→ "
//	}
//
//	→ Foo
//	→ → Bar
//	→ → → Baz
//	→ → → → Qux
//	→ → → → → Quux
func (t *Tree) Indenter(indenter Indenter) *Tree {
	t.ensureRenderer().indenter = indenter
	return t
}

// Children returns the children of a node.
func (t *Tree) Children() Children {
	var data []Node
	for i := t.offset[0]; i < t.children.Length()-t.offset[1]; i++ {
		data = append(data, t.children.At(i))
	}
	return NodeChildren(data)
}

// Root returns a new tree with the root set.
//
//	tree.Root(root)
//
// It is a shorthand for:
//
//	tree.New().Root(root)
func Root(root any) *Tree {
	t := New()
	return t.Root(root)
}

// Root sets the root value of this tree.
func (t *Tree) Root(root any) *Tree {
	// root is a tree or string
	switch item := root.(type) {
	case *Tree:
		t.value = item.value
		t = t.Child(item.children)
	case Node, fmt.Stringer:
		t.value = item.(fmt.Stringer).String()
	case string, nil:
		t.value = item.(string)
	default:
		t.value = fmt.Sprintf("%v", item)
	}
	return t
}

// New returns a new tree.
func New() *Tree {
	return &Tree{
		children: NodeChildren(nil),
	}
}
