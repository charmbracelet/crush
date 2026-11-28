package dialog

import (
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
	// spacers holds one stable separator per group so filtering keeps
	// hitting the list's per-item render cache.
	spacers []list.Item
	query   string
	t       *styles.Styles
}

// NewModelsList creates a new list suitable for model items and groups.
func NewModelsList(sty *styles.Styles, groups ...ModelGroup) *ModelsList {
	f := &ModelsList{
		List: list.NewList(),
		t:    sty,
	}
	f.setGroups(groups)
	f.RegisterRenderCallback(list.FocusedRenderCallback(f.List))
	return f
}

// modelCount returns the number of model items across all groups, filtered or not.
func (f *ModelsList) modelCount() int {
	n := 0
	for _, g := range f.groups {
		n += len(g.Items)
	}
	return n
}

// HasModels reports whether any group has a model, whatever the filter.
func (f *ModelsList) HasModels() bool {
	return f.modelCount() > 0
}

// VisibleCount returns the number of rows the filter leaves, headers and spacers included.
func (f *ModelsList) VisibleCount() int {
	return f.Len()
}

// SetGroups sets the model groups and updates the list items.
func (f *ModelsList) SetGroups(groups ...ModelGroup) {
	f.setGroups(groups)
	items := []list.Item{}
	for gi := range f.groups {
		g := &f.groups[gi]
		items = append(items, g)
		for _, item := range g.Items {
			items = append(items, item)
		}
		// Add a space separator after each provider section
		items = append(items, f.spacers[gi])
	}
	f.SetItems(items...)
}

// setGroups stores the groups along with one spacer per group. Headers are
// referenced as &f.groups[i], so both keep the same pointer across filter
// changes.
func (f *ModelsList) setGroups(groups []ModelGroup) {
	f.groups = groups
	f.spacers = make([]list.Item, len(groups))
	for i := range f.spacers {
		f.spacers[i] = list.NewSpacerItem(1)
	}
}

// SetFilter sets the filter query and updates the list items.
func (f *ModelsList) SetFilter(q string) {
	f.query = q
	f.SetItems(f.VisibleItems()...)
}

// SetSelected sets the selected item index. It overrides the base method to
// skip non-model items.
func (f *ModelsList) SetSelected(index int) {
	if index < 0 || index >= f.modelCount() {
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
		if index >= f.modelCount() {
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
		for gi := range f.groups {
			g := &f.groups[gi]
			items = append(items, g)
			for _, item := range g.Items {
				item.SetMatch(fuzzy.Match{})
				items = append(items, item)
			}
			// Add a space separator after each provider section
			items = append(items, f.spacers[gi])
		}
		return items
	}

	items := []list.Item{}
	var names []string

	// Match each group's items against the query, prefixed with the group
	// title so a provider name matches all of its models.
	for gi := range f.groups {
		g := &f.groups[gi]
		name := strings.ToLower(g.Title) + " "

		names = names[:0]
		for _, item := range g.Items {
			names = append(names, name+item.Filter())
		}

		matches := fuzzy.Find(query, names)
		if len(matches) == 0 {
			continue
		}

		// Sort by original index to preserve order within the group
		sort.SliceStable(matches, func(i, j int) bool {
			return matches[i].Index < matches[j].Index
		})

		// Add section header
		items = append(items, g)
		for _, match := range matches {
			item := g.Items[match.Index]
			idxs := []int{}
			for _, idx := range match.MatchedIndexes {
				// Adjusts removing provider name highlights
				if idx < len(name) {
					continue
				}
				idxs = append(idxs, idx-len(name))
			}

			match.MatchedIndexes = idxs
			// Add the matched item
			item.SetMatch(match)
			items = append(items, item)
		}
		// Add a space separator after each provider section
		items = append(items, f.spacers[gi])
	}

	return items
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
