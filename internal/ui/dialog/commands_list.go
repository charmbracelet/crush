package dialog

import (
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/sahilm/fuzzy"
)

// CommandsList is a list specifically for command items and groups. It
// mirrors [ModelsList]: groups are flattened into the item list as section
// headers interleaved with their items, and selection skips headers and
// spacers. When no groups are set the list behaves like a plain filterable
// list of commands.
type CommandsList struct {
	*list.List
	groups []CommandGroup
	items  []*CommandItem
	query  string
	t      *styles.Styles
}

// NewCommandsList creates a new list suitable for command items and groups.
func NewCommandsList(sty *styles.Styles) *CommandsList {
	f := &CommandsList{
		List: list.NewList(),
		t:    sty,
	}
	f.RegisterRenderCallback(list.FocusedRenderCallback(f.List))
	return f
}

// Len returns the number of command items across all groups, or the number
// of flat items when no groups are set.
func (f *CommandsList) Len() int {
	if f.groups == nil {
		return len(f.items)
	}
	n := 0
	for _, g := range f.groups {
		n += len(g.Items)
	}
	return n
}

// groupsContentHeight returns the height the given sections occupy at width,
// headers and the blank separators between sections included. Callers use it
// to size a dialog to unfiltered content, so it ignores any active filter.
func groupsContentHeight(width int, groups []CommandGroup) int {
	height := 0
	for gi := range groups {
		g := &groups[gi]
		// Sections after the first are preceded by a blank separator row.
		if height > 0 {
			height++
		}
		height += lipgloss.Height(g.Render(width))
		height += itemsContentHeight(width, g.Items)
	}
	return height
}

// itemsContentHeight returns the height the given command items occupy at
// width. Items carrying a description take more than a single row.
func itemsContentHeight(width int, items []*CommandItem) int {
	height := 0
	for _, item := range items {
		height += lipgloss.Height(item.Render(width))
	}
	return height
}

// SetGroups sets the command groups and updates the list items.
func (f *CommandsList) SetGroups(groups ...CommandGroup) {
	f.groups = groups
	f.items = nil
	f.setListItems(f.VisibleItems()...)
}

// SetItems sets flat command items without section headers and updates the
// list items.
func (f *CommandsList) SetItems(items ...*CommandItem) {
	f.groups = nil
	f.items = items
	f.setListItems(f.VisibleItems()...)
}

// setListItems replaces the underlying list items.
func (f *CommandsList) setListItems(items ...list.Item) {
	f.List.SetItems(items...)
}

// SetFilter sets the filter query and updates the list items.
func (f *CommandsList) SetFilter(q string) {
	f.query = q
	f.setListItems(f.VisibleItems()...)
}

// SetSelected sets the selected item index. It overrides the base method to
// skip non-command items.
func (f *CommandsList) SetSelected(index int) {
	if index < 0 || index >= f.List.Len() {
		f.List.SetSelected(index)
		return
	}

	f.List.SetSelected(index)
	for {
		selectedItem := f.SelectedItem()
		if _, ok := selectedItem.(*CommandItem); ok {
			return
		}
		index++
		if index >= f.List.Len() {
			return
		}
		f.List.SetSelected(index)
	}
}

// SelectNext selects the next command item, skipping any non-focusable
// items like group headers and spacers.
func (f *CommandsList) SelectNext() (v bool) {
	v = f.List.SelectNext()
	for v {
		selectedItem := f.SelectedItem()
		if _, ok := selectedItem.(*CommandItem); ok {
			return v
		}
		v = f.List.SelectNext()
	}
	return v
}

// SelectPrev selects the previous command item, skipping any non-focusable
// items like group headers and spacers.
func (f *CommandsList) SelectPrev() (v bool) {
	v = f.List.SelectPrev()
	for v {
		selectedItem := f.SelectedItem()
		if _, ok := selectedItem.(*CommandItem); ok {
			return v
		}
		v = f.List.SelectPrev()
	}
	return v
}

// SelectFirst selects the first command item in the list.
func (f *CommandsList) SelectFirst() (v bool) {
	v = f.List.SelectFirst()
	for v {
		selectedItem := f.SelectedItem()
		_, ok := selectedItem.(*CommandItem)
		if ok {
			return v
		}
		v = f.List.SelectNext()
	}
	return v
}

// SelectLast selects the last command item in the list.
func (f *CommandsList) SelectLast() (v bool) {
	v = f.List.SelectLast()
	for v {
		selectedItem := f.SelectedItem()
		if _, ok := selectedItem.(*CommandItem); ok {
			return v
		}
		v = f.List.SelectPrev()
	}
	return v
}

