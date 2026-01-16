package dialog

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/sahilm/fuzzy"
)

// ModelsList is a list specifically for model items and groups.
type ModelsList struct {
	*list.List
	groups []ModelGroup
	query  string
	t      *styles.Styles
}

// NewModelsList creates a new list suitable for model items and groups.
func NewModelsList(sty *styles.Styles, groups ...ModelGroup) *ModelsList {
	f := &ModelsList{
		List:   list.NewList(),
		groups: groups,
		t:      sty,
	}
	return f
}

// Len returns the number of model items across all groups.
func (f *ModelsList) Len() int {
	n := 0
	for _, g := range f.groups {
		n += len(g.Items)
	}
	return n
}

// SetGroups sets the model groups and updates the list items.
func (f *ModelsList) SetGroups(groups ...ModelGroup) {
	f.groups = groups
	items := []list.Item{}
	for _, g := range f.groups {
		items = append(items, &g)
		for _, item := range g.Items {
			items = append(items, item)
		}
		// Add a space separator after each provider section
		items = append(items, list.NewSpacerItem(1))
	}
	f.SetItems(items...)
}

// SetFilter sets the filter query and updates the list items.
func (f *ModelsList) SetFilter(q string) {
	f.query = q
}

// SetSelected sets the selected item index. It overrides the base method to
// skip non-model items.
func (f *ModelsList) SetSelected(index int) {
	if index < 0 || index >= f.Len() {
		f.List.SetSelected(index)
		return
	}

	f.List.SetSelected(index)
	for {
		selectedItem := f.SelectedItem()
		if _, ok := selectedItem.(*ModelItem); ok {
			return
		}
		f.List.SetSelected(index + 1)
		index++
		if index >= f.Len() {
			return
		}
	}
}

// SetSelectedItem sets the selected item in the list by item ID.
func (f *ModelsList) SetSelectedItem(itemID string) {
	if itemID == "" {
		f.SetSelected(0)
		return
	}

	count := 0
	for _, g := range f.groups {
		for _, item := range g.Items {
			if item.ID() == itemID {
				f.SetSelected(count)
				return
			}
			count++
		}
	}
}

// SelectNext selects the next model item, skipping any non-focusable items
// like group headers and spacers.
func (f *ModelsList) SelectNext() (v bool) {
	for {
		v = f.List.SelectNext()
		selectedItem := f.SelectedItem()
		if _, ok := selectedItem.(*ModelItem); ok {
			return v
		}
	}
}

// SelectPrev selects the previous model item, skipping any non-focusable items
// like group headers and spacers.
func (f *ModelsList) SelectPrev() (v bool) {
	for {
		v = f.List.SelectPrev()
		selectedItem := f.SelectedItem()
		if _, ok := selectedItem.(*ModelItem); ok {
			return v
		}
	}
}

// SelectFirst selects the first model item in the list.
func (f *ModelsList) SelectFirst() (v bool) {
	v = f.List.SelectFirst()
	for {
		selectedItem := f.SelectedItem()
		if _, ok := selectedItem.(*ModelItem); ok {
			return v
		}
		v = f.List.SelectNext()
	}
}

// SelectLast selects the last model item in the list.
func (f *ModelsList) SelectLast() (v bool) {
	v = f.List.SelectLast()
	for {
		selectedItem := f.SelectedItem()
		if _, ok := selectedItem.(*ModelItem); ok {
			return v
		}
		v = f.List.SelectPrev()
	}
}

// IsSelectedFirst checks if the selected item is the first model item.
func (f *ModelsList) IsSelectedFirst() bool {
	originalIndex := f.Selected()
	f.SelectFirst()
	isFirst := f.Selected() == originalIndex
	f.List.SetSelected(originalIndex)
	return isFirst
}

// IsSelectedLast checks if the selected item is the last model item.
func (f *ModelsList) IsSelectedLast() bool {
	originalIndex := f.Selected()
	f.SelectLast()
	isLast := f.Selected() == originalIndex
	f.List.SetSelected(originalIndex)
	return isLast
}

// VisibleItems returns the visible items after filtering.
func (f *ModelsList) VisibleItems() []list.Item {
	query := strings.ToLower(strings.ReplaceAll(f.query, " ", ""))

	if query == "" {
		// No filter, return all items with group headers
		items := []list.Item{}
		for _, g := range f.groups {
			items = append(items, &g)
			for _, item := range g.Items {
				item.SetMatch(fuzzy.Match{})
				items = append(items, item)
			}
			// Add a space separator after each provider section
			items = append(items, list.NewSpacerItem(1))
		}
		return items
	}

	filterableItems := make([]list.FilterableItem, 0, f.Len())
	for _, g := range f.groups {
		for _, item := range g.Items {
			filterableItems = append(filterableItems, item)
		}
	}

	items := []list.Item{}
	visitedGroups := map[int]bool{}

	// Reconstruct groups with matched items
	// Find which group this item belongs to
	for gi, g := range f.groups {
		addedCount := 0
		name := strings.ToLower(g.Title) + " "

		names := make([]string, len(filterableItems))
		for i, item := range filterableItems {
			ms := item.(*ModelItem)
			names[i] = fmt.Sprintf("%s%s", name, ms.Filter())
		}

		matches := fuzzy.Find(query, names)
		sort.SliceStable(matches, func(i, j int) bool {
			return matches[i].Score > matches[j].Score
		})

		for _, match := range matches {
			item := filterableItems[match.Index].(*ModelItem)
			idxs := []int{}
			for _, idx := range match.MatchedIndexes {
				// Adjusts removing provider name highlights
				if idx < len(name) {
					continue
				}
				idxs = append(idxs, idx-len(name))
			}

			match.MatchedIndexes = idxs
			if slices.Contains(g.Items, item) {
				if !visitedGroups[gi] {
					// Add section header
					items = append(items, &g)
					visitedGroups[gi] = true
				}
				// Add the matched item
				item.SetMatch(match)
				items = append(items, item)
				addedCount++
			}
		}
		if addedCount > 0 {
			// Add a space separator after each provider section
			items = append(items, list.NewSpacerItem(1))
		}
	}

	return items
}

// Render renders the filterable list.
func (f *ModelsList) Render() string {
	f.SetItems(f.VisibleItems()...)
	return f.List.Render()
}

type modelGroups []ModelGroup

func (m modelGroups) Len() int {
	n := 0
	for _, g := range m {
		n += len(g.Items)
	}
	return n
}

func (m modelGroups) String(i int) string {
	count := 0
	for _, g := range m {
		if i < count+len(g.Items) {
			return g.Items[i-count].Filter()
		}
		count += len(g.Items)
	}
	return ""
}
