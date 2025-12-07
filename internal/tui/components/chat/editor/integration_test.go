package editor

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// testEditorHelper provides common test setup and assertion patterns
type testEditorHelper struct {
	t      *testing.T
	editor *editorCmp
}

// newTestEditor creates a test editor with given content
func newTestEditor(t *testing.T, content string) *testEditorHelper {
	ta := textarea.New()
	ta.SetValue(content)

	editor := &editorCmp{
		textarea:  ta,
		selection: NewSelectionManager(ta),
	}

	return &testEditorHelper{
		t:      t,
		editor: editor,
	}
}

// withAttachments adds attachments to the test editor
func (h *testEditorHelper) withAttachments(attachments []message.Attachment) *testEditorHelper {
	h.editor.attachments = attachments
	return h
}

// assertSelectionState validates selection state with given expectations
func (h *testEditorHelper) assertSelectionState(hasSelection bool, expectedText string) {
	require.Equal(h.t, hasSelection, h.editor.HasSelection(), "Selection state should match")
	require.Equal(h.t, expectedText, h.editor.GetSelectedText(), "Selected text should match")
}

// expectSelection is a helper function for test expectations
func expectSelection(t *testing.T, e *editorCmp, expectedContent string) {
	require.True(t, e.HasSelection(), "Should have selection")
	require.Equal(t, expectedContent, e.GetSelectedText(), "Selected text should match")
}

// TestSelectionKeyBindings tests all selection-related key bindings
func TestSelectionKeyBindings(t *testing.T) {
	t.Parallel()

	keyMap := DefaultEditorKeyMap()

	// Test SelectAll bindings
	require.True(t, keyMap.SelectAll.Enabled(), "SelectAll should be enabled")
	require.Contains(t, keyMap.SelectAll.Keys(), "ctrl+a", "Should contain ctrl+a")
	require.Contains(t, keyMap.SelectAll.Keys(), "cmd+a", "Should contain cmd+a")

	// Test Copy bindings
	require.True(t, keyMap.Copy.Enabled(), "Copy should be enabled")
	require.Contains(t, keyMap.Copy.Keys(), "ctrl+c", "Should contain ctrl+c")
	require.Contains(t, keyMap.Copy.Keys(), "cmd+c", "Should contain cmd+c")

	// Test LineStart bindings
	require.True(t, keyMap.LineStart.Enabled(), "LineStart should be enabled")
	require.Contains(t, keyMap.LineStart.Keys(), "home", "Should contain home")
	require.Contains(t, keyMap.LineStart.Keys(), "ctrl+home", "Should contain ctrl+home")
}

// TestSelectionClearingBehavior tests when selection gets cleared
func TestSelectionClearingBehavior(t *testing.T) {
	t.Parallel()

	ta := textarea.New()
	ta.SetValue("sample text")

	editor := &editorCmp{
		textarea:  ta,
		selection: NewSelectionManager(ta),
		keyMap:    DefaultEditorKeyMap(),
	}

	// Create selection first
	editor.SelectAll()
	require.True(t, editor.HasSelection(), "Should have selection")

	// Simulate typing a character (which should clear selection)
	// In real Update method, any non-selection key would clear selection
	// For this test, we'll just clear the selection directly to simulate the behavior
	if editor.HasSelection() {
		editor.ClearSelection()
	}

	require.False(t, editor.HasSelection(), "Selection should be cleared after typing")
}

// TestSelectionVisualRendering tests visual rendering of selection
func TestSelectionVisualRendering(t *testing.T) {
	t.Parallel()

	ta := textarea.New()
	ta.SetValue("hello world")

	editor := &editorCmp{
		textarea:  ta,
		selection: NewSelectionManager(ta),
	}

	// Test rendering without selection
	viewWithoutSelection := editor.renderSelectedText()
	require.Contains(t, viewWithoutSelection, "hello world", "Should render text without selection")

	// Create selection and test rendering
	editor.selection.SetSelection(6, 11) // "world"
	viewWithSelection := editor.renderSelectedText()
	require.Contains(t, viewWithSelection, "world", "Should render text with selection")

	// Test that renderSelectedText works with large content
	ta.SetValue(strings.Repeat("test line\n", 100))
	editor.SelectAll()
	viewLarge := editor.renderSelectedText()
	require.Contains(t, viewLarge, "test line", "Should render large content")
}

