package tabmgr

import (
	"testing"

	"github.com/charmbracelet/crush/internal/split"
	"github.com/stretchr/testify/require"
)

func TestTabLayoutSinglePane(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)
	layouts := tab.Layout(120, 40)
	require.Len(t, layouts, 1)
	require.Equal(t, tab.FocusedPane, layouts[0].PaneID)
	require.Equal(t, split.Rect{X: 0, Y: 0, Width: 120, Height: 40}, layouts[0].Rect)
}

func TestTabLayoutAfterSplit(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)
	origPane := tab.FocusedPane

	newPaneID, err := tab.SplitFocused(split.Vertical, PaneSession)
	require.NoError(t, err)

	layouts := tab.Layout(120, 40)
	require.Len(t, layouts, 2)

	// Both panes should have positive dimensions.
	for _, l := range layouts {
		require.Greater(t, l.Rect.Width, 0, "pane %s width", l.PaneID)
		require.Greater(t, l.Rect.Height, 0, "pane %s height", l.PaneID)
	}

	// Pane IDs should match the original and new.
	ids := []string{layouts[0].PaneID, layouts[1].PaneID}
	require.Contains(t, ids, origPane)
	require.Contains(t, ids, newPaneID)
}

func TestTabDividersSinglePane(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)
	divs := tab.Dividers(120, 40)
	require.Empty(t, divs)
}

func TestTabDividersAfterHorizontalSplit(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)

	_, err := tab.SplitFocused(split.Horizontal, PaneSession)
	require.NoError(t, err)

	divs := tab.Dividers(120, 40)
	require.Len(t, divs, 1)

	div := divs[0]
	// Vertical divider: 1 col wide, full height.
	require.Equal(t, 1, div.Width)
	require.Equal(t, 40, div.Height)
}

func TestTabDividersAfterVerticalSplit(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)

	_, err := tab.SplitFocused(split.Vertical, PaneSession)
	require.NoError(t, err)

	divs := tab.Dividers(120, 40)
	require.Len(t, divs, 1)

	div := divs[0]
	// Horizontal divider: full width, 1 row tall.
	require.Equal(t, 120, div.Width)
	require.Equal(t, 1, div.Height)
}

func TestTabLayoutThreePanes(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)

	// Split focused (vertical), then split one of them (horizontal).
	_, err := tab.SplitFocused(split.Vertical, PaneSession)
	require.NoError(t, err)
	_, err = tab.SplitFocused(split.Horizontal, PaneShell)
	require.NoError(t, err)

	require.Equal(t, 3, tab.PaneCount())

	layouts := tab.Layout(120, 40)
	require.Len(t, layouts, 3)

	divs := tab.Dividers(120, 40)
	require.Len(t, divs, 2)

	// No pane overlaps any divider.
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
}

func TestTabLayoutFocusCyclePreservesLayout(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)
	_, err := tab.SplitFocused(split.Horizontal, PaneSession)
	require.NoError(t, err)

	layoutBefore := tab.Layout(80, 24)
	tab.FocusNext()
	layoutAfter := tab.Layout(80, 24)

	// Layout rectangles shouldn't change when focus moves.
	require.Equal(t, len(layoutBefore), len(layoutAfter))
	for i := range layoutBefore {
		require.Equal(t, layoutBefore[i].Rect, layoutAfter[i].Rect)
		require.Equal(t, layoutBefore[i].PaneID, layoutAfter[i].PaneID)
	}
}

func TestTabLayoutAfterClosePane(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)

	newID, err := tab.SplitFocused(split.Horizontal, PaneSession)
	require.NoError(t, err)
	require.Equal(t, 2, tab.PaneCount())

	err = tab.ClosePane(newID)
	require.NoError(t, err)
	require.Equal(t, 1, tab.PaneCount())

	// Back to single pane: fills entire area.
	layouts := tab.Layout(120, 40)
	require.Len(t, layouts, 1)
	require.Equal(t, split.Rect{X: 0, Y: 0, Width: 120, Height: 40}, layouts[0].Rect)

	divs := tab.Dividers(120, 40)
	require.Empty(t, divs)
}

func TestTabLayoutMixedPaneTypes(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)

	shellID, err := tab.SplitFocused(split.Horizontal, PaneShell)
	require.NoError(t, err)

	// Verify pane metadata preserved through layout.
	meta := tab.Panes[shellID]
	require.NotNil(t, meta)
	require.Equal(t, PaneShell, meta.Type)

	layouts := tab.Layout(100, 30)
	require.Len(t, layouts, 2)

	// Find the shell pane in layouts.
	found := false
	for _, l := range layouts {
		if l.PaneID == shellID {
			found = true
			require.Greater(t, l.Rect.Width, 0)
			require.Greater(t, l.Rect.Height, 0)
		}
	}
	require.True(t, found, "shell pane should appear in layout")
}

func TestTabManagerActiveTabLayout(t *testing.T) {
	t.Parallel()
	tm := New()
	tab1 := tm.AddTab("tab1", "/tmp", PaneSession)

	_, err := tab1.SplitFocused(split.Horizontal, PaneSession)
	require.NoError(t, err)

	// Active tab should be tab1.
	active := tm.ActiveTab()
	require.NotNil(t, active)
	require.Equal(t, tab1.ID, active.ID)

	layouts := active.Layout(80, 24)
	require.Len(t, layouts, 2)
}
