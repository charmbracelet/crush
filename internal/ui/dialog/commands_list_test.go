package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func testCommandsList(t *testing.T) (*CommandsList, *styles.Styles) {
	t.Helper()
	sty := styles.CharmtonePantera()
	return NewCommandsList(&sty), &sty
}

func requireVisibleTypes(t *testing.T, items []list.Item, expected ...any) {
	t.Helper()
	require.Len(t, items, len(expected))
	for i, want := range expected {
		switch want.(type) {
		case *CommandGroup:
			_, ok := items[i].(*CommandGroup)
			require.Truef(t, ok, "item %d: expected *CommandGroup, got %T", i, items[i])
		case *CommandItem:
			_, ok := items[i].(*CommandItem)
			require.Truef(t, ok, "item %d: expected *CommandItem, got %T", i, items[i])
		case *list.SpacerItem:
			_, ok := items[i].(*list.SpacerItem)
			require.Truef(t, ok, "item %d: expected *SpacerItem, got %T", i, items[i])
		}
	}
}

func TestCommandsList_GroupedLayout(t *testing.T) {
	t.Parallel()

	l, sty := testCommandsList(t)
	g1 := NewCommandGroup(sty, "Session",
		NewCommandItem(sty, "new_session", "New Session", "", nil),
		NewCommandItem(sty, "switch_session", "Sessions", "", nil),
	)
	g2 := NewCommandGroup(sty, "Application",
		NewCommandItem(sty, "quit", "Quit", "", nil),
	)
	l.SetGroups(g1, g2)

	require.Equal(t, 3, l.Len())
	requireVisibleTypes(t, l.VisibleItems(),
		&CommandGroup{}, &CommandItem{}, &CommandItem{}, &list.SpacerItem{},
		&CommandGroup{}, &CommandItem{},
	)
}

func TestCommandsList_SelectionSkipsHeadersAndSpacers(t *testing.T) {
	t.Parallel()

	l, sty := testCommandsList(t)
	g1 := NewCommandGroup(sty, "Session",
		NewCommandItem(sty, "new_session", "New Session", "", nil),
		NewCommandItem(sty, "switch_session", "Sessions", "", nil),
	)
	g2 := NewCommandGroup(sty, "Application",
		NewCommandItem(sty, "quit", "Quit", "", nil),
	)
	l.SetGroups(g1, g2)

	selectedID := func() string {
		item, ok := l.SelectedItem().(*CommandItem)
		require.True(t, ok, "selected item must be a *CommandItem, got %T", l.SelectedItem())
		return item.ID()
	}

	// SetSelected(0) must skip the section header at index 0.
	l.SetSelected(0)
	require.Equal(t, "new_session", selectedID())
	require.True(t, l.IsSelectedFirst())
	require.False(t, l.IsSelectedLast())

	require.True(t, l.SelectNext())
	require.Equal(t, "switch_session", selectedID())

	// Crossing into the next group must skip the spacer and header.
	require.True(t, l.SelectNext())
	require.Equal(t, "quit", selectedID())
	require.True(t, l.IsSelectedLast())

	require.True(t, l.SelectPrev())
	require.Equal(t, "switch_session", selectedID())

	require.True(t, l.SelectLast())
	require.Equal(t, "quit", selectedID())

	require.True(t, l.SelectFirst())
	require.Equal(t, "new_session", selectedID())
}

func TestCommandsList_FilterKeepsHeadersForMatchedGroups(t *testing.T) {
	t.Parallel()

	l, sty := testCommandsList(t)
	g1 := NewCommandGroup(sty, "Session",
		NewCommandItem(sty, "new_session", "New Session", "", nil),
	)
	g2 := NewCommandGroup(sty, "Application",
		NewCommandItem(sty, "quit", "Quit", "", nil).WithAliases("exit"),
	)
	l.SetGroups(g1, g2)

	// A query matching only the second group keeps its header and drops the
	// first group entirely.
	l.SetFilter("quit")
	visible := l.VisibleItems()
	requireVisibleTypes(t, visible, &CommandGroup{}, &CommandItem{})
	header := visible[0].(*CommandGroup)
	require.Equal(t, "Application", header.Title)

	// Typing a section name surfaces all of its commands.
	l.SetFilter("session")
	visible = l.VisibleItems()
	requireVisibleTypes(t, visible, &CommandGroup{}, &CommandItem{})
	header = visible[0].(*CommandGroup)
	require.Equal(t, "Session", header.Title)

	// Clearing the filter restores all groups.
	l.SetFilter("")
	require.Len(t, l.VisibleItems(), 5)
}

