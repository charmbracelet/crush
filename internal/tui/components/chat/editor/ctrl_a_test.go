package editor

import (
	"testing"

	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/stretchr/testify/require"
)

// TestCtrlAKeyBinding tests that Ctrl+A key is properly bound and recognized
func TestCtrlAKeyBinding(t *testing.T) {
	t.Parallel()

	keyMap := DefaultEditorKeyMap()

	// Test that SelectAll binding exists and is enabled
	require.True(t, keyMap.SelectAll.Enabled(), "SelectAll should be enabled")
	require.Contains(t, keyMap.SelectAll.Keys(), "ctrl+a", "Should contain ctrl+a")
	require.Contains(t, keyMap.SelectAll.Keys(), "cmd+a", "Should contain cmd+a")
}

// TestSelectAllFunctionality tests that SelectAll works correctly with Unicode content
func TestSelectAllFunctionality(t *testing.T) {
	t.Parallel()

	// Create a textarea with Unicode content
	ta := textarea.New()
	ta.SetValue("🌟 Hello World 🌍")

	// Create selection manager
	sm := NewSelectionManager(ta)

	// Test SelectAll functionality
	sm.SelectAll()

	require.True(t, sm.HasSelection(), "Should have selection after SelectAll")
	require.Equal(t, "🌟 Hello World 🌍", sm.GetSelectedText(), "Should select all content including Unicode")
}