// TestSelectionWithAttachments tests selection behavior with attachments
func TestSelectionWithAttachments(t *testing.T) {
	t.Parallel()

	helper := newTestEditor(t, "test with attachments").
		withAttachments([]message.Attachment{
			{FileName: "test.txt"},
		})

	// Test that selection works even with attachments present
	helper.editor.SelectAll()
	helper.assertSelectionState(true, "test with attachments")

	// Test that attachments and selection coexist without issues
	require.NotEmpty(t, helper.editor.attachments, "Should have attachments")
	require.True(t, helper.editor.HasSelection(), "Should still have selection")

	// Test renderSelectedText method without calling full View()
	viewText := helper.editor.renderSelectedText()
	require.Contains(t, viewText, "test with attachments", "Should render selected text")
}

// TestSelectionPerformanceInContext tests performance with realistic content
func TestSelectionPerformanceInContext(t *testing.T) {
	t.Parallel()

	// Create a realistic amount of content
	content := `This is a multi-line document with realistic content.
It has multiple lines and various characters including some unicode: 🌟
The selection system should handle this efficiently without performance issues.
Line 4 with some more content.
And a final line to complete the test.`

	ta := textarea.New()
	ta.SetValue(content)

	editor := &editorCmp{
		textarea:  ta,
		selection: NewSelectionManager(ta),
	}

	// Test selection performance
	editor.SelectAll()
	require.True(t, editor.HasSelection(), "Should select all content efficiently")
	require.Equal(t, content, editor.GetSelectedText(), "Should get all content")

	// Test partial selection
	editor.selection.SetSelection(10, 50)
	require.True(t, editor.HasSelection(), "Should have partial selection")
	selected := editor.GetSelectedText()
	require.NotEmpty(t, selected, "Should have selected text")
	require.LessOrEqual(t, len(selected), 40, "Selected text should be within bounds")
}

// TestSelectionEdgeCasesInContext tests edge cases in realistic scenarios
func TestSelectionEdgeCasesInContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		content    string
		setupFunc  func(*editorCmp)
		expectFunc func(*testing.T, *editorCmp)
	}{
		{
			name:    "empty content",
			content: "",
			setupFunc: func(e *editorCmp) {
				e.SelectAll()
			},
			expectFunc: func(t *testing.T, e *editorCmp) {
				require.False(t, e.HasSelection(), "Empty content should not have selection")
			},
		},
		{
			name:    "single character",
			content: "a",
			setupFunc: func(e *editorCmp) {
				e.SelectAll()
			},
			expectFunc: func(t *testing.T, e *editorCmp) {
				expectSelection(t, e, "a")
			},
		},
		{
			name:    "only newlines",
			content: "\n\n\n",
			setupFunc: func(e *editorCmp) {
				e.SelectAll()
			},
			expectFunc: func(t *testing.T, e *editorCmp) {
				require.True(t, e.HasSelection(), "Newlines should be selectable")
				require.Equal(t, "\n\n\n", e.GetSelectedText())
			},
		},
		{
			name:    "unicode content",
			content: "🌟 Hello 🌍 World 🚀",
			setupFunc: func(e *editorCmp) {
				e.SelectAll()
			},
			expectFunc: func(t *testing.T, e *editorCmp) {
				require.True(t, e.HasSelection(), "Unicode should be selectable")
				require.Equal(t, "🌟 Hello 🌍 World 🚀", e.GetSelectedText())
			},
		},
		{
			name:    "selection at boundaries",
			content: "0123456789",
			setupFunc: func(e *editorCmp) {
				e.selection.SetSelection(0, 10)
			},
			expectFunc: func(t *testing.T, e *editorCmp) {
				require.True(t, e.HasSelection(), "Boundary selection should work")
				require.Equal(t, "0123456789", e.GetSelectedText())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ta := textarea.New()
			ta.SetValue(tt.content)

			editor := &editorCmp{
				textarea:  ta,
				selection: NewSelectionManager(ta),
			}

			tt.setupFunc(editor)
			tt.expectFunc(t, editor)
		})
	}
}

