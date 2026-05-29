// Package editor defines the bridge between Crush and an external editor
// process (currently Neovim) running on the same machine.
//
// Crush uses the bridge to:
//   - Pull live editor context (current file, cursor, selection) on demand.
//   - Flash highlights on lines Crush just edited.
//   - Notify the editor that a file changed on disk so it can reload its
//     buffer without prompting the user (W11 "file has been edited" warning).
//   - Display rich navigation pickers (e.g. "show locations") inside the
//     editor's UI rather than the chat transcript.
//
// The bridge is auto-detected at startup: if Crush sees an environment
// variable indicating it was spawned from inside an editor (e.g. $NVIM
// for Neovim's :terminal), it dials the editor over the editor's native
// RPC channel. Otherwise the no-op bridge is used and editor-only tools
// stay hidden.
package editor

import "context"

// Position is a 0-indexed cursor position in a buffer.
type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// EditorContext is the snapshot returned by Bridge.Context.
type EditorContext struct {
	// Path is the absolute path to the focused file, if any.
	Path string `json:"path,omitempty"`
	// URI is the LSP-style file URI form of Path.
	URI string `json:"uri,omitempty"`
	// Cursor is the cursor position in the focused buffer.
	Cursor Position `json:"cursor"`
	// ContextBefore holds up to N lines preceding the cursor.
	ContextBefore string `json:"context_before,omitempty"`
	// ContextLine is the line under the cursor.
	ContextLine string `json:"context_line,omitempty"`
	// ContextAfter holds up to N lines following the cursor.
	ContextAfter string `json:"context_after,omitempty"`
	// TotalLines is the line count of the focused buffer.
	TotalLines int `json:"total_lines,omitempty"`
	// HasSelection is true when the user has an active visual selection.
	HasSelection bool `json:"has_selection"`
	// Selection holds the selected text when HasSelection is true.
	Selection string `json:"selection,omitempty"`
}

// Location is one entry rendered by Bridge.ShowLocations.
type Location struct {
	Filename string `json:"filename"`
	Line     int    `json:"lnum"`
	Col      int    `json:"col,omitempty"`
	Text     string `json:"text"`
	Note     string `json:"note"`
	Type     string `json:"type,omitempty"`
}

// Bridge is the abstraction Crush uses to talk to an external editor.
//
// All methods are safe to call from any goroutine. Implementations must
// be cheap when Available reports false: in that mode the methods return
// nil errors after a no-op so callers do not need to gate every call.
type Bridge interface {
	// Available reports whether the bridge is connected to a live editor.
	// Tools that only make sense with an editor attached should hide
	// themselves when Available is false.
	Available() bool

	// Context returns a snapshot of the editor's current state. Pull-based
	// (no caching): each call fetches the latest cursor, selection, and
	// surrounding code from the editor.
	Context(ctx context.Context) (EditorContext, error)

	// ShowLocations asks the editor to display a navigable list of code
	// locations (e.g. via Telescope in Neovim).
	ShowLocations(ctx context.Context, title string, items []Location) error

	// FlashEdit briefly highlights a range Crush just modified. startLine
	// and endLine are 0-indexed; endLine is exclusive. Best-effort: a
	// failure here must never propagate to the user-facing tool result.
	FlashEdit(ctx context.Context, path string, startLine, endLine int) error

	// NotifyFileChanged tells the editor that path was modified on disk.
	// The editor reloads the buffer if it is loaded and unmodified,
	// suppressing the "file changed since editing started" prompt that
	// would otherwise appear when the user next focuses the buffer.
	NotifyFileChanged(ctx context.Context, path string) error

	// Close releases any resources (sockets, goroutines) held by the
	// bridge. Idempotent.
	Close() error
}

// Noop is a Bridge that reports unavailable and ignores every call. Used
// when Crush is not running under an editor or detection fails.
type Noop struct{}

// Available implements Bridge.
func (Noop) Available() bool { return false }

// Context implements Bridge.
func (Noop) Context(context.Context) (EditorContext, error) { return EditorContext{}, ErrUnavailable }

// ShowLocations implements Bridge.
func (Noop) ShowLocations(context.Context, string, []Location) error { return ErrUnavailable }

// FlashEdit implements Bridge.
func (Noop) FlashEdit(context.Context, string, int, int) error { return nil }

// NotifyFileChanged implements Bridge.
func (Noop) NotifyFileChanged(context.Context, string) error { return nil }

// Close implements Bridge.
func (Noop) Close() error { return nil }
