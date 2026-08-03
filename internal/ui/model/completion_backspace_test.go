package model

import (
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
)

func newCompletionBackspaceUI() *UI {
	com := common.DefaultCommon(&slashCommandWorkspace{ready: true})
	return &UI{
		com:      com,
		dialog:   dialog.NewOverlay(),
		status:   NewStatus(com, nil),
		chat:     NewChat(com, config.ScrollbarDefault),
		textarea: textarea.New(),
		attachments: attachments.New(
			attachments.NewRenderer(
				lipgloss.NewStyle(), lipgloss.NewStyle(),
				lipgloss.NewStyle(), lipgloss.NewStyle(),
				lipgloss.NewStyle(), lipgloss.NewStyle(),
			),
			attachments.Keymap{},
		),
		state:  uiChat,
		focus:  uiFocusEditor,
		width:  140,
		height: 45,
	}
}

// TestAtomicBackspaceDeletesEntireCompletion verifies that pressing
// backspace immediately after a completion insertion deletes the entire
// inserted text at once rather than one character at a time.
func TestAtomicBackspaceDeletesEntireCompletion(t *testing.T) {
	t.Parallel()

	m := newCompletionBackspaceUI()

	// Simulate what insertCompletionText does: set the text and record
	// the range.
	m.textarea.SetValue("hello @path/to/file.go ")
	m.lastCompletionStart = 6 // index of '@'
	m.lastCompletionEnd = 23  // end of inserted text + trailing space

	// Press backspace — should delete the entire completion.
	m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})

	got := m.textarea.Value()
	if got != "hello " {
		t.Fatalf("expected %q after atomic backspace, got %q", "hello ", got)
	}
	if m.lastCompletionEnd != 0 {
		t.Fatal("expected lastCompletionEnd to be cleared after atomic delete")
	}
}

// TestAtomicBackspaceInvalidatedByTyping verifies that typing any
// character after completion clears the atomic-delete range so
// subsequent backspaces behave normally.
func TestAtomicBackspaceInvalidatedByTyping(t *testing.T) {
	t.Parallel()

	m := newCompletionBackspaceUI()

	m.textarea.SetValue("hello @path/to/file.go ")
	m.lastCompletionStart = 6
	m.lastCompletionEnd = 23

	// Type a character — should invalidate the range.
	m.Update(tea.KeyPressMsg{Code: 'x'})

	if m.lastCompletionEnd != 0 {
		t.Fatal("expected lastCompletionEnd to be cleared after typing")
	}
}

// TestAtomicBackspaceInvalidatedByTextChange verifies that if the
// textarea value changes between insertion and backspace (e.g. user
// clicked elsewhere and edited), the atomic delete is skipped.
func TestAtomicBackspaceInvalidatedByTextChange(t *testing.T) {
	t.Parallel()

	m := newCompletionBackspaceUI()

	m.textarea.SetValue("hello @path/to/file.go ")
	m.lastCompletionStart = 6
	m.lastCompletionEnd = 23

	// Simulate text changing (e.g. user pasted or deleted something).
	m.textarea.SetValue("hello @path/to/file.go extra")

	// Press backspace — should NOT do atomic delete since length changed.
	m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})

	got := m.textarea.Value()
	// The textarea's normal backspace should have deleted the last char.
	if got == "hello " {
		t.Fatal("atomic backspace should not have fired after text change")
	}
	if m.lastCompletionEnd != 0 {
		t.Fatal("expected lastCompletionEnd to be cleared after invalidated backspace")
	}
}