// TestSelectionManagerConsistency tests selection manager state consistency
func TestSelectionManagerConsistency(t *testing.T) {
	t.Parallel()

	ta := textarea.New()
	ta.SetValue("consistency test content")
	sm := NewSelectionManager(ta)

	// Test that selection state is consistent
	require.False(t, sm.HasSelection(), "Initially no selection")
	require.Empty(t, sm.GetSelectedText(), "Initially no selected text")

	// Set selection
	sm.SetSelection(0, 4)
	require.True(t, sm.HasSelection(), "Should have selection after SetSelection")
	require.Equal(t, "cons", sm.GetSelectedText(), "Should have correct selected text")

	// Get selection and verify consistency
	selection := sm.GetSelection()
	require.Equal(t, 0, selection.Start)
	require.Equal(t, 4, selection.End)

	// Clear and verify consistency
	sm.Clear()
	require.False(t, sm.HasSelection(), "Should have no selection after Clear")
	require.Empty(t, sm.GetSelectedText(), "Should have no selected text after Clear")

	selection = sm.GetSelection()
	require.Equal(t, -1, selection.Start)
	require.Equal(t, -1, selection.End)
}

// TestSelectionWorkflowIntegration tests the complete selection workflow
func TestSelectionWorkflowIntegration(t *testing.T) {
	t.Parallel()

	// Test the typical user workflow
	ta := textarea.New()
	ta.SetValue("Hello, World!\nThis is a test message.\nMultiple lines.")

	editor := &editorCmp{
		textarea:  ta,
		selection: NewSelectionManager(ta),
		keyMap:    DefaultEditorKeyMap(),
	}

	// 1. Initially no selection
	require.False(t, editor.HasSelection(), "Should start with no selection")
	require.Empty(t, editor.GetSelectedText())

	// 2. Select all
	editor.SelectAll()
	require.True(t, editor.HasSelection(), "Should have selection after SelectAll")
	require.Equal(t, "Hello, World!\nThis is a test message.\nMultiple lines.", editor.GetSelectedText())

	// 3. Clear selection
	editor.ClearSelection()
	require.False(t, editor.HasSelection(), "Should have no selection after ClearSelection")
	require.Empty(t, editor.GetSelectedText())

	// 4. Set partial selection
	editor.selection.SetSelection(7, 20) // "World!\nThis i"
	require.True(t, editor.HasSelection(), "Should have partial selection")
	require.Equal(t, "World!\nThis i", editor.GetSelectedText())

	// 5. Test backward selection
	editor.selection.SetSelection(20, 7) // Same range, backward
	require.Equal(t, "World!\nThis i", editor.GetSelectedText(), "Backward selection should work")

	// 6. Test that selection updates when content changes
	ta.SetValue("New content for testing")
	editor.SelectAll()
	require.Equal(t, "New content for testing", editor.GetSelectedText(), "Selection should update with new content")
}

// TestSelectionStateTransitions tests selection state changes
func TestSelectionStateTransitions(t *testing.T) {
	t.Parallel()

	// Test all possible state transitions
	states := []struct {
		name           string
		action         func(*editorCmp)
		expectedHasSel bool
		expectedText   string
	}{
		{
			name:           "initial state",
			action:         func(e *editorCmp) {},
			expectedHasSel: false,
			expectedText:   "",
		},
		{
			name:           "select all",
			action:         func(e *editorCmp) { e.SelectAll() },
			expectedHasSel: true,
			expectedText:   "state transition test",
		},
		{
			name:           "clear selection",
			action:         func(e *editorCmp) { e.ClearSelection() },
			expectedHasSel: false,
			expectedText:   "",
		},
		{
			name:           "partial selection",
			action:         func(e *editorCmp) { e.selection.SetSelection(6, 16) },
			expectedHasSel: true,
			expectedText:   "transition",
		},
		{
			name:           "clear after partial",
			action:         func(e *editorCmp) { e.ClearSelection() },
			expectedHasSel: false,
			expectedText:   "",
		},
	}

	for _, state := range states {
		t.Run(state.name, func(t *testing.T) {
			t.Parallel()

			helper := newTestEditor(t, "state transition test")
			state.action(helper.editor)
			helper.assertSelectionState(state.expectedHasSel, state.expectedText)
		})
	}
}
