package list

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Item represents a single item in the lazy-loaded list.
type Item interface {
	// Render returns the string representation of the item for the given
	// width.
	Render(width int) string
}

// Focusable represents an item that can be aware of focus state changes.
type Focusable interface {
	// SetFocused sets the focus state of the item.
	SetFocused(focused bool)
}

// Highlightable represents an item that can highlight a portion of its content.
type Highlightable interface {
	// Highlight highlights the content from the given start to end positions.
	// Use -1 for no highlight.
	Highlight(startLine, startCol, endLine, endCol int)
}

// MouseClickable represents an item that can handle mouse click events.
type MouseClickable interface {
	// HandleMouseClick processes a mouse click event at the given coordinates.
	// It returns true if the event was handled, false otherwise.
	HandleMouseClick(btn ansi.MouseButton, x, y int) bool
}

// SpacerItem is a spacer item that adds vertical space in the list.
type SpacerItem struct {
	Height int
}

// NewSpacerItem creates a new [SpacerItem] with the specified height.
func NewSpacerItem(height int) *SpacerItem {
	return &SpacerItem{
		Height: max(0, height-1),
	}
}

// Render implements the Item interface for [SpacerItem].
func (s *SpacerItem) Render(width int) string {
	return strings.Repeat("\n", s.Height)
}
