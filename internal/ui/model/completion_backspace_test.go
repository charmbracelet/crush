package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/textarea"
)

func newCompletionBackspaceUI() *UI {
	com := common.DefaultCommon(&slashCommandWorkspace{ready: true})
	ta := textarea.New()
	ta.Focus()
	return &UI{
		com:      com,
		dialog:   dialog.NewOverlay(),
		status:   NewStatus(com, nil),
		chat:     NewChat(com, config.ScrollbarDefault),
		textarea: ta,
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

	// Go through the real insertion path so the recorded range and text
	// cannot drift from what the implementation actually writes.
	insertTestCompletion(t, m, "hello ", "@path/to/file.go")

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

	insertTestCompletion(t, m, "hello ", "@path/to/file.go")

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

	insertTestCompletion(t, m, "hello ", "@path/to/file.go")

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

// insertTestCompletion drives the real completion-insert path: it seeds
// the prompt with prefix, points the completion machinery at the end of
// it, and inserts text exactly as selecting from the popup would.
func insertTestCompletion(t *testing.T, m *UI, prefix, text string) {
	t.Helper()
	m.textarea.SetValue(prefix)
	m.completionsStartIndex = len(prefix)
	if !m.insertCompletionText(text) {
		t.Fatalf("insertCompletionText(%q) failed", text)
	}
	if got, want := m.textarea.Value(), prefix+text+" "; got != want {
		t.Fatalf("setup produced %q, want %q", got, want)
	}
}

// TestAtomicBackspaceIgnoresStaleRangeOfEqualLength is the regression
// test for the range being trusted on length alone. Recalling a history
// entry replaces the whole value; if it happens to be the same length as
// the completion that preceded it, a backspace would have cut an
// unrelated span out of the middle of it.
func TestAtomicBackspaceIgnoresStaleRangeOfEqualLength(t *testing.T) {
	t.Parallel()

	m := newCompletionBackspaceUI()
	insertTestCompletion(t, m, "hello ", "@path/to/file.go")

	// Same length, entirely different content — as a history recall would be.
	stale := "totally different promp"
	if len(stale) != len(m.textarea.Value()) {
		t.Fatalf("test needs an equal-length replacement: %d vs %d", len(stale), len(m.textarea.Value()))
	}
	m.textarea.SetValue(stale)
	m.textarea.MoveToEnd() // history recall leaves the cursor at the end

	m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})

	if got := m.textarea.Value(); got != "totally different prom" {
		t.Fatalf("expected a normal single-character backspace, got %q", got)
	}
	if m.lastCompletionEnd != 0 {
		t.Fatal("expected the stale range to be cleared")
	}
}
