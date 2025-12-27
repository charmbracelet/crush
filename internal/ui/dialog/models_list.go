package dialog

import (
	"slices"

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

// VisibleItems returns the visible items after filtering.
func (f *ModelsList) VisibleItems() []list.Item {
	if len(f.query) == 0 {
		// No filter, return all items with group headers
		items := []list.Item{}
		for _, g := range f.groups {
			items = append(items, &g)
			for _, item := range g.Items {
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

	matches := fuzzy.FindFrom(f.query, list.FilterableItemsSource(filterableItems))
	for _, match := range matches {
		item := filterableItems[match.Index]
		if ms, ok := item.(list.MatchSettable); ok {
			ms.SetMatch(match)
			item = ms.(list.FilterableItem)
		}
		filterableItems = append(filterableItems, item)
	}

	items := []list.Item{}
	visitedGroups := map[int]bool{}

	// Reconstruct groups with matched items
	// Find which group this item belongs to
	for gi, g := range f.groups {
		addedCount := 0
		for _, match := range matches {
			item := filterableItems[match.Index]
			if slices.Contains(g.Items, item.(*ModelItem)) {
				if !visitedGroups[gi] {
					// Add section header
					items = append(items, &g)
					visitedGroups[gi] = true
				}
				// Add the matched item
				if ms, ok := item.(list.MatchSettable); ok {
					ms.SetMatch(match)
					item = ms.(list.FilterableItem)
				}
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
