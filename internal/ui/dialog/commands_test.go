package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
	"github.com/stretchr/testify/require"
)

type testCommandsWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *testCommandsWorkspace) Config() *config.Config {
	return w.cfg
}

func newTestCommands(t *testing.T) *Commands {
	t.Helper()
	s := styles.CharmtonePantera()
	cfg := &config.Config{}
	com := &common.Common{
		Workspace: &testCommandsWorkspace{cfg: cfg},
		Styles:    &s,
	}
	c, err := NewCommands(com, "", false, false, false, nil, nil)
	require.NoError(t, err)
	return c
}

func TestCommands_SubMenuNavigation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		keys       []tea.KeyPressMsg
		wantStack  int
		wantBread  []string
		wantClose  bool
		wantAction bool
	}{
		{
			name:      "enter parent pushes sub-menu, no action fires",
			keys:      []tea.KeyPressMsg{{Code: tea.KeyEnter}},
			wantStack: 1,
			wantBread: []string{"Parent"},
		},
		{
			name:      "esc in sub-menu pops back",
			keys:      []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: tea.KeyEscape}},
			wantStack: 0,
			wantBread: []string{},
		},
		{
			name:      "backspace in sub-menu pops back",
			keys:      []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: tea.KeyBackspace}},
			wantStack: 0,
			wantBread: []string{},
		},
		{
			name:      "esc at top level closes dialog",
			keys:      []tea.KeyPressMsg{{Code: tea.KeyEscape}},
			wantStack: 0,
			wantClose: true,
		},
		{
			name:       "leaf child action fires on enter",
			keys:       []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: tea.KeyEnter}, {Code: tea.KeyEnter}},
			wantStack:  2,
			wantBread:  []string{"Parent", "Child A"},
			wantAction: true,
		},
		{
			name:      "nested sub-menu pushes twice",
			keys:      []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: tea.KeyEnter}},
			wantStack: 2,
			wantBread: []string{"Parent", "Child A"},
		},
		{
			name:      "pop from nested restores to first sub-menu",
			keys:      []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: tea.KeyEnter}, {Code: tea.KeyEscape}},
			wantStack: 1,
			wantBread: []string{"Parent"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := setupMenuHierarchy(t)

			var gotAction Action
			for _, k := range tc.keys {
				gotAction = c.HandleMsg(k)
			}

			require.Equal(t, tc.wantStack, len(c.menuStack), "menu stack depth")
			require.Equal(t, tc.wantBread, c.breadcrumb, "breadcrumb")

			if tc.wantClose {
				_, ok := gotAction.(ActionClose)
				require.True(t, ok, "expected ActionClose")
			}
			if tc.wantAction {
				require.NotNil(t, gotAction, "expected an action to fire")
				_, ok := gotAction.(ActionNewSession)
				require.True(t, ok, "expected ActionNewSession from leaf")
			}
		})
	}
}

func TestCommands_FilterScopedToCurrentLevel(t *testing.T) {
	t.Parallel()

	c := setupMenuHierarchy(t)

	// Navigate into the sub-menu.
	c.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, c.inSubMenu())

	// Type a filter that matches a child but not the parent.
	c.HandleMsg(keyMsg('a'))

	filtered := c.list.FilteredItems()
	// Should have at least one item (Child A matches "a").
	require.NotEmpty(t, filtered)

	// None of the filtered items should be the parent.
	for _, item := range filtered {
		if ci, ok := item.(*CommandItem); ok {
			require.NotEqual(t, ci.id, "parent", "filter should not show parent items")
		}
	}
}

