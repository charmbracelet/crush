package completions

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/ordered"
)

const (
	minHeight = 1
	maxHeight = 10
	minWidth  = 10
	maxWidth  = 100
)

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
	normalStyle  lipgloss.Style
	focusedStyle lipgloss.Style
	matchStyle   lipgloss.Style
}

// New creates a new completions component.
func New(normalStyle, focusedStyle, matchStyle lipgloss.Style) *Completions {
	l := list.NewFilterableList()
	l.SetGap(0)
	l.SetReverse(true)

	return &Completions{
		keyMap:       DefaultKeyMap(),
		list:         l,
		normalStyle:  normalStyle,
		focusedStyle: focusedStyle,
		matchStyle:   matchStyle,
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
	visible := len(c.list.FilteredItems())
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
			c.focusedStyle,
			c.matchStyle,
		)
		items = append(items, item)
	}

	c.open = true
	c.query = ""
	c.list.SetItems(items...)
	c.list.SetFilter("") // Clear any previous filter.
	c.list.Focus()

	c.width = maxWidth
	c.height = ordered.Clamp(len(items), int(minHeight), int(maxHeight))
	c.list.SetSize(c.width, c.height)
	c.list.SelectFirst()
	c.list.ScrollToSelected()

	// recalculate width by using just the visible items
	start, end := c.list.VisibleItemIndices()
	width := 0
	if end != 0 {
		for _, file := range files[start : end+1] {
			width = max(width, ansi.StringWidth(file))
		}
	}
	c.width = ordered.Clamp(width+2, int(minWidth), int(maxWidth))
	c.list.SetSize(c.width, c.height)
}

// Close closes the completions popup.
func (c *Completions) Close() {
	c.open = false
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

	// recalculate width by using just the visible items
	items := c.list.FilteredItems()
	start, end := c.list.VisibleItemIndices()
	width := 0
	if end != 0 {
		for _, item := range items[start : end+1] {
			width = max(width, ansi.StringWidth(item.(interface{ Text() string }).Text()))
		}
	}
	c.width = ordered.Clamp(width+2, int(minWidth), int(maxWidth))
	c.height = ordered.Clamp(len(items), int(minHeight), int(maxHeight))
	c.list.SetSize(c.width, c.height)
	c.list.SelectFirst()
	c.list.ScrollToSelected()
}

// HasItems returns whether there are visible items.
func (c *Completions) HasItems() bool {
	return len(c.list.FilteredItems()) > 0
}

// Update handles key events for the completions.
func (c *Completions) Update(msg tea.KeyPressMsg) (tea.Msg, bool) {
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
		c.Close()
		return ClosedMsg{}, true
	}

	return nil, false
}

// selectPrev selects the previous item with circular navigation.
func (c *Completions) selectPrev() {
	items := c.list.FilteredItems()
	if len(items) == 0 {
		return
	}
	if !c.list.SelectPrev() {
		c.list.WrapToEnd()
	}
	c.list.ScrollToSelected()
}

// selectNext selects the next item with circular navigation.
func (c *Completions) selectNext() {
	items := c.list.FilteredItems()
	if len(items) == 0 {
		return
	}
	if !c.list.SelectNext() {
		c.list.WrapToStart()
	}
	c.list.ScrollToSelected()
}

// selectCurrent returns a command with the currently selected item.
func (c *Completions) selectCurrent(insert bool) tea.Msg {
	items := c.list.FilteredItems()
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

	return SelectionMsg{
		Value:  item.Value(),
		Insert: insert,
	}
}

// Render renders the completions popup.
func (c *Completions) Render() string {
	if !c.open {
		return ""
	}

	items := c.list.FilteredItems()
	if len(items) == 0 {
		return ""
	}

	return c.list.Render()
}
