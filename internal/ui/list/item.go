package list

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Item represents a single item in the lazy-loaded list.
type Item interface {
	// Render returns the string representation of the item for the given
	// width.
	Render(width int) string
}

// FocusStylable represents an item that can be styled based on focus state.
type FocusStylable interface {
	// FocusStyle returns the style to apply when the item is focused.
	FocusStyle() lipgloss.Style
	// BlurStyle returns the style to apply when the item is unfocused.
	BlurStyle() lipgloss.Style
}

// HighlightStylable represents an item that can be styled for highlighted regions.
type HighlightStylable interface {
	// HighlightStyle returns the style to apply for highlighted regions.
	HighlightStyle() lipgloss.Style
}

// MouseClickable represents an item that can handle mouse click events.
type MouseClickable interface {
	// HandleMouseClick processes a mouse click event at the given coordinates.
	// It returns true if the event was handled, false otherwise.
	HandleMouseClick(btn ansi.MouseButton, x, y int) bool
}

// FocusAware represents an item that needs to know its focus state for
// internal rendering decisions.
type FocusAware interface {
	// SetFocused is called before Render to inform the item of its focus state.
	SetFocused(focused bool)
}
