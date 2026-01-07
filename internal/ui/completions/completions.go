package completions

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/ui/list"
)

const maxCompletionsHeight = 10

// SelectionMsg is sent when a completion is selected.
type SelectionMsg struct {
	Value  any
	Insert bool // If true, insert without closing.
}

// ClosedMsg is sent when the completions are closed.
type ClosedMsg struct{}

// FilesLoadedMsg is sent when files have been loaded for completions.
type FilesLoadedMsg struct {
	Files []string
}

// Completions represents the completions popup component.
type Completions struct {
	// Popup dimensions
	width  int
	height int

	// State
	open  bool
	query string

	// Key bindings
	keyMap KeyMap

	// List component
	list *list.FilterableList

	// Styling
	normalStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	matchStyle    lipgloss.Style
}

// New creates a new completions component.
func New(normalStyle, selectedStyle, matchStyle lipgloss.Style) *Completions {
	l := list.NewFilterableList()
	l.SetGap(0)

	return &Completions{
		keyMap:        DefaultKeyMap(),
		list:          l,
		normalStyle:   normalStyle,
		selectedStyle: selectedStyle,
		matchStyle:    matchStyle,
	}
}

// IsOpen returns whether the completions popup is open.
func (c *Completions) IsOpen() bool {
	return c.open
}

// Query returns the current filter query.
func (c *Completions) Query() string {
	return c.query
}

// Size returns the visible size of the popup.
func (c *Completions) Size() (width, height int) {
	visible := len(c.list.VisibleItems())
	return c.width, min(visible, c.height)
}

// KeyMap returns the key bindings.
func (c *Completions) KeyMap() KeyMap {
	return c.keyMap
}

// OpenWithFiles opens the completions with file items from the filesystem.
func (c *Completions) OpenWithFiles(depth, limit int) tea.Cmd {
	return func() tea.Msg {
		files, _, _ := fsext.ListDirectory(".", nil, depth, limit)
		slices.Sort(files)
		return FilesLoadedMsg{Files: files}
	}
}

// SetFiles sets the file items on the completions popup.
func (c *Completions) SetFiles(files []string) {
	items := make([]list.FilterableItem, 0, len(files))
	for _, file := range files {
		file = strings.TrimPrefix(file, "./")
		item := NewCompletionItem(
			file,
			FileCompletionValue{Path: file},
			c.normalStyle,
			c.selectedStyle,
			c.matchStyle,
		)
		items = append(items, item)
	}

	c.open = true
	c.query = ""
	c.list.SetItems(items...)
	c.list.SetFilter("") // Clear any previous filter.
	c.list.Focus()

	// Calculate width based on longest item.
	c.width = c.calculateWidth(items)
	c.height = max(min(maxCompletionsHeight, len(items)), 1)
	c.list.SetSize(c.width, c.height)
	c.list.SetSelected(0)
}

// Close closes the completions popup.
func (c *Completions) Close() tea.Cmd {
	c.open = false
	return func() tea.Msg {
		return ClosedMsg{}
	}
}

// Filter filters the completions with the given query.
func (c *Completions) Filter(query string) {
	if !c.open {
		return
	}

	if query == c.query {
		return
	}

	c.query = query
	c.list.SetFilter(query)

	items := c.list.VisibleItems()
	c.height = max(min(maxCompletionsHeight, len(items)), 1)
	c.list.SetSize(c.width, c.height)
	c.list.SetSelected(0)
	c.list.ScrollToSelected()
}

// HasItems returns whether there are visible items.
func (c *Completions) HasItems() bool {
	return len(c.list.VisibleItems()) > 0
}

// Update handles key events for the completions.
func (c *Completions) Update(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if !c.open {
		return nil, false
	}

	switch {
	case key.Matches(msg, c.keyMap.Up):
		c.selectPrev()
		return nil, true

	case key.Matches(msg, c.keyMap.Down):
		c.selectNext()
		return nil, true

	case key.Matches(msg, c.keyMap.UpInsert):
		c.selectPrev()
		return c.selectCurrent(true), true

	case key.Matches(msg, c.keyMap.DownInsert):
		c.selectNext()
		return c.selectCurrent(true), true

	case key.Matches(msg, c.keyMap.Select):
		return c.selectCurrent(false), true

	case key.Matches(msg, c.keyMap.Cancel):
		return c.Close(), true
	}

	return nil, false
}

// selectPrev selects the previous item with circular navigation.
func (c *Completions) selectPrev() {
	items := c.list.VisibleItems()
	if len(items) == 0 {
		return
	}
	if !c.list.SelectPrev() {
		// Wrap to last item.
		c.list.SelectLast()
	}
	c.list.ScrollToSelected()
}

// selectNext selects the next item with circular navigation.
func (c *Completions) selectNext() {
	items := c.list.VisibleItems()
	if len(items) == 0 {
		return
	}
	if !c.list.SelectNext() {
		// Wrap to first item.
		c.list.SelectFirst()
	}
	c.list.ScrollToSelected()
}

// selectCurrent returns a command with the currently selected item.
func (c *Completions) selectCurrent(insert bool) tea.Cmd {
	items := c.list.VisibleItems()
	if len(items) == 0 {
		return nil
	}

	selected := c.list.Selected()
	if selected < 0 || selected >= len(items) {
		return nil
	}

	item, ok := items[selected].(*CompletionItem)
	if !ok {
		return nil
	}

	if !insert {
		c.open = false
	}

	return func() tea.Msg {
		return SelectionMsg{
			Value:  item.Value(),
			Insert: insert,
		}
	}
}

// Render renders the completions popup.
func (c *Completions) Render() string {
	if !c.open {
		return ""
	}

	items := c.list.VisibleItems()
	if len(items) == 0 {
		return ""
	}

	return c.list.Render()
}

// calculateWidth calculates the width based on items.
func (c *Completions) calculateWidth(items []list.FilterableItem) int {
	var width int
	count := min(len(items), 10)
	for i := len(items) - 1; i >= 0 && i >= len(items)-count; i-- {
		itemWidth := lipgloss.Width(items[i].(*CompletionItem).Text()) + 2
		width = max(width, itemWidth)
	}
	return max(width, 20)
}