func TestCommandsList_ScrollToSelectedKeepsHeaderVisible(t *testing.T) {
	t.Parallel()

	l, sty := testCommandsList(t)
	g1 := NewCommandGroup(sty, "Session",
		NewCommandItem(sty, "new_session", "New Session", "", nil),
		NewCommandItem(sty, "switch_session", "Sessions", "", nil),
	)
	g2 := NewCommandGroup(sty, "Application",
		NewCommandItem(sty, "quit", "Quit", "", nil),
		NewCommandItem(sty, "restart", "Restart", "", nil),
	)
	l.SetGroups(g1, g2)
	l.SetSize(40, 3)

	// Scroll to the bottom, then back to the first command item. Its group
	// header must stay visible at the top of the viewport.
	l.SelectLast()
	l.ScrollToSelected()
	l.SelectFirst()
	l.ScrollToSelected()

	require.Equal(t, 1, l.Selected())
	startIdx, _ := l.VisibleItemIndices()
	require.Equal(t, 0, startIdx)

	// The same applies when jumping to the first item of a later group.
	l.SelectLast()
	l.ScrollToSelected()
	l.SetSelected(5) // "quit", first item of the "Application" group.
	l.ScrollToSelected()
	startIdx, _ = l.VisibleItemIndices()
	require.LessOrEqual(t, startIdx, 4)
}

func TestCommandsList_ScrollToLastReachesBottom(t *testing.T) {
	t.Parallel()

	l, sty := testCommandsList(t)
	g1 := NewCommandGroup(sty, "Session",
		NewCommandItem(sty, "new_session", "New Session", "", nil),
		NewCommandItem(sty, "switch_session", "Sessions", "", nil),
	)
	g2 := NewCommandGroup(sty, "Application",
		NewCommandItem(sty, "quit", "Quit", "", nil),
		NewCommandItem(sty, "restart", "Restart", "", nil),
	)
	l.SetGroups(g1, g2)
	l.SetSize(40, 3)

	// Selecting the last item must reach the bottom-most scroll offset so
	// the scrollbar thumb touches the bottom of the track.
	l.SelectLast()
	l.ScrollToSelected()
	require.Equal(t, l.TotalHeight()-l.Height(), l.Offset())
}

func TestCommandsContentHeightIgnoresFilter(t *testing.T) {
	t.Parallel()

	l, sty := testCommandsList(t)
	groups := []CommandGroup{
		NewCommandGroup(sty, "Session",
			NewCommandItem(sty, "new_session", "New Session", "", nil),
			NewCommandItem(sty, "switch_session", "Sessions", "", nil),
		),
		NewCommandGroup(sty, "Application",
			NewCommandItem(sty, "quit", "Quit", "", nil),
		),
	}
	l.SetGroups(groups...)
	l.SetSize(40, 3)

	// Two headers, three items and one separator between the sections.
	require.Equal(t, 6, groupsContentHeight(40, groups))
	require.Equal(t, l.TotalHeight(), groupsContentHeight(40, groups))

	// Filtering shrinks the list but not the height the dialog sizes to, so
	// the palette keeps a steady height while typing.
	l.SetFilter("quit")
	require.Less(t, l.TotalHeight(), groupsContentHeight(40, groups))
	require.Equal(t, 6, groupsContentHeight(40, groups))

	// Descriptions make an item two rows tall.
	require.Equal(t, 3, itemsContentHeight(40, []*CommandItem{
		NewCommandItem(sty, "custom_a", "My Command", "", nil).WithDescription("does things"),
		NewCommandItem(sty, "custom_b", "Other Command", "", nil),
	}))
}

func TestCommandsList_FlatModeHasNoHeaders(t *testing.T) {
	t.Parallel()

	l, sty := testCommandsList(t)
	l.SetItems(
		NewCommandItem(sty, "custom_a", "My Command", "", nil),
		NewCommandItem(sty, "custom_b", "Other Command", "", nil),
	)

	require.Equal(t, 2, l.Len())
	requireVisibleTypes(t, l.VisibleItems(), &CommandItem{}, &CommandItem{})

	l.SetSelected(0)
	item, ok := l.SelectedItem().(*CommandItem)
	require.True(t, ok)
	require.Equal(t, "custom_a", item.ID())

	l.SetFilter("other")
	visible := l.VisibleItems()
	require.Len(t, visible, 1)
	item, ok = visible[0].(*CommandItem)
	require.True(t, ok)
	require.Equal(t, "custom_b", item.ID())
}
