package tree

// Enumerator enumerates a tree. Typically, this is used to draw the branches
// for the tree nodes and is different for the last child.
//
// For example, the default enumerator would be:
//
//	func TreeEnumerator(children Children, index int) string {
//		if children.Length()-1 == index {
//			return "└──"
//		}
//
//		return "├──"
//	}
type Enumerator func(children Children, index int) string

// DefaultEnumerator enumerates a tree.
//
// ├── Foo
// ├── Bar
// ├── Baz
// └── Qux.
func DefaultEnumerator(children Children, index int) string {
	if children.Length()-1 == index {
		return "└──"
	}
	return "├──"
}

// RoundedEnumerator enumerates a tree with rounded edges.
//
// ├── Foo
// ├── Bar
// ├── Baz
// ╰── Qux.
func RoundedEnumerator(children Children, index int) string {
	if children.Length()-1 == index {
		return "╰──"
	}
	return "├──"
}

// Indenter indents the children of a tree.
//
// Indenters allow for displaying nested tree items with connecting borders
// to sibling nodes.
//
// For example, the default indenter would be:
//
//	func TreeIndenter(children Children, index int) string {
//		if children.Length()-1 == index {
//			return "│  "
//		}
//
//		return "   "
//	}
type Indenter func(children Children, index int) string

// DefaultIndenter indents a tree for nested trees and multiline content.
//
// ├── Foo
// ├── Bar
// │   ├── Qux
// │   ├── Quux
// │   │   ├── Foo
// │   │   └── Bar
// │   └── Quuux
// └── Baz.
func DefaultIndenter(children Children, index int) string {
	if children.Length()-1 == index {
		return "   "
	}
	return "│  "
}
