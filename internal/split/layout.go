package split

// Rect represents a rectangular area in terminal coordinates.
type Rect struct {
	X, Y          int
	Width, Height int
}

// PaneLayout maps a pane ID to its computed rectangle.
type PaneLayout struct {
	PaneID string
	Rect   Rect
}

// Layout resolves a split tree into absolute pane rectangles given a total
// available width and height. The minimum pane dimension is 1 column / 1 row.
func Layout(root *Node, width, height int) []PaneLayout {
	if root == nil {
		return nil
	}
	return layoutRecursive(root, Rect{X: 0, Y: 0, Width: width, Height: height})
}

func layoutRecursive(node *Node, area Rect) []PaneLayout {
	if node.IsLeaf() {
		return []PaneLayout{{PaneID: node.Leaf.PaneID, Rect: area}}
	}

	if !node.IsSplit() {
		return nil
	}

	s := node.Split
	aRect, bRect := splitRect(area, s.Dir, s.Ratio)
	a := layoutRecursive(s.A, aRect)
	b := layoutRecursive(s.B, bRect)
	return append(a, b...)
}

// splitRect divides a rectangle according to direction and ratio.
// Allocates at least 1 column/row to each side.
func splitRect(area Rect, dir Direction, ratio float64) (Rect, Rect) {
	var a, b Rect

	switch dir {
	case Horizontal:
		// Split left/right.
		aWidth := int(float64(area.Width) * ratio)
		if aWidth < 1 {
			aWidth = 1
		}
		if aWidth >= area.Width {
			aWidth = area.Width - 1
		}
		bWidth := area.Width - aWidth

		a = Rect{X: area.X, Y: area.Y, Width: aWidth, Height: area.Height}
		b = Rect{X: area.X + aWidth, Y: area.Y, Width: bWidth, Height: area.Height}

	case Vertical:
		// Split top/bottom.
		aHeight := int(float64(area.Height) * ratio)
		if aHeight < 1 {
			aHeight = 1
		}
		if aHeight >= area.Height {
			aHeight = area.Height - 1
		}
		bHeight := area.Height - aHeight

		a = Rect{X: area.X, Y: area.Y, Width: area.Width, Height: aHeight}
		b = Rect{X: area.X, Y: area.Y + aHeight, Width: area.Width, Height: bHeight}
	}

	return a, b
}

// LayoutWithDividers is like Layout but reserves 1 column/row for split dividers.
// This ensures panes don't overlap with visual divider lines.
func LayoutWithDividers(root *Node, width, height int) []PaneLayout {
	if root == nil {
		return nil
	}
	return layoutWithDividersRecursive(root, Rect{X: 0, Y: 0, Width: width, Height: height})
}

func layoutWithDividersRecursive(node *Node, area Rect) []PaneLayout {
	if node.IsLeaf() {
		return []PaneLayout{{PaneID: node.Leaf.PaneID, Rect: area}}
	}

	if !node.IsSplit() {
		return nil
	}

	s := node.Split
	aRect, bRect := splitRectWithDivider(area, s.Dir, s.Ratio)
	a := layoutWithDividersRecursive(s.A, aRect)
	b := layoutWithDividersRecursive(s.B, bRect)
	return append(a, b...)
}

// splitGeometry computes the positions of child A, the divider, and child B.
// Single source of truth — used by both LayoutWithDividers and Dividers.
func splitGeometry(area Rect, dir Direction, ratio float64) (aRect, divider, bRect Rect) {
	const dividerSize = 1

	switch dir {
	case Horizontal:
		if area.Width < 3 {
			return Rect{X: area.X, Y: area.Y, Width: area.Width, Height: area.Height},
				Rect{}, // no divider
				Rect{X: area.X + area.Width, Y: area.Y, Width: 0, Height: area.Height}
		}
		usable := area.Width - dividerSize
		aWidth := int(float64(usable) * ratio)
		if aWidth < 1 {
			aWidth = 1
		}
		if aWidth >= usable {
			aWidth = usable - 1
		}
		bWidth := usable - aWidth
		aRect = Rect{X: area.X, Y: area.Y, Width: aWidth, Height: area.Height}
		divider = Rect{X: area.X + aWidth, Y: area.Y, Width: 1, Height: area.Height}
		bRect = Rect{X: area.X + aWidth + dividerSize, Y: area.Y, Width: bWidth, Height: area.Height}

	case Vertical:
		if area.Height < 3 {
			return Rect{X: area.X, Y: area.Y, Width: area.Width, Height: area.Height},
				Rect{},
				Rect{X: area.X, Y: area.Y + area.Height, Width: area.Width, Height: 0}
		}
		usable := area.Height - dividerSize
		aHeight := int(float64(usable) * ratio)
		if aHeight < 1 {
			aHeight = 1
		}
		if aHeight >= usable {
			aHeight = usable - 1
		}
		bHeight := usable - aHeight
		aRect = Rect{X: area.X, Y: area.Y, Width: area.Width, Height: aHeight}
		divider = Rect{X: area.X, Y: area.Y + aHeight, Width: area.Width, Height: 1}
		bRect = Rect{X: area.X, Y: area.Y + aHeight + dividerSize, Width: area.Width, Height: bHeight}
	}
	return
}

// splitRectWithDivider is a convenience wrapper returning just the two child rects.
func splitRectWithDivider(area Rect, dir Direction, ratio float64) (Rect, Rect) {
	a, _, b := splitGeometry(area, dir, ratio)
	return a, b
}

// Dividers returns the positions of all divider lines in the layout.
// Each divider is a Rect with either Width=1 (vertical divider) or Height=1 (horizontal).
func Dividers(root *Node, width, height int) []Rect {
	if root == nil {
		return nil
	}
	return dividersRecursive(root, Rect{X: 0, Y: 0, Width: width, Height: height})
}

func dividersRecursive(node *Node, area Rect) []Rect {
	if node.IsLeaf() {
		return nil
	}
	if !node.IsSplit() {
		return nil
	}

	s := node.Split
	aRect, divider, bRect := splitGeometry(area, s.Dir, s.Ratio)

	result := []Rect{divider}
	result = append(result, dividersRecursive(s.A, aRect)...)
	result = append(result, dividersRecursive(s.B, bRect)...)
	return result
}
