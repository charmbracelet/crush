package split

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewLeaf(t *testing.T) {
	t.Parallel()
	n := NewLeaf("p1")
	require.True(t, n.IsLeaf())
	require.False(t, n.IsSplit())
	require.Equal(t, "p1", n.Leaf.PaneID)
}

func TestNewSplit(t *testing.T) {
	t.Parallel()
	a := NewLeaf("p1")
	b := NewLeaf("p2")
	s := NewSplit(Horizontal, 0.5, a, b)
	require.True(t, s.IsSplit())
	require.False(t, s.IsLeaf())
	require.Equal(t, Horizontal, s.Split.Dir)
	require.InDelta(t, 0.5, s.Split.Ratio, 0.001)
}

func TestNewSplitClampsBadRatio(t *testing.T) {
	t.Parallel()
	s := NewSplit(Vertical, 0.0, NewLeaf("a"), NewLeaf("b"))
	require.InDelta(t, 0.5, s.Split.Ratio, 0.001)

	s2 := NewSplit(Vertical, 1.0, NewLeaf("a"), NewLeaf("b"))
	require.InDelta(t, 0.5, s2.Split.Ratio, 0.001)
}

func TestSplitLeaf(t *testing.T) {
	t.Parallel()
	root := NewLeaf("p1")

	err := SplitLeaf(root, "p1", Horizontal, "p2")
	require.NoError(t, err)
	require.True(t, root.IsSplit())
	require.Equal(t, Horizontal, root.Split.Dir)
	require.Equal(t, "p1", root.Split.A.Leaf.PaneID)
	require.Equal(t, "p2", root.Split.B.Leaf.PaneID)
}

func TestSplitLeafDeep(t *testing.T) {
	t.Parallel()
	root := NewLeaf("p1")

	require.NoError(t, SplitLeaf(root, "p1", Horizontal, "p2"))
	require.NoError(t, SplitLeaf(root, "p2", Vertical, "p3"))

	leaves := AllLeaves(root)
	require.Equal(t, []string{"p1", "p2", "p3"}, leaves)
	require.Equal(t, 3, LeafCount(root))
}

func TestSplitLeafNotFound(t *testing.T) {
	t.Parallel()
	root := NewLeaf("p1")
	err := SplitLeaf(root, "nonexistent", Horizontal, "p2")
	require.Error(t, err)
}

func TestRemoveLeaf(t *testing.T) {
	t.Parallel()
	root := NewSplit(Horizontal, 0.5, NewLeaf("p1"), NewLeaf("p2"))

	err := RemoveLeaf(root, "p1")
	require.NoError(t, err)
	require.True(t, root.IsLeaf())
	require.Equal(t, "p2", root.Leaf.PaneID)
}

func TestRemoveLeafFromB(t *testing.T) {
	t.Parallel()
	root := NewSplit(Horizontal, 0.5, NewLeaf("p1"), NewLeaf("p2"))

	err := RemoveLeaf(root, "p2")
	require.NoError(t, err)
	require.True(t, root.IsLeaf())
	require.Equal(t, "p1", root.Leaf.PaneID)
}

func TestRemoveLeafDeep(t *testing.T) {
	t.Parallel()
	// Build: H(p1, V(p2, p3))
	root := NewSplit(Horizontal, 0.5,
		NewLeaf("p1"),
		NewSplit(Vertical, 0.5, NewLeaf("p2"), NewLeaf("p3")),
	)

	err := RemoveLeaf(root, "p2")
	require.NoError(t, err)
	// Should collapse to H(p1, p3)
	require.True(t, root.IsSplit())
	leaves := AllLeaves(root)
	require.Equal(t, []string{"p1", "p3"}, leaves)
}

func TestRemoveOnlyPane(t *testing.T) {
	t.Parallel()
	root := NewLeaf("p1")
	err := RemoveLeaf(root, "p1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot remove the only pane")
}

func TestRemoveLeafNotFound(t *testing.T) {
	t.Parallel()
	root := NewSplit(Horizontal, 0.5, NewLeaf("p1"), NewLeaf("p2"))
	err := RemoveLeaf(root, "nonexistent")
	require.Error(t, err)
}

func TestFindLeaf(t *testing.T) {
	t.Parallel()
	root := NewSplit(Horizontal, 0.5,
		NewLeaf("p1"),
		NewSplit(Vertical, 0.5, NewLeaf("p2"), NewLeaf("p3")),
	)

	require.NotNil(t, FindLeaf(root, "p1"))
	require.NotNil(t, FindLeaf(root, "p2"))
	require.NotNil(t, FindLeaf(root, "p3"))
	require.Nil(t, FindLeaf(root, "nonexistent"))
}

func TestAllLeaves(t *testing.T) {
	t.Parallel()
	root := NewSplit(Vertical, 0.5,
		NewSplit(Horizontal, 0.5, NewLeaf("a"), NewLeaf("b")),
		NewSplit(Horizontal, 0.5, NewLeaf("c"), NewLeaf("d")),
	)
	require.Equal(t, []string{"a", "b", "c", "d"}, AllLeaves(root))
}

