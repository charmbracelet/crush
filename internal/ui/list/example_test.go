package list_test

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/list"
	uv "github.com/charmbracelet/ultraviolet"
)

// Example demonstrates basic list usage with string items.
func Example_basic() {
	// Create some items
	items := []list.Item{
		list.NewStringItem("First item"),
		list.NewStringItem("Second item"),
		list.NewStringItem("Third item"),
	}

	// Create a list with options
	l := list.New(items...)
	l.SetSize(80, 10)
	l.SetSelected(0)
	if true {
		l.Focus()
	}

	// Draw to a screen buffer
	screen := uv.NewScreenBuffer(80, 10)
	area := uv.Rect(0, 0, 80, 10)
	l.Draw(&screen, area)

	// Render to string
	output := screen.Render()
	fmt.Println(output)
}

// BorderedItem demonstrates a focusable item with borders.
type BorderedItem struct {
	id      string
	content string
	focused bool
	width   int
}

func NewBorderedItem(id, content string) *BorderedItem {
	return &BorderedItem{
		id:      id,
		content: content,
		width:   80,
	}
}

func (b *BorderedItem) ID() string {
	return b.id
}

func (b *BorderedItem) Height(width int) int {
	// Account for border (2 lines for top and bottom)
	b.width = width // Update width for rendering
	return lipgloss.Height(b.render())
}

func (b *BorderedItem) Draw(scr uv.Screen, area uv.Rectangle) {
	rendered := b.render()
	styled := uv.NewStyledString(rendered)
	styled.Draw(scr, area)
}

func (b *BorderedItem) render() string {
	style := lipgloss.NewStyle().
		Width(b.width-4).
		Padding(0, 1)

	if b.focused {
		style = style.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205"))
	} else {
		style = style.
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))
	}

	return style.Render(b.content)
}

func (b *BorderedItem) Focus() {
	b.focused = true
}

func (b *BorderedItem) Blur() {
	b.focused = false
}

func (b *BorderedItem) IsFocused() bool {
	return b.focused
}

// Example demonstrates focusable items with borders.
func Example_focusable() {
	// Create focusable items
	items := []list.Item{
		NewBorderedItem("1", "Focusable Item 1"),
		NewBorderedItem("2", "Focusable Item 2"),
		NewBorderedItem("3", "Focusable Item 3"),
	}

	// Create list with first item selected and focused
	l := list.New(items...)
	l.SetSize(80, 20)
	l.SetSelected(0)
	if true {
		l.Focus()
	}

	// Draw to screen
	screen := uv.NewScreenBuffer(80, 20)
	area := uv.Rect(0, 0, 80, 20)
	l.Draw(&screen, area)

	// The first item will have a colored border since it's focused
	output := screen.Render()
	fmt.Println(output)
}

// Example demonstrates dynamic item updates.
func Example_dynamicUpdates() {
	items := []list.Item{
		list.NewStringItem("Item 1"),
		list.NewStringItem("Item 2"),
	}

	l := list.New(items...)
	l.SetSize(80, 10)

	// Draw initial state
	screen := uv.NewScreenBuffer(80, 10)
	area := uv.Rect(0, 0, 80, 10)
	l.Draw(&screen, area)

	// Update an item
	l.UpdateItem(2, list.NewStringItem("Updated Item 2"))

	// Draw again - only changed item is re-rendered
	l.Draw(&screen, area)

	// Append a new item
	l.AppendItem(list.NewStringItem("New Item 3"))

	// Draw again - master buffer grows efficiently
	l.Draw(&screen, area)

	output := screen.Render()
	fmt.Println(output)
}

// Example demonstrates scrolling with a large list.
func Example_scrolling() {
	// Create many items
	items := make([]list.Item, 100)
	for i := range items {
		items[i] = list.NewStringItem(
			fmt.Sprintf("Item %d", i),
		)
	}

	// Create list with small viewport
	l := list.New(items...)
	l.SetSize(80, 10)
	l.SetSelected(0)

	// Draw initial view (shows items 0-9)
	screen := uv.NewScreenBuffer(80, 10)
	area := uv.Rect(0, 0, 80, 10)
	l.Draw(&screen, area)

	// Scroll down
	l.ScrollBy(5)
	l.Draw(&screen, area) // Now shows items 5-14

	// Jump to specific item
	l.ScrollToItem(50)
	l.Draw(&screen, area) // Now shows item 50 and neighbors

	// Scroll to bottom
	l.ScrollToBottom()
	l.Draw(&screen, area) // Now shows last 10 items

	output := screen.Render()
	fmt.Println(output)
}

// VariableHeightItem demonstrates items with different heights.
type VariableHeightItem struct {
	id    string
	lines []string
	width int
}

func NewVariableHeightItem(id string, lines []string) *VariableHeightItem {
	return &VariableHeightItem{
		id:    id,
		lines: lines,
		width: 80,
	}
}

func (v *VariableHeightItem) ID() string {
	return v.id
}

func (v *VariableHeightItem) Height(width int) int {
	return len(v.lines)
}

func (v *VariableHeightItem) Draw(scr uv.Screen, area uv.Rectangle) {
	content := ""
	for i, line := range v.lines {
		if i > 0 {
			content += "\n"
		}
		content += line
	}
	styled := uv.NewStyledString(content)
	styled.Draw(scr, area)
}

// Example demonstrates variable height items.
func Example_variableHeights() {
	items := []list.Item{
		NewVariableHeightItem("1", []string{"Short item"}),
		NewVariableHeightItem("2", []string{
			"This is a taller item",
			"that spans multiple lines",
			"to demonstrate variable heights",
		}),
		NewVariableHeightItem("3", []string{"Another short item"}),
		NewVariableHeightItem("4", []string{
			"A medium height item",
			"with two lines",
		}),
	}

	l := list.New(items...)
	l.SetSize(80, 15)

	screen := uv.NewScreenBuffer(80, 15)
	area := uv.Rect(0, 0, 80, 15)
	l.Draw(&screen, area)

	output := screen.Render()
	fmt.Println(output)
}

// Example demonstrates markdown items in a list.
func Example_markdown() {
	// Create markdown items
	items := []list.Item{
		list.NewMarkdownItem("# Welcome\n\nThis is a **markdown** item."),
		list.NewMarkdownItem("## Features\n\n- Supports **bold**\n- Supports *italic*\n- Supports `code`"),
		list.NewMarkdownItem("### Code Block\n\n```go\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```"),
	}

	// Create list
	l := list.New(items...)
	l.SetSize(80, 20)

	screen := uv.NewScreenBuffer(80, 20)
	area := uv.Rect(0, 0, 80, 20)
	l.Draw(&screen, area)

	output := screen.Render()
	fmt.Println(output)
}
