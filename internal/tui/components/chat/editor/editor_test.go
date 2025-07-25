package editor

import (
	"testing"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/session"
)

func TestHistoryNavigation(t *testing.T) {
	editor := New(&app.App{}).(*editorCmp)

	// Test empty history
	editor.navigateHistory(1)
	if editor.textarea.Value() != "" {
		t.Errorf("Expected empty textarea with empty history, got %q", editor.textarea.Value())
	}

	// Add some history items
	editor.addToHistory("first command")
	editor.addToHistory("second command")
	editor.addToHistory("third command")

	if len(editor.history) != 3 {
		t.Errorf("Expected 3 history items, got %d", len(editor.history))
	}

	// Test navigating up (to previous commands)
	editor.navigateHistory(1) // Should show "third command"
	if editor.textarea.Value() != "third command" {
		t.Errorf("Expected 'third command', got %q", editor.textarea.Value())
	}

	editor.navigateHistory(1) // Should show "second command"
	if editor.textarea.Value() != "second command" {
		t.Errorf("Expected 'second command', got %q", editor.textarea.Value())
	}

	editor.navigateHistory(1) // Should show "first command"
	if editor.textarea.Value() != "first command" {
		t.Errorf("Expected 'first command', got %q", editor.textarea.Value())
	}

	// Test navigating down (to newer commands)
	editor.navigateHistory(-1) // Should show "second command"
	if editor.textarea.Value() != "second command" {
		t.Errorf("Expected 'second command', got %q", editor.textarea.Value())
	}

	editor.navigateHistory(-1) // Should show "third command"
	if editor.textarea.Value() != "third command" {
		t.Errorf("Expected 'third command', got %q", editor.textarea.Value())
	}
}

func TestAddToHistory(t *testing.T) {
	editor := New(&app.App{}).(*editorCmp)

	// Test adding normal commands
	editor.addToHistory("command1")
	editor.addToHistory("command2")

	if len(editor.history) != 2 {
		t.Errorf("Expected 2 history items, got %d", len(editor.history))
	}

	// Test adding duplicate command (should not be added)
	editor.addToHistory("command2")
	if len(editor.history) != 2 {
		t.Errorf("Expected 2 history items after duplicate, got %d", len(editor.history))
	}

	// Test adding empty command (should not be added)
	editor.addToHistory("")
	editor.addToHistory("   ")
	if len(editor.history) != 2 {
		t.Errorf("Expected 2 history items after empty commands, got %d", len(editor.history))
	}
}

func TestHistoryLoadedMsg(t *testing.T) {
	editor := New(&app.App{}).(*editorCmp)
	editor.session = session.Session{ID: "test-session"}

	// Simulate loading history from database
	historyMsg := historyLoadedMsg{
		history: []string{"old command 1", "old command 2"},
	}

	// Process the message
	_, _ = editor.Update(historyMsg)

	if len(editor.history) != 2 {
		t.Errorf("Expected 2 history items after loading, got %d", len(editor.history))
	}

	if !editor.historyLoaded {
		t.Error("Expected historyLoaded to be true after loading")
	}

	if editor.historyIndex != -1 {
		t.Errorf("Expected historyIndex to be -1 after loading, got %d", editor.historyIndex)
	}
}
