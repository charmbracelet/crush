package split

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLayoutWithDividersHorizontal(t *testing.T) {
	t.Parallel()
	root := NewSplit(Horizontal, 0.5, NewLeaf("left"), NewLeaf("right"))
	layouts := LayoutWithDividers(root, 80, 24)
	require.Len(t, layouts, 2)

	left := layouts[0]
	right := layouts[1]

	// Widths should sum to total minus 1 (divider).
	require.Equal(t, 79, left.Rect.Width+right.Rect.Width)

	// Gap of 1 column between them (the divider).
	require.Equal(t, left.Rect.X+left.Rect.Width+1, right.Rect.X)
}

func TestLayoutWithDividersVertical(t *testing.T) {
	t.Parallel()
	root := NewSplit(Vertical, 0.5, NewLeaf("top"), NewLeaf("bottom"))
	layouts := LayoutWithDividers(root, 80, 24)
	require.Len(t, layouts, 2)

	top := layouts[0]
	bottom := layouts[1]

	// Heights should sum to total minus 1 (divider).
	require.Equal(t, 23, top.Rect.Height+bottom.Rect.Height)

	// Gap of 1 row between them.
	require.Equal(t, top.Rect.Y+top.Rect.Height+1, bottom.Rect.Y)
}

func TestDividersHorizontal(t *testing.T) {
	t.Parallel()
	root := NewSplit(Horizontal, 0.5, NewLeaf("left"), NewLeaf("right"))
	divs := Dividers(root, 80, 24)
	require.Len(t, divs, 1)

	div := divs[0]
	// Vertical divider line: 1 column wide, full height.
	require.Equal(t, 1, div.Width)
	require.Equal(t, 24, div.Height)
}

func TestDividersVertical(t *testing.T) {
	t.Parallel()
	root := NewSplit(Vertical, 0.5, NewLeaf("top"), NewLeaf("bottom"))
	divs := Dividers(root, 80, 24)
	require.Len(t, divs, 1)

	div := divs[0]
	// Horizontal divider line: full width, 1 row tall.
	require.Equal(t, 80, div.Width)
	require.Equal(t, 1, div.Height)
}

func TestDividersNestedSplits(t *testing.T) {
	t.Parallel()
	// left | (top-right / bottom-right)
	right := NewSplit(Vertical, 0.5, NewLeaf("tr"), NewLeaf("br"))
	root := NewSplit(Horizontal, 0.5, NewLeaf("left"), right)

	divs := Dividers(root, 80, 24)
	// One vertical divider between left and right, one horizontal inside right.
	require.Len(t, divs, 2)
}

func TestLayoutDeepTree(t *testing.T) {
	t.Parallel()
	// Four panes in a 2x2 grid:
	// p1 | p2
	// -------
	// p3 | p4
	topRow := NewSplit(Horizontal, 0.5, NewLeaf("p1"), NewLeaf("p2"))
	botRow := NewSplit(Horizontal, 0.5, NewLeaf("p3"), NewLeaf("p4"))
	root := NewSplit(Vertical, 0.5, topRow, botRow)

	layouts := LayoutWithDividers(root, 80, 24)
	require.Len(t, layouts, 4)

	ids := make([]string, len(layouts))
	for i, l := range layouts {
		ids[i] = l.PaneID
	}
	require.Equal(t, []string{"p1", "p2", "p3", "p4"}, ids)

	// All panes should have positive dimensions.
	for _, l := range layouts {
		require.Greater(t, l.Rect.Width, 0, "pane %s width", l.PaneID)
		require.Greater(t, l.Rect.Height, 0, "pane %s height", l.PaneID)
	}
}

func TestLayoutMinimumDimensions(t *testing.T) {
	t.Parallel()
	root := NewSplit(Horizontal, 0.5, NewLeaf("a"), NewLeaf("b"))

	// Even with tiny area, each pane gets at least 1 column.
	layouts := Layout(root, 2, 1)
	require.Len(t, layouts, 2)
	require.GreaterOrEqual(t, layouts[0].Rect.Width, 1)
	require.GreaterOrEqual(t, layouts[1].Rect.Width, 1)
}

func TestSplitGeometrySmallArea(t *testing.T) {
	t.Parallel()
	// Width < 3 should degrade gracefully (no divider, B gets zero width).
	area := Rect{X: 0, Y: 0, Width: 2, Height: 10}
	a, div, b := splitGeometry(area, Horizontal, 0.5)

	require.Equal(t, 2, a.Width)
	require.Equal(t, 0, div.Width)
	require.Equal(t, 0, b.Width)
}

func TestSplitGeometrySmallAreaVertical(t *testing.T) {
	t.Parallel()
	// Height < 3 should degrade gracefully.
	area := Rect{X: 0, Y: 0, Width: 80, Height: 2}
	a, div, b := splitGeometry(area, Vertical, 0.5)

	require.Equal(t, 2, a.Height)
	require.Equal(t, 0, div.Height)
	require.Equal(t, 0, b.Height)
}

