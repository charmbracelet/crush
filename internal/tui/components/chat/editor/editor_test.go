package editor

import (
	"testing"

	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/stretchr/testify/require"
)

func TestEditorSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		initialValue  string
		expectedAfter string
		operation     func(*editorCmp)
	}{
		{
			name:          "select all empty text",
			initialValue:  "",
			operation:     func(e *editorCmp) { e.SelectAll() },
			expectedAfter: "",
		},
		{
			name:          "select all single line",
			initialValue:  "hello world",
			operation:     func(e *editorCmp) { e.SelectAll() },
			expectedAfter: "hello world",
		},
		{
			name:         "clear selection after select all",
			initialValue: "test content",
			operation: func(e *editorCmp) {
				e.SelectAll()
				e.ClearSelection()
			},
			expectedAfter: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create real textarea model
			ta := textarea.New()
			ta.SetValue(tt.initialValue)

			// Create editor instance with real textarea
			editor := &editorCmp{
				textarea:  ta,
				selection: NewSelectionManager(ta),
			}

			// Perform operation
			tt.operation(editor)

			// Check result
			if tt.expectedAfter == "" {
				require.False(t, editor.HasSelection(), "Should have no selection")
			} else {
				require.True(t, editor.HasSelection(), "Should have selection")
				selectedText := editor.GetSelectedText()
				require.Equal(t, tt.expectedAfter, selectedText, "Selected text should match")
			}
		})
	}
}

func TestEditorGetSelectedText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		initialValue   string
		selectionStart int
		selectionEnd   int
		expectedText   string
	}{
		{
			name:           "no selection (start=end)",
			initialValue:   "hello",
			selectionStart: 2,
			selectionEnd:   2,
			expectedText:   "",
		},
		{
			name:           "no selection (negative values)",
			initialValue:   "hello",
			selectionStart: -1,
			selectionEnd:   -1,
			expectedText:   "",
		},
		{
			name:           "partial selection forward",
			initialValue:   "hello world",
			selectionStart: 2,
			selectionEnd:   7,
			expectedText:   "llo w",
		},
		{
			name:           "partial selection backward",
			initialValue:   "hello world",
			selectionStart: 7,
			selectionEnd:   2,
			expectedText:   "llo w",
		},
		{
			name:           "select all",
			initialValue:   "test content",
			selectionStart: 0,
			selectionEnd:   12,
			expectedText:   "test content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create real textarea model
			ta := textarea.New()
			ta.SetValue(tt.initialValue)
			sm := NewSelectionManager(ta)

			editor := &editorCmp{
				textarea:  ta,
				selection: sm,
			}

			// Set selection
			sm.SetSelection(tt.selectionStart, tt.selectionEnd)

			selectedText := editor.GetSelectedText()
			require.Equal(t, tt.expectedText, selectedText, "Selected text should match")
		})
	}
}

// Test that editorCmp implements Editor interface properly
func TestEditorInterfaceImplementation(t *testing.T) {
	t.Parallel()

	// Type assertion test - will fail at compile time if interface not implemented
	var _ Editor = &editorCmp{}
}

// Test key bindings
func TestEditorKeyBindings(t *testing.T) {
	t.Parallel()

	keyMap := DefaultEditorKeyMap()

	require.True(t, keyMap.SelectAll.Enabled(), "SelectAll key should be enabled")
	require.True(t, keyMap.Copy.Enabled(), "Copy key should be enabled")
	require.True(t, keyMap.LineStart.Enabled(), "LineStart key should be enabled")

	// Check that keys are bound to expected keys
	require.Contains(t, keyMap.SelectAll.Keys(), "ctrl+a")
	require.Contains(t, keyMap.SelectAll.Keys(), "cmd+a")
	require.Contains(t, keyMap.Copy.Keys(), "ctrl+c")
	require.Contains(t, keyMap.Copy.Keys(), "cmd+c")
	require.Contains(t, keyMap.LineStart.Keys(), "home")
	require.Contains(t, keyMap.LineStart.Keys(), "ctrl+home")
}

// Test that editor can be created without panics
func TestNewEditor(t *testing.T) {
	t.Parallel()

	// Skip this test for now as it requires full app setup
	// In a real environment, you'd need to properly initialize the app
	t.Skip("Skipping New editor test - requires full app setup")
}

