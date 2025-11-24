package list

import (
	"io"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/glamour/v2"
	"github.com/charmbracelet/glamour/v2/ansi"
)

// Item represents a rendered item in the [List] component.
type Item interface {
	// Content is the rendered content of the item.
	Content() string
	// Height returns the height of the item based on its content.
	Height() int
}

// Gap is [GapItem] to be used as a vertical gap in the list.
var Gap = GapItem{}

// GapItem represents a vertical gap in the list.
type GapItem struct{}

// Content returns the content of the gap item.
func (g GapItem) Content() string {
	return ""
}

// Height returns the height of the gap item.
func (g GapItem) Height() int {
	return 1
}

// StringItem represents a simple string item in the list.
type StringItem struct {
	content string
}

// NewStringItem creates a new [StringItem] with the given id and content.
func NewStringItem(content string) StringItem {
	return StringItem{
		content: content,
	}
}

// Content returns the content of the string item.
func (s StringItem) Content() string {
	return s.content
}

// Height returns the height of the string item based on its content.
func (s StringItem) Height() int {
	return lipgloss.Height(s.content)
}

// MarkdownItem represents a markdown item in the list.
type MarkdownItem struct {
	StringItem
}

// NewMarkdownItem creates a new [MarkdownItem] with the given id and content.
func NewMarkdownItem(id, content string) MarkdownItem {
	return MarkdownItem{
		StringItem: StringItem{
			content: content,
		},
	}
}

// Content returns the content of the markdown item.
func (m MarkdownItem) Content() string {
	return m.StringItem.Content()
}

// Height returns the height of the markdown item based on its content.
func (m MarkdownItem) Height() int {
	return m.StringItem.Height()
}

// MarkdownItemMaxWidth is the maximum width for rendering markdown items.
const MarkdownItemMaxWidth = 120

// MarkdownItemRenderer renders [MarkdownItem]s in a [List].
type MarkdownItemRenderer struct {
	Styles *ansi.StyleConfig
}

// Render implements [ItemRenderer].
func (m *MarkdownItemRenderer) Render(w io.Writer, list *List, index int, item Item) {
	width := min(list.Width(), MarkdownItemMaxWidth)
	var r *glamour.TermRenderer
	if m.Styles != nil {
		r = common.MarkdownRenderer(*m.Styles, width)
	} else {
		r = common.PlainMarkdownRenderer(width)
	}

	rendered, _ := r.Render(item.Content())
	_, _ = io.WriteString(w, rendered)
}