// IsSelectedFirst checks if the selected item is the first command item.
func (f *CommandsList) IsSelectedFirst() bool {
	originalIndex := f.Selected()
	f.SelectFirst()
	isFirst := f.Selected() == originalIndex
	f.List.SetSelected(originalIndex)
	return isFirst
}

// IsSelectedLast checks if the selected item is the last command item.
func (f *CommandsList) IsSelectedLast() bool {
	originalIndex := f.Selected()
	f.SelectLast()
	isLast := f.Selected() == originalIndex
	f.List.SetSelected(originalIndex)
	return isLast
}

// ScrollToSelected scrolls the list to the selected item. When the selected
// item is the first command item of a group, the group's section header is
// kept visible above it instead of being scrolled off the top.
func (f *CommandsList) ScrollToSelected() {
	f.List.ScrollToSelected()

	selected := f.Selected()
	if selected <= 0 {
		return
	}
	if _, ok := f.ItemAt(selected - 1).(*CommandGroup); !ok {
		return
	}
	startIdx, _ := f.VisibleItemIndices()
	if startIdx > selected-1 {
		f.ScrollToIndex(selected - 1)
	}
}

// VisibleItems returns the visible items after filtering, with group headers
// for sections that contain visible items.
func (f *CommandsList) VisibleItems() []list.Item {
	// Flat mode: no section headers, plain fuzzy filtering.
	if f.groups == nil {
		return f.visibleFlatItems()
	}

	if f.query == "" {
		items := []list.Item{}
		for gi := range f.groups {
			g := &f.groups[gi]
			// Separate sections with a blank line. No trailing spacer: the
			// last command item doubles as the content end so scrolling to
			// it reaches the bottom-most scroll offset.
			if len(items) > 0 {
				items = append(items, list.NewSpacerItem(1))
			}
			items = append(items, g)
			for _, item := range g.Items {
				item.SetMatch(fuzzy.Match{})
				items = append(items, item)
			}
		}
		return items
	}

	query := strings.ToLower(strings.ReplaceAll(f.query, " ", ""))
	items := []list.Item{}
	for gi := range f.groups {
		g := &f.groups[gi]
		if len(g.Items) == 0 {
			continue
		}

		// Match against the section title prefixed to each item so typing a
		// section name surfaces all of its commands.
		name := strings.ToLower(g.Title) + " "
		names := make([]string, len(g.Items))
		for i, item := range g.Items {
			names[i] = name + item.Filter()
		}

		matches := fuzzy.Find(query, names)

		// Sort by original index to preserve order within the group.
		sort.SliceStable(matches, func(i, j int) bool {
			return matches[i].Index < matches[j].Index
		})

		if len(matches) == 0 {
			continue
		}

		if len(items) > 0 {
			items = append(items, list.NewSpacerItem(1))
		}
		items = append(items, g)
		for _, match := range matches {
			item := g.Items[match.Index]
			idxs := []int{}
			for _, idx := range match.MatchedIndexes {
				// Adjusts removing section title highlights.
				if idx < len(name) {
					continue
				}
				idxs = append(idxs, idx-len(name))
			}
			match.MatchedIndexes = idxs
			item.SetMatch(match)
			items = append(items, item)
		}
	}

	return items
}

// visibleFlatItems returns the visible items for the ungrouped (flat) mode.
func (f *CommandsList) visibleFlatItems() []list.Item {
	if f.query == "" {
		items := make([]list.Item, len(f.items))
		for i, item := range f.items {
			item.SetMatch(fuzzy.Match{})
			items[i] = item
		}
		return items
	}

	source := make(list.FilterableItemsSource, len(f.items))
	for i, item := range f.items {
		source[i] = item
	}
	matches := fuzzy.FindFrom(f.query, source)
	items := []list.Item{}
	for _, match := range matches {
		item := f.items[match.Index]
		item.SetMatch(match)
		items = append(items, item)
	}
	return items
}

// Render renders the list.
func (f *CommandsList) Render() string {
	return f.List.Render()
}

// commandItems flattens the given items to only command items, skipping
// headers and spacers.
func commandItems(items []list.Item) []*CommandItem {
	cmds := []*CommandItem{}
	for _, item := range items {
		if ci, ok := item.(*CommandItem); ok && ci != nil {
			cmds = append(cmds, ci)
		}
	}
	return cmds
}
