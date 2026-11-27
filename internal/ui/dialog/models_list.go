package dialog

import (
	"sort"
	"strings"

	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/sahilm/fuzzy"
)

// filterCache holds precomputed filter strings for each model item.
type filterCache struct {
	strings  []string
	items    []list.FilterableItem
	groupIdx []int
}

// ModelsList is a list specifically for model items and groups.
type ModelsList struct {
	*list.List
	groups      []ModelGroup
	query       string
	t           *styles.Styles
	filterCache *filterCache
}

// NewModelsList creates a new list suitable for model items and groups.
func NewModelsList(sty *styles.Styles, groups ...ModelGroup) *ModelsList {
	f := &ModelsList{
		List:   list.NewList(),
		groups: groups,
		t:      sty,
	}
	f.RegisterRenderCallback(list.FocusedRenderCallback(f.List))
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
	f.filterCache = nil // Invalidate cache when groups change
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
	f.SetItems(f.VisibleItems()...)
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
		return
	}

	// Walk the selectable model items using the same helpers that
	// keyboard navigation uses, so we stay in sync with the flat
	// list layout.
	for ok := f.SelectFirst(); ok; ok = f.SelectNext() {
		if mi, is := f.SelectedItem().(*ModelItem); is && mi.ID() == itemID {
			return
		}
	}
}

// SelectNext selects the next model item, skipping any non-focusable items
// like group headers and spacers.
func (f *ModelsList) SelectNext() (v bool) {
	v = f.List.SelectNext()
	for v {
		selectedItem := f.SelectedItem()
		if _, ok := selectedItem.(*ModelItem); ok {
			return v
		}
		v = f.List.SelectNext()
	}
	return v
}

// SelectPrev selects the previous model item, skipping any non-focusable items
// like group headers and spacers.
func (f *ModelsList) SelectPrev() (v bool) {
	v = f.List.SelectPrev()
	for v {
		selectedItem := f.SelectedItem()
		if _, ok := selectedItem.(*ModelItem); ok {
			return v
		}
		v = f.List.SelectPrev()
	}
	return v
}

// SelectFirst selects the first model item in the list.
func (f *ModelsList) SelectFirst() (v bool) {
	v = f.List.SelectFirst()
	for v {
		selectedItem := f.SelectedItem()
		_, ok := selectedItem.(*ModelItem)
		if ok {
			return v
		}
		v = f.List.SelectNext()
	}
	return v
}

// SelectLast selects the last model item in the list.
func (f *ModelsList) SelectLast() (v bool) {
	v = f.List.SelectLast()
	for v {
		selectedItem := f.SelectedItem()
		if _, ok := selectedItem.(*ModelItem); ok {
			return v
		}
		v = f.List.SelectPrev()
	}
	return v
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

	// Build or reuse filter cache
	if f.filterCache == nil {
		f.filterCache = f.buildFilterCache()
	}

	cache := f.filterCache
	if len(cache.items) == 0 {
		return []list.Item{}
	}

	// Run fuzzy matching once on all items
	matches := fuzzy.Find(query, cache.strings)

	// Sort matches by score (descending), then by original index (ascending)
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].Index < matches[j].Index
	})

	// Group matched items by their group index
	groupMatches := make(map[int][]fuzzy.Match)
	for _, match := range matches {
		gi := cache.groupIdx[match.Index]
		groupMatches[gi] = append(groupMatches[gi], match)
	}

	// Build result items preserving group order
	items := []list.Item{}
	for gi, g := range f.groups {
		matches := groupMatches[gi]
		if len(matches) == 0 {
			continue
		}

		// Add section header
		items = append(items, &g)

		// Sort matches within this group by original index to preserve order
		sort.SliceStable(matches, func(i, j int) bool {
			return matches[i].Index < matches[j].Index
		})

		name := strings.ToLower(g.Title) + " "
		for _, match := range matches {
			item := cache.items[match.Index].(*ModelItem)
			// Adjust matched indexes to remove provider name prefix
			idxs := make([]int, 0, len(match.MatchedIndexes))
			for _, idx := range match.MatchedIndexes {
				if idx >= len(name) {
					idxs = append(idxs, idx-len(name))
				}
			}
			match.MatchedIndexes = idxs
			item.SetMatch(match)
			items = append(items, item)
		}

		// Add a space separator after each provider section
		items = append(items, list.NewSpacerItem(1))
	}

	return items
}

// buildFilterCache builds the filter cache for all items.
func (f *ModelsList) buildFilterCache() *filterCache {
	var filterStrings []string
	var items []list.FilterableItem
	var groupIdx []int

	for gi, g := range f.groups {
		name := strings.ToLower(g.Title) + " "
		for _, item := range g.Items {
			ms := item
			filterStrings = append(filterStrings, name+ms.Filter())
			items = append(items, item)
			groupIdx = append(groupIdx, gi)
		}
	}

	return &filterCache{
		strings:  filterStrings,
		items:    items,
		groupIdx: groupIdx,
	}
}

// Render renders the filterable list.
func (f *ModelsList) Render() string {
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
