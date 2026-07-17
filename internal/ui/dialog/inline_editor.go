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

	// Height returns the number of content lines for layout at the
	// given content width. It must be a pure function of width so
	// layout stays in sync with Draw during resize.
	Height(width int) int

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

// CollapseInlineMsg asks the UI to move focus away from the active inline
// editor without dismissing it. The editor remains available for restoration.
type CollapseInlineMsg struct{}

// CollapsibleInlineEditor is implemented by inline editors that provide a
// compact representation while the chat has focus.
type CollapsibleInlineEditor interface {
	InlineEditor

	// ShouldCollapse reports whether the compact representation should be used
	// for the given editor width and terminal height while the editor is blurred.
	ShouldCollapse(width, terminalHeight int) bool

	// CollapsedHeight returns the compact representation's content height.
	CollapsedHeight() int

	// DrawCollapsed renders the compact representation.
	DrawCollapsed(scr uv.Screen, area uv.Rectangle)

	// CollapsedHelp returns the help description for returning focus to the
	// editor, such as "review plan" or "answer questions".
	CollapsedHelp() string
}

// ResizableInlineEditor is implemented by inline editors whose internal
// layout depends on the width allocated by the UI.
type ResizableInlineEditor interface {
	InlineEditor

	// SetWidth updates the editor's available content width.
	SetWidth(width int)
}

// CmdOnDone is an optional interface for inline editors that need to
// run a tea.Cmd when dismissed via mouse. The UI checks for this after
// HandleMouseClick returns done=true and queues the returned cmd.
type CmdOnDone interface {
	PendingCmd() tea.Cmd
}

// MouseClickableEditor is an optional interface for inline editors
// that handle mouse clicks and hover highlighting. The UI
// type-asserts for this before routing click and motion events.
type MouseClickableEditor interface {
	InlineEditor
	// HandleMouseClick processes a mouse click at the given screen
	// coordinates. Returns done=true when the editor has completed
	// (answer submitted or dismissed), and handled=true if the click
	// was consumed (even if not done).
	HandleMouseClick(x, y int) (done bool, handled bool)
	// SetHover updates the current mouse position for hover
	// highlighting. Called on every MouseMotionMsg while the
	// editor is active.
	SetHover(x, y int)
}

// MouseSelectableEditor is implemented by inline editors that support mouse
// drag selection within their own content.
type MouseSelectableEditor interface {
	InlineEditor
	// HandleMouseDown begins a selection when the press lands in selectable
	// content.
	HandleMouseDown(x, y int) bool
	// HandleMouseDrag updates an active selection.
	HandleMouseDrag(x, y int) bool
	// HandleMouseRelease finishes an active selection and optionally returns a
	// command that copies it.
	HandleMouseRelease(x, y int) (handled bool, cmd tea.Cmd)
}

// PasteableEditor is an optional interface for inline editors
// that contain text areas and can receive paste events. The UI
// type-asserts for this before routing tea.PasteMsg.
type PasteableEditor interface {
	// HandlePaste processes a paste message. Returns an optional
	// tea.Cmd for side effects (e.g., focus commands).
	HandlePaste(msg tea.PasteMsg) tea.Cmd
}