// TestCommandItem_FilterIncludesShortcut is a regression guard for the
// commands dialog swallowing slash-command names. Typing "/clear" (or
// "clear") after opening the dialog must keep "New Session" visible; the
// filter previously matched only title+aliases+description, so a query for
// the shortcut text left the list empty.
func TestCommandItem_FilterIncludesShortcut(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	item := NewCommandItem(&s, "new_session", "New Session", "ctrl+n / /clear", ActionNewSession{})

	filter := item.Filter()
	require.Contains(t, filter, "/clear", "shortcut must be part of the filter string")
	require.Contains(t, filter, "New Session", "title must remain in the filter string")

	// The fuzzy matcher must find the item by its slash-command name.
	matches := fuzzy.FindFrom("clear", list.FilterableItemsSource{item})
	require.Len(t, matches, 1, "typing 'clear' should match the New Session command")
}

func TestCommands_TabDisabledInSubMenu(t *testing.T) {
	t.Parallel()

	c := setupMenuHierarchy(t)
	require.False(t, c.inSubMenu())

	// Tab normally cycles command types (no-op here since no custom commands).
	c.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, SystemCommands, c.selected)

	// Navigate into sub-menu.
	c.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, c.inSubMenu())

	// Tab should be a no-op while in sub-menu (selected stays SystemCommands).
	c.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, SystemCommands, c.selected)
}

func TestCommands_LeafEnterFiresActionAndCloses(t *testing.T) {
	t.Parallel()

	c := newTestCommands(t)

	// Add a leaf item to the system commands.
	leaf := NewCommandItem(c.com.Styles, "leaf", "Leaf Item", "", ActionNewSession{})
	items := make([]list.FilterableItem, 1)
	items[0] = leaf
	c.list.SetItems(items...)
	c.list.SetSelected(0)

	action := c.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	_, ok := action.(ActionNewSession)
	require.True(t, ok, "leaf item should fire its action")
}

func TestCommands_BreadcrumbDraw(t *testing.T) {
	t.Parallel()

	c := setupMenuHierarchy(t)
	c.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, c.inSubMenu())
	require.Equal(t, []string{"Parent"}, c.breadcrumb)
}

// setupMenuHierarchy creates a Commands dialog with a parent item that has
// children, where one child is itself a parent with a nested leaf.
func TestCommands_PopRestoresFullParentList(t *testing.T) {
	t.Parallel()

	c := newTestCommands(t)

	leaf := NewCommandItem(c.com.Styles, "leaf", "Leaf", "", ActionNewSession{})
	parent := NewCommandItem(c.com.Styles, "parent", "Alpha", "", nil).WithChildren(leaf)
	sibling := NewCommandItem(c.com.Styles, "sibling", "Zulu", "", ActionNewSession{})

	items := []list.FilterableItem{parent, sibling}
	c.list.SetItems(items...)
	c.list.SetSelected(0)

	// Filter so only the parent ("Alpha") matches, leaving the sibling hidden.
	c.HandleMsg(keyMsg('a'))
	require.Len(t, c.list.FilteredItems(), 1, "filter should narrow to the parent")

	// Enter the parent's sub-menu, then pop back to the top level.
	c.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, c.inSubMenu())
	c.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.False(t, c.inSubMenu())

	// Both top-level items must be restored — not just the previously-filtered
	// subset. Regression guard for pushMenu snapshotting FilteredItems() while a
	// filter was active.
	require.Len(t, c.list.FilteredItems(), 2, "popping should restore the full parent list")
}

