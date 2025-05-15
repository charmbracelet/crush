package utilComponents

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

type SimpleListItem interface {
	Render(selected bool, width int) string
}

type SimpleList[T SimpleListItem] interface {
	tea.Model
	layout.Bindings
	SetMaxWidth(maxWidth int)
	GetSelectedItem() (item T, idx int)
	SetItems(items []T)
	GetItems() []T
}

type simpleListCmp[T SimpleListItem] struct {
	fallbackMsg         string
	items               []T
	selectedIdx         int
	maxWidth            int
	maxVisibleItems     int
	useAlphaNumericKeys bool
	width               int
	height              int
}

type simpleListKeyMap struct {
	Up        key.Binding
	Down      key.Binding
	UpAlpha   key.Binding
	DownAlpha key.Binding
}

var simpleListKeys = simpleListKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "previous list item"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "next list item"),
	),
	UpAlpha: key.NewBinding(
		key.WithKeys("k"),
		key.WithHelp("k", "previous list item"),
	),
	DownAlpha: key.NewBinding(
		key.WithKeys("j"),
		key.WithHelp("j", "next list item"),
	),
}

func (c *simpleListCmp[T]) Init() tea.Cmd {
	return nil
}

func (c *simpleListCmp[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, simpleListKeys.Up) || (c.useAlphaNumericKeys && key.Matches(msg, simpleListKeys.UpAlpha)):
			if c.selectedIdx > 0 {
				c.selectedIdx--
			}
			return c, nil
		case key.Matches(msg, simpleListKeys.Down) || (c.useAlphaNumericKeys && key.Matches(msg, simpleListKeys.DownAlpha)):
			if c.selectedIdx < len(c.items)-1 {
				c.selectedIdx++
			}
			return c, nil
		}
	}

	return c, nil
}

func (c *simpleListCmp[T]) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(simpleListKeys)
}

func (c *simpleListCmp[T]) GetSelectedItem() (T, int) {
	if len(c.items) > 0 {
		return c.items[c.selectedIdx], c.selectedIdx
	}

	var zero T
	return zero, -1
}

func (c *simpleListCmp[T]) SetItems(items []T) {
	c.selectedIdx = 0
	c.items = items
}

func (c *simpleListCmp[T]) GetItems() []T {
	return c.items
}

func (c *simpleListCmp[T]) SetMaxWidth(width int) {
	c.maxWidth = width
}

func (c *simpleListCmp[T]) View() string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	items := c.items
	maxWidth := c.maxWidth
	maxVisibleItems := min(c.maxVisibleItems, len(items))
	startIdx := 0

	if len(items) <= 0 {
		return baseStyle.
			Background(t.Background()).
			Padding(0, 1).
			Width(maxWidth).
			Render(c.fallbackMsg)
	}

	if len(items) > maxVisibleItems {
		halfVisible := maxVisibleItems / 2
		if c.selectedIdx >= halfVisible && c.selectedIdx < len(items)-halfVisible {
			startIdx = c.selectedIdx - halfVisible
		} else if c.selectedIdx >= len(items)-halfVisible {
			startIdx = len(items) - maxVisibleItems
		}
	}

	endIdx := min(startIdx+maxVisibleItems, len(items))

	listItems := make([]string, 0, maxVisibleItems)

	for i := startIdx; i < endIdx; i++ {
		item := items[i]
		title := item.Render(i == c.selectedIdx, maxWidth)
		listItems = append(listItems, title)
	}

	return lipgloss.JoinVertical(lipgloss.Left, listItems...)
}

func NewSimpleList[T SimpleListItem](items []T, maxVisibleItems int, fallbackMsg string, useAlphaNumericKeys bool) SimpleList[T] {
	return &simpleListCmp[T]{
		fallbackMsg:         fallbackMsg,
		items:               items,
		maxVisibleItems:     maxVisibleItems,
		useAlphaNumericKeys: useAlphaNumericKeys,
		selectedIdx:         0,
	}
}