func TestSplitGeometryExtremeRatio(t *testing.T) {
	t.Parallel()
	area := Rect{X: 0, Y: 0, Width: 80, Height: 24}

	// Near-zero ratio: A gets minimum 1 column.
	a, _, b := splitGeometry(area, Horizontal, 0.01)
	require.GreaterOrEqual(t, a.Width, 1)
	require.Greater(t, b.Width, 0)

	// Near-one ratio: B gets minimum 1 column.
	a2, _, b2 := splitGeometry(area, Horizontal, 0.99)
	require.Greater(t, a2.Width, 0)
	require.GreaterOrEqual(t, b2.Width, 1)
}

func TestDividerPositionConsistency(t *testing.T) {
	t.Parallel()
	// Verify divider sits exactly between the two pane rects.
	root := NewSplit(Horizontal, 0.5, NewLeaf("a"), NewLeaf("b"))
	layouts := LayoutWithDividers(root, 80, 24)
	divs := Dividers(root, 80, 24)

	require.Len(t, layouts, 2)
	require.Len(t, divs, 1)

	left := layouts[0]
	right := layouts[1]
	div := divs[0]

	// Divider should start right after left pane.
	require.Equal(t, left.Rect.X+left.Rect.Width, div.X)
	// Right pane should start right after divider.
	require.Equal(t, div.X+div.Width, right.Rect.X)
	// Total should equal 80.
	require.Equal(t, 80, left.Rect.Width+div.Width+right.Rect.Width)
}

func TestDividerPositionConsistencyVertical(t *testing.T) {
	t.Parallel()
	root := NewSplit(Vertical, 0.5, NewLeaf("a"), NewLeaf("b"))
	layouts := LayoutWithDividers(root, 80, 24)
	divs := Dividers(root, 80, 24)

	require.Len(t, layouts, 2)
	require.Len(t, divs, 1)

	top := layouts[0]
	bottom := layouts[1]
	div := divs[0]

	require.Equal(t, top.Rect.Y+top.Rect.Height, div.Y)
	require.Equal(t, div.Y+div.Height, bottom.Rect.Y)
	require.Equal(t, 24, top.Rect.Height+div.Height+bottom.Rect.Height)
}

func TestLayoutWithDividersNoOverlap(t *testing.T) {
	t.Parallel()
	// 3 panes: left | (top-right / bottom-right)
	right := NewSplit(Vertical, 0.5, NewLeaf("tr"), NewLeaf("br"))
	root := NewSplit(Horizontal, 0.5, NewLeaf("left"), right)

	layouts := LayoutWithDividers(root, 120, 40)
	divs := Dividers(root, 120, 40)

	// No pane should overlap any divider.
	for _, pl := range layouts {
		for _, d := range divs {
			overlapX := pl.Rect.X < d.X+d.Width && pl.Rect.X+pl.Rect.Width > d.X
			overlapY := pl.Rect.Y < d.Y+d.Height && pl.Rect.Y+pl.Rect.Height > d.Y
			if overlapX && overlapY {
				t.Errorf("pane %s overlaps divider at (%d,%d %dx%d)",
					pl.PaneID, d.X, d.Y, d.Width, d.Height)
			}
		}
	}

	// No two panes should overlap each other.
	for i := 0; i < len(layouts); i++ {
		for j := i + 1; j < len(layouts); j++ {
			a := layouts[i].Rect
			b := layouts[j].Rect
			overlapX := a.X < b.X+b.Width && a.X+a.Width > b.X
			overlapY := a.Y < b.Y+b.Height && a.Y+a.Height > b.Y
			if overlapX && overlapY {
				t.Errorf("pane %s overlaps pane %s", layouts[i].PaneID, layouts[j].PaneID)
			}
		}
	}
}

func TestLayoutVariousRatios(t *testing.T) {
	t.Parallel()
	ratios := []float64{0.1, 0.25, 0.33, 0.5, 0.66, 0.75, 0.9}
	for _, ratio := range ratios {
		root := NewSplit(Horizontal, ratio, NewLeaf("a"), NewLeaf("b"))
		layouts := LayoutWithDividers(root, 100, 50)
		require.Len(t, layouts, 2, "ratio %f", ratio)

		// Both panes must have positive dimensions.
		require.Greater(t, layouts[0].Rect.Width, 0, "left pane width at ratio %f", ratio)
		require.Greater(t, layouts[1].Rect.Width, 0, "right pane width at ratio %f", ratio)

		divs := Dividers(root, 100, 50)
		require.Len(t, divs, 1)
		total := layouts[0].Rect.Width + divs[0].Width + layouts[1].Rect.Width
		require.Equal(t, 100, total, "total width at ratio %f", ratio)
	}
}