func TestCommands_BackspaceEditsTopLevelFilter(t *testing.T) {
	t.Parallel()

	c := newTestCommands(t)

	// Type into the top-level type-ahead filter.
	c.HandleMsg(keyMsg('m'))
	c.HandleMsg(keyMsg('c'))
	c.HandleMsg(keyMsg('p'))
	require.Equal(t, "mcp", c.input.Value())
	require.False(t, c.inSubMenu())

	// Backspace at the top level must edit the filter, not be swallowed by the
	// Back (pop-sub-menu) key binding. Regression guard for the case matching
	// backspace unconditionally.
	c.HandleMsg(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.False(t, c.inSubMenu())
	require.Equal(t, "mc", c.input.Value(), "backspace should delete a char from the top-level filter")
}

// TestCommands_ResizeInSubMenuResetsToTopLevel verifies that when Draw
// repopulates the command items because the width changed, any open sub-menu
// is abandoned along with it — otherwise the list would show top-level items
// while inSubMenu() is still true and the breadcrumb is stale.
func TestCommands_ResizeInSubMenuResetsToTopLevel(t *testing.T) {
	t.Parallel()

	c := newTestCommands(t)

	// Establish an initial window width via a first Draw.
	scr := uv.NewScreenBuffer(120, 40)
	c.Draw(scr, uv.Rect(0, 0, 120, 40))
	require.False(t, c.inSubMenu())

	// There are no production sub-menus yet, so inject a parent with a child
	// and enter its sub-menu.
	leaf := NewCommandItem(c.com.Styles, "leaf", "Leaf", "", ActionNewSession{})
	parent := NewCommandItem(c.com.Styles, "parent", "Parent", "", nil).WithChildren(leaf)
	items := []list.FilterableItem{parent}
	c.list.SetItems(items...)
	c.list.SetSelected(0)
	c.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, c.inSubMenu())
	require.Equal(t, []string{"Parent"}, c.breadcrumb)

	// A redraw at the same width must not reset the menu.
	c.Draw(scr, uv.Rect(0, 0, 120, 40))
	require.True(t, c.inSubMenu(), "same-width draws must preserve the sub-menu")

	// A redraw at a different width repopulates the top-level items, so the
	// menu stack and breadcrumb must reset with them.
	scr2 := uv.NewScreenBuffer(140, 40)
	c.Draw(scr2, uv.Rect(0, 0, 140, 40))
	require.False(t, c.inSubMenu(), "width change must reset the sub-menu state")
	require.Empty(t, c.menuStack)
	require.Empty(t, c.breadcrumb)

	// And the visible items are top-level commands again, not the sub-menu's
	// children.
	for _, item := range c.list.FilteredItems() {
		if ci, ok := item.(*CommandItem); ok {
			require.NotEqual(t, "leaf", ci.id, "sub-menu children must not remain in the list")
		}
	}
}

// TestCommands_SubMenuHelpShowsBack verifies the Back binding is advertised
// in the help views only while inside a sub-menu.
func TestCommands_SubMenuHelpShowsBack(t *testing.T) {
	t.Parallel()

	c := setupMenuHierarchy(t)

	shortHasBack := func() bool {
		for _, b := range c.ShortHelp() {
			if b.Help().Desc == "back" {
				return true
			}
		}
		return false
	}
	fullHasBack := func() bool {
		for _, row := range c.FullHelp() {
			for _, b := range row {
				if b.Help().Desc == "back" {
					return true
				}
			}
		}
		return false
	}

	require.False(t, shortHasBack(), "Back must not show in ShortHelp at the top level")
	require.False(t, fullHasBack(), "Back must not show in FullHelp at the top level")

	c.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, c.inSubMenu())
	require.True(t, shortHasBack(), "Back must show in ShortHelp inside a sub-menu")
	require.True(t, fullHasBack(), "Back must show in FullHelp inside a sub-menu")
}

func setupMenuHierarchy(t *testing.T) *Commands {
	t.Helper()
	c := newTestCommands(t)

	leaf := NewCommandItem(c.com.Styles, "leaf", "Leaf", "", ActionNewSession{})
	nestedParent := NewCommandItem(c.com.Styles, "child_a", "Child A", "", nil).WithChildren(leaf)
	leaf2 := NewCommandItem(c.com.Styles, "child_b", "Child B", "", ActionNewSession{})
	parent := NewCommandItem(c.com.Styles, "parent", "Parent", "", nil).WithChildren(nestedParent, leaf2)

	items := make([]list.FilterableItem, 1)
	items[0] = parent
	c.list.SetItems(items...)
	c.list.SetSelected(0)
	return c
}
