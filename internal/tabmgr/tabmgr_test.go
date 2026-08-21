package tabmgr

import (
	"testing"

	"github.com/charmbracelet/crush/internal/split"
	"github.com/stretchr/testify/require"
)

func TestNewTab(t *testing.T) {
	t.Parallel()
	tab := NewTab("main", "/home/user/project", PaneSession)

	require.NotEmpty(t, tab.ID)
	require.Equal(t, "main", tab.Name)
	require.Equal(t, "/home/user/project", tab.CWD)
	require.Equal(t, 1, tab.PaneCount())
	require.NotEmpty(t, tab.FocusedPane)
	require.Len(t, tab.Panes, 1)

	meta := tab.Panes[tab.FocusedPane]
	require.Equal(t, PaneSession, meta.Type)
}

func TestTabSplitFocused(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)

	newID, err := tab.SplitFocused(split.Horizontal, PaneShell)
	require.NoError(t, err)
	require.NotEmpty(t, newID)
	require.Equal(t, 2, tab.PaneCount())

	meta := tab.Panes[newID]
	require.Equal(t, PaneShell, meta.Type)
	require.Equal(t, "/tmp", meta.CWD)
}

func TestTabSplitPane(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)
	firstPane := tab.FocusedPane

	p2, err := tab.SplitPane(firstPane, split.Vertical, PaneShell)
	require.NoError(t, err)

	p3, err := tab.SplitPane(p2, split.Horizontal, PaneSession)
	require.NoError(t, err)

	require.Equal(t, 3, tab.PaneCount())
	require.NotNil(t, tab.Panes[firstPane])
	require.NotNil(t, tab.Panes[p2])
	require.NotNil(t, tab.Panes[p3])
}

func TestTabSplitPaneNotFound(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)
	_, err := tab.SplitPane("nonexistent", split.Horizontal, PaneShell)
	require.Error(t, err)
}

func TestTabClosePane(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)
	firstPane := tab.FocusedPane

	p2, _ := tab.SplitFocused(split.Horizontal, PaneShell)

	// Close the original pane.
	err := tab.ClosePane(firstPane)
	require.NoError(t, err)
	require.Equal(t, 1, tab.PaneCount())
	require.Equal(t, p2, tab.FocusedPane)
	require.Nil(t, tab.Panes[firstPane])
}

func TestTabClosePaneOnlyPaneErrors(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)
	err := tab.ClosePane(tab.FocusedPane)
	require.Error(t, err)
}

func TestTabFocusCycle(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)
	first := tab.FocusedPane

	p2, _ := tab.SplitFocused(split.Horizontal, PaneShell)
	p3, _ := tab.SplitPane(p2, split.Vertical, PaneSession)

	// Focus is still on first pane.
	tab.FocusedPane = first

	// Cycle forward.
	require.Equal(t, p2, tab.FocusNext())
	require.Equal(t, p3, tab.FocusNext())
	require.Equal(t, first, tab.FocusNext()) // wraps

	// Cycle backward.
	require.Equal(t, p3, tab.FocusPrev())
	require.Equal(t, p2, tab.FocusPrev())
	require.Equal(t, first, tab.FocusPrev()) // wraps
}

func TestTabLayout(t *testing.T) {
	t.Parallel()
	tab := NewTab("test", "/tmp", PaneSession)
	tab.SplitFocused(split.Horizontal, PaneShell)

	layouts := tab.Layout(100, 50)
	require.Len(t, layouts, 2)

	dividers := tab.Dividers(100, 50)
	require.Len(t, dividers, 1)
}

func TestPaneTypeString(t *testing.T) {
	t.Parallel()
	require.Equal(t, "session", PaneSession.String())
	require.Equal(t, "shell", PaneShell.String())
}

// --- TabManager tests ---

func TestTabManagerAddTab(t *testing.T) {
	t.Parallel()
	mgr := New()

	tab1 := mgr.AddTab("main", "/project", PaneSession)
	require.Equal(t, 1, mgr.Len())
	require.Equal(t, tab1, mgr.ActiveTab())
	require.Equal(t, 0, mgr.ActiveIndex())

	tab2 := mgr.AddTab("shell", "/tmp", PaneShell)
	require.Equal(t, 2, mgr.Len())
	require.Equal(t, tab2, mgr.ActiveTab())
	require.Equal(t, 1, mgr.ActiveIndex())
}

func TestTabManagerCloseTab(t *testing.T) {
	t.Parallel()
	mgr := New()
	mgr.AddTab("a", "/a", PaneSession)
	mgr.AddTab("b", "/b", PaneSession)
	mgr.AddTab("c", "/c", PaneSession)

	// Active is tab 2 (index 2). Close it.
	err := mgr.CloseActiveTab()
	require.NoError(t, err)
	require.Equal(t, 2, mgr.Len())
	require.Equal(t, "b", mgr.ActiveTab().Name)
}

func TestTabManagerCloseFirstTab(t *testing.T) {
	t.Parallel()
	mgr := New()
	mgr.AddTab("a", "/a", PaneSession)
	mgr.AddTab("b", "/b", PaneSession)

	require.NoError(t, mgr.SelectTab(0))
	require.NoError(t, mgr.CloseTab(0))
	require.Equal(t, 1, mgr.Len())
	require.Equal(t, "b", mgr.ActiveTab().Name)
	require.Equal(t, 0, mgr.ActiveIndex())
}

