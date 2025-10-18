package editor

import (
	"testing"

	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/charmbracelet/crush/internal/commandhistory"
	"github.com/charmbracelet/crush/internal/session"
)

func TestCommandHistoryNavigation(t *testing.T) {
	// Create test editor with properly initialized textarea
	editor := &editorCmp{
		history: []commandhistory.CommandHistory{
			{Command: "first command"},
			{Command: "second command"},
			{Command: "third command"},
		},
		historyIndex:    3,
		isInHistoryMode: false,
	}

	// Initialize textarea properly
	editor.textarea = textarea.New()
	editor.textarea.SetValue("current input")
	editor.textarea.Focus()

	// Test up arrow navigation - should go to most recent (third command)
	cmd := editor.navigateHistory(-1)
	if cmd != nil {
		t.Error("Expected nil cmd for navigation")
	}

	if !editor.isInHistoryMode {
		t.Error("Expected to be in history mode")
	}

	// The first up should show "third command" (most recent)
	if editor.textarea.Value() != "third command" {
		t.Errorf("Expected 'third command', got '%s'", editor.textarea.Value())
	}

	// Test another up arrow - should go to second command
	cmd = editor.navigateHistory(-1)
	if cmd != nil {
		t.Error("Expected nil cmd for navigation")
	}

	if editor.textarea.Value() != "second command" {
		t.Errorf("Expected 'second command', got '%s'", editor.textarea.Value())
	}

	// Test another up arrow - should go to first command
	cmd = editor.navigateHistory(-1)
	if cmd != nil {
		t.Error("Expected nil cmd for navigation")
	}

	if editor.textarea.Value() != "first command" {
		t.Errorf("Expected 'first command', got '%s'", editor.textarea.Value())
	}

	// Test down arrow - should go to second command
	cmd = editor.navigateHistory(1)
	if cmd != nil {
		t.Error("Expected nil cmd for navigation")
	}

	if editor.textarea.Value() != "second command" {
		t.Errorf("Expected 'second command', got '%s'", editor.textarea.Value())
	}

	// Test down arrow - should go to third command
	cmd = editor.navigateHistory(1)
	if cmd != nil {
		t.Error("Expected nil cmd for navigation")
	}

	if editor.textarea.Value() != "third command" {
		t.Errorf("Expected 'third command', got '%s'", editor.textarea.Value())
	}

	// Test down arrow - should exit history mode and return to current input
	cmd = editor.navigateHistory(1)
	if cmd != nil {
		t.Error("Expected nil cmd for navigation")
	}

	if editor.isInHistoryMode {
		t.Error("Expected to exit history mode")
	}

	if editor.textarea.Value() != "current input" {
		t.Errorf("Expected 'current input', got '%s'", editor.textarea.Value())
	}
}

func TestAddToHistory(t *testing.T) {
	// Mock app and session for testing
	editor := &editorCmp{
		session: session.Session{ID: "test-session"},
	}

	// Test with empty command
	cmd := editor.addToHistory("")
	if cmd != nil {
		// Should return nil for empty command
		// In real implementation this would check session.ID and command content
		t.Error("Expected nil cmd for empty command")
	}

	// Test with valid command
	cmd = editor.addToHistory("test command")
	if cmd == nil {
		t.Error("Expected non-nil cmd for valid command")
	}
}

func TestSetSessionResetsHistory(t *testing.T) {
	editor := &editorCmp{
		history: []commandhistory.CommandHistory{
			{Command: "old command"},
		},
		historyIndex:    1,
		isInHistoryMode: true,
		tempInput:       "temp",
	}

	newSession := session.Session{ID: "new-session"}
	cmd := editor.SetSession(newSession)

	if editor.session.ID != "new-session" {
		t.Error("Expected session to be updated")
	}

	if len(editor.history) != 0 {
		t.Error("Expected history to be reset")
	}

	if editor.historyIndex != 0 {
		t.Error("Expected history index to be reset")
	}

	if editor.isInHistoryMode {
		t.Error("Expected history mode to be reset")
	}

	if editor.tempInput != "" {
		t.Error("Expected temp input to be reset")
	}

	if cmd == nil {
		t.Error("Expected non-nil cmd to load history")
	}
}