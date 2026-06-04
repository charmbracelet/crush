package dialog

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// InlineEditor is the interface for components that replace the
// textarea in the editor area. The UI model holds a single
// InlineEditor field and routes keys, rendering, layout, and help
// through it without knowing the concrete type.
type InlineEditor interface {
	// HandleKey processes a key event. Returns true when the user
	// has finished interacting (answer submitted or dismissed),
	// plus an optional tea.Cmd.
	HandleKey(msg tea.KeyPressMsg) (done bool, cmd tea.Cmd)

	// ShortHelp returns key bindings for the status bar.
	ShortHelp() []key.Binding

	// Height returns the number of content lines for layout.
	Height() int

	// Draw renders the component onto the screen within the given
	// area. Returns the cursor position relative to the area's
	// top-left, or nil if no cursor should be shown.
	Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor

	// HeightChanged reports whether the height changed since the
	// last call, indicating the UI should recalculate layout.
	HeightChanged() bool

	// SetFocused tells the component whether the editor area is
	// focused.
	SetFocused(focused bool)
}