func TestTabManagerCloseAllTabs(t *testing.T) {
	t.Parallel()
	mgr := New()
	mgr.AddTab("a", "/a", PaneSession)

	require.NoError(t, mgr.CloseTab(0))
	require.Equal(t, 0, mgr.Len())
	require.Nil(t, mgr.ActiveTab())
}

func TestTabManagerCloseTabOutOfRange(t *testing.T) {
	t.Parallel()
	mgr := New()
	require.Error(t, mgr.CloseTab(0))
	require.Error(t, mgr.CloseTab(-1))
}

func TestTabManagerSelectTab(t *testing.T) {
	t.Parallel()
	mgr := New()
	mgr.AddTab("a", "/a", PaneSession)
	mgr.AddTab("b", "/b", PaneSession)
	mgr.AddTab("c", "/c", PaneSession)

	require.NoError(t, mgr.SelectTab(0))
	require.Equal(t, "a", mgr.ActiveTab().Name)

	require.NoError(t, mgr.SelectTab(2))
	require.Equal(t, "c", mgr.ActiveTab().Name)

	require.Error(t, mgr.SelectTab(5))
}

func TestTabManagerSelectTabByID(t *testing.T) {
	t.Parallel()
	mgr := New()
	tab1 := mgr.AddTab("a", "/a", PaneSession)
	mgr.AddTab("b", "/b", PaneSession)

	require.NoError(t, mgr.SelectTabByID(tab1.ID))
	require.Equal(t, "a", mgr.ActiveTab().Name)

	require.Error(t, mgr.SelectTabByID("nonexistent"))
}

func TestTabManagerNextPrev(t *testing.T) {
	t.Parallel()
	mgr := New()
	mgr.AddTab("a", "/a", PaneSession)
	mgr.AddTab("b", "/b", PaneSession)
	mgr.AddTab("c", "/c", PaneSession)

	require.NoError(t, mgr.SelectTab(0))

	mgr.NextTab()
	require.Equal(t, "b", mgr.ActiveTab().Name)
	mgr.NextTab()
	require.Equal(t, "c", mgr.ActiveTab().Name)
	mgr.NextTab()
	require.Equal(t, "a", mgr.ActiveTab().Name) // wraps

	mgr.PrevTab()
	require.Equal(t, "c", mgr.ActiveTab().Name) // wraps back
}

func TestTabManagerMoveTab(t *testing.T) {
	t.Parallel()
	mgr := New()
	mgr.AddTab("a", "/a", PaneSession)
	mgr.AddTab("b", "/b", PaneSession)
	mgr.AddTab("c", "/c", PaneSession)

	// Active is "c" at index 2. Move up.
	mgr.MoveTabUp()
	require.Equal(t, 1, mgr.ActiveIndex())
	require.Equal(t, "c", mgr.ActiveTab().Name)

	tabs := mgr.Tabs()
	require.Equal(t, "a", tabs[0].Name)
	require.Equal(t, "c", tabs[1].Name)
	require.Equal(t, "b", tabs[2].Name)

	// Move back down.
	mgr.MoveTabDown()
	require.Equal(t, 2, mgr.ActiveIndex())
	tabs = mgr.Tabs()
	require.Equal(t, "a", tabs[0].Name)
	require.Equal(t, "b", tabs[1].Name)
	require.Equal(t, "c", tabs[2].Name)
}

func TestTabManagerMoveTabEdges(t *testing.T) {
	t.Parallel()
	mgr := New()
	mgr.AddTab("a", "/a", PaneSession)
	mgr.AddTab("b", "/b", PaneSession)

	// Select first, move up (no-op).
	require.NoError(t, mgr.SelectTab(0))
	mgr.MoveTabUp()
	require.Equal(t, 0, mgr.ActiveIndex())

	// Select last, move down (no-op).
	require.NoError(t, mgr.SelectTab(1))
	mgr.MoveTabDown()
	require.Equal(t, 1, mgr.ActiveIndex())
}

func TestTabManagerFindTab(t *testing.T) {
	t.Parallel()
	mgr := New()
	tab := mgr.AddTab("test", "/test", PaneSession)

	require.Equal(t, tab, mgr.FindTab(tab.ID))
	require.Nil(t, mgr.FindTab("nonexistent"))
}

func TestTabManagerGetTab(t *testing.T) {
	t.Parallel()
	mgr := New()
	tab := mgr.AddTab("test", "/test", PaneSession)

	require.Equal(t, tab, mgr.GetTab(0))
	require.Nil(t, mgr.GetTab(1))
	require.Nil(t, mgr.GetTab(-1))
}

func TestTabManagerRename(t *testing.T) {
	t.Parallel()
	mgr := New()
	mgr.AddTab("old", "/test", PaneSession)

	require.NoError(t, mgr.RenameTab(0, "new"))
	require.Equal(t, "new", mgr.GetTab(0).Name)
	require.Error(t, mgr.RenameTab(5, "bad"))
}

func TestTabManagerEmpty(t *testing.T) {
	t.Parallel()
	mgr := New()
	require.Equal(t, 0, mgr.Len())
	require.Nil(t, mgr.ActiveTab())

	// Operations on empty manager should not panic.
	mgr.NextTab()
	mgr.PrevTab()
	mgr.MoveTabUp()
	mgr.MoveTabDown()
}