// Integration tests for the complete selection system
func TestEditorSelectionIntegration(t *testing.T) {
	t.Parallel()

	// Test the full selection workflow
	ta := textarea.New()
	ta.SetValue("Hello, world!\nThis is a test.\nMultiple lines.")

	editor := &editorCmp{
		textarea:  ta,
		selection: NewSelectionManager(ta),
		keyMap:    DefaultEditorKeyMap(),
	}

	// Initially no selection
	require.False(t, editor.HasSelection(), "Should start with no selection")
	require.Empty(t, editor.GetSelectedText(), "Selected text should be empty")

	// Select all
	editor.SelectAll()
	require.True(t, editor.HasSelection(), "Should have selection after SelectAll")
	require.Equal(t, "Hello, world!\nThis is a test.\nMultiple lines.", editor.GetSelectedText(), "Selected text should be all content")

	// Clear selection
	editor.ClearSelection()
	require.False(t, editor.HasSelection(), "Should have no selection after ClearSelection")
	require.Empty(t, editor.GetSelectedText(), "Selected text should be empty after ClearSelection")

	// Test partial selection
	editor.selection.SetSelection(7, 12)
	require.True(t, editor.HasSelection(), "Should have selection")
	require.Equal(t, "world", editor.GetSelectedText(), "Selected text should match partial selection")

	// Test backward selection
	editor.selection.SetSelection(12, 7)
	require.Equal(t, "world", editor.GetSelectedText(), "Selected text should work backward")

	// Test that selection updates when textarea content changes
	ta.SetValue("New content")
	editor.SelectAll()
	require.Equal(t, "New content", editor.GetSelectedText(), "Selection should update with new content")
}

// Test key handling integration
func TestEditorKeyHandling(t *testing.T) {
	t.Parallel()

	ta := textarea.New()
	ta.SetValue("Sample text for testing.")

	editor := &editorCmp{
		textarea:  ta,
		selection: NewSelectionManager(ta),
		keyMap:    DefaultEditorKeyMap(),
	}

	// Test that Ctrl+A triggers SelectAll
	// Note: In real scenario, this would be tested via tea.KeyPressMsg
	// Here we test SelectAll method directly
	editor.SelectAll()
	require.True(t, editor.HasSelection(), "SelectAll should create selection")
	require.Equal(t, "Sample text for testing.", editor.GetSelectedText(), "Should select all text")

	// Test that ClearSelection works
	editor.ClearSelection()
	require.False(t, editor.HasSelection(), "ClearSelection should remove selection")
	require.Empty(t, editor.GetSelectedText(), "Selected text should be empty")

	// Test setting partial selection via SelectionManager
	editor.selection.SetSelection(7, 11)
	require.True(t, editor.HasSelection(), "Should have partial selection")
	require.Equal(t, "text", editor.GetSelectedText(), "Should have correct partial text")

	// Test that selection clearing works with any action
	// In real Update method, any non-selection key should clear selection
	// Here we simulate that behavior
	if editor.HasSelection() {
		editor.ClearSelection()
	}
	require.False(t, editor.HasSelection(), "Selection should be cleared")
}

// Test edge cases and error handling
func TestEditorSelectionEdgeCases(t *testing.T) {
	t.Parallel()

	// Test with empty textarea
	ta := textarea.New()
	editor := &editorCmp{
		textarea:  ta,
		selection: NewSelectionManager(ta),
	}

	// Select all on empty text
	editor.SelectAll()
	require.False(t, editor.HasSelection(), "Empty text should have no selection")
	require.Empty(t, editor.GetSelectedText(), "Selected text should be empty")

	// Test with single character
	ta.SetValue("a")
	editor.SelectAll()
	require.True(t, editor.HasSelection(), "Single character should have selection")
	require.Equal(t, "a", editor.GetSelectedText(), "Should select single character")

	// Test with unicode/multibyte characters
	ta.SetValue("🌟 Hello 🌍")
	editor.SelectAll()
	require.True(t, editor.HasSelection(), "Unicode text should have selection")
	require.Equal(t, "🌟 Hello 🌍", editor.GetSelectedText(), "Should select unicode text correctly")

	// Test setting selection boundaries
	ta.SetValue("0123456789")
	editor.selection.SetSelection(-5, 15) // Out of bounds
	require.Empty(t, editor.GetSelectedText(), "Out of bounds selection should be empty")

	// Test zero-length selection
	editor.selection.SetSelection(5, 5)
	require.False(t, editor.HasSelection(), "Zero-length selection should not be active")
	require.Empty(t, editor.GetSelectedText(), "Zero-length selection should return empty")
}