func TestSetRatio(t *testing.T) {
	t.Parallel()
	root := NewSplit(Horizontal, 0.5, NewLeaf("p1"), NewLeaf("p2"))

	require.NoError(t, SetRatio(root, "p1", 0.3))
	require.InDelta(t, 0.3, root.Split.Ratio, 0.001)

	require.NoError(t, SetRatio(root, "p2", 0.7))
	require.InDelta(t, 0.7, root.Split.Ratio, 0.001)
}

func TestSetRatioInvalid(t *testing.T) {
	t.Parallel()
	root := NewSplit(Horizontal, 0.5, NewLeaf("p1"), NewLeaf("p2"))
	require.Error(t, SetRatio(root, "p1", 0.0))
	require.Error(t, SetRatio(root, "p1", 1.0))
	require.Error(t, SetRatio(root, "p1", -0.5))
}

func TestLayout(t *testing.T) {
	t.Parallel()
	root := NewSplit(Horizontal, 0.5, NewLeaf("p1"), NewLeaf("p2"))

	layouts := Layout(root, 100, 50)
	require.Len(t, layouts, 2)
	require.Equal(t, "p1", layouts[0].PaneID)
	require.Equal(t, "p2", layouts[1].PaneID)

	// p1 gets left half, p2 gets right half.
	require.Equal(t, 0, layouts[0].Rect.X)
	require.Equal(t, 50, layouts[0].Rect.Width)
	require.Equal(t, 50, layouts[1].Rect.X)
	require.Equal(t, 50, layouts[1].Rect.Width)
}

func TestLayoutVertical(t *testing.T) {
	t.Parallel()
	root := NewSplit(Vertical, 0.5, NewLeaf("top"), NewLeaf("bottom"))

	layouts := Layout(root, 80, 40)
	require.Len(t, layouts, 2)
	require.Equal(t, 0, layouts[0].Rect.Y)
	require.Equal(t, 20, layouts[0].Rect.Height)
	require.Equal(t, 20, layouts[1].Rect.Y)
	require.Equal(t, 20, layouts[1].Rect.Height)
}

func TestLayoutWithDividers(t *testing.T) {
	t.Parallel()
	root := NewSplit(Horizontal, 0.5, NewLeaf("p1"), NewLeaf("p2"))

	layouts := LayoutWithDividers(root, 101, 50)
	require.Len(t, layouts, 2)

	// Total width minus 1 divider = 100 usable, split 50/50.
	require.Equal(t, 50, layouts[0].Rect.Width)
	require.Equal(t, 50, layouts[1].Rect.Width)
	// p2 starts after p1 + 1 divider.
	require.Equal(t, 51, layouts[1].Rect.X)
}

func TestLayoutSingleLeaf(t *testing.T) {
	t.Parallel()
	root := NewLeaf("only")
	layouts := Layout(root, 80, 40)
	require.Len(t, layouts, 1)
	require.Equal(t, Rect{X: 0, Y: 0, Width: 80, Height: 40}, layouts[0].Rect)
}

func TestLayoutNil(t *testing.T) {
	t.Parallel()
	require.Nil(t, Layout(nil, 80, 40))
}

func TestDividers(t *testing.T) {
	t.Parallel()
	root := NewSplit(Horizontal, 0.5, NewLeaf("p1"), NewLeaf("p2"))

	dividers := Dividers(root, 101, 50)
	require.Len(t, dividers, 1)
	require.Equal(t, 1, dividers[0].Width)
	require.Equal(t, 50, dividers[0].Height)
}

func TestDividersComplex(t *testing.T) {
	t.Parallel()
	// H(p1, V(p2, p3)) — one vertical divider + one horizontal divider.
	root := NewSplit(Horizontal, 0.5,
		NewLeaf("p1"),
		NewSplit(Vertical, 0.5, NewLeaf("p2"), NewLeaf("p3")),
	)

	dividers := Dividers(root, 101, 51)
	require.Len(t, dividers, 2)
}

func TestDirectionString(t *testing.T) {
	t.Parallel()
	require.Equal(t, "H", Horizontal.String())
	require.Equal(t, "V", Vertical.String())
}

func TestStressManySplits(t *testing.T) {
	t.Parallel()
	root := NewLeaf("p0")
	for i := 1; i <= 20; i++ {
		paneID := "p" + string(rune('0'+i%10)) + string(rune('0'+i/10))
		target := AllLeaves(root)[0]
		require.NoError(t, SplitLeaf(root, target, Direction(i%2), paneID))
	}
	require.Equal(t, 21, LeafCount(root))

	layouts := LayoutWithDividers(root, 200, 100)
	require.Len(t, layouts, 21)

	// All panes have non-negative dimensions. Some may be zero when the
	// area is too small for a divider (graceful degradation).
	for _, l := range layouts {
		require.GreaterOrEqual(t, l.Rect.Width, 0, "pane %s has negative width", l.PaneID)
		require.GreaterOrEqual(t, l.Rect.Height, 0, "pane %s has negative height", l.PaneID)
	}
	// At least the first few panes should have real area.
	require.Greater(t, layouts[0].Rect.Width, 0)
	require.Greater(t, layouts[0].Rect.Height, 0)
}
