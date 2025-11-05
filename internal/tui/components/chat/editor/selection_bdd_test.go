package editor

import (
	"testing"

	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Behavior-driven development tests for selection system
// These tests describe the expected behavior from a user's perspective

// GivenAndWhenScenarios encapsulates BDD test scenarios
type GivenAndWhenScenarios struct {
	description string
	given       func() *SelectionManager
	when        func(*SelectionManager)
	then        func(*SelectionManager, *testing.T)
}

// TestSelectionBehavior implements comprehensive BDD scenarios
func TestSelectionBehavior(t *testing.T) {
	t.Parallel()

	scenarios := []GivenAndWhenScenarios{
		{
			description: "empty textarea should have no selection",
			given: func() *SelectionManager {
				ta := mockTextarea("")
				return NewSelectionManager(ta)
			},
			when: func(sm *SelectionManager) {
				// No action
			},
			then: func(sm *SelectionManager, t *testing.T) {
				assert.False(t, sm.HasSelection(), "Empty textarea should have no selection")
				assert.Empty(t, sm.GetSelectedText(), "Selected text should be empty")
			},
		},
		{
			description: "selecting all text should select entire content",
			given: func() *SelectionManager {
				ta := mockTextarea("hello world")
				return NewSelectionManager(ta)
			},
			when: func(sm *SelectionManager) {
				sm.SelectAll()
			},
			then: func(sm *SelectionManager, t *testing.T) {
				assert.True(t, sm.HasSelection(), "Should have selection after SelectAll")
				assert.Equal(t, "hello world", sm.GetSelectedText(), "Should select entire content")
			},
		},
		{
			description: "clearing selection should remove all selection",
			given: func() *SelectionManager {
				ta := mockTextarea("test content")
				sm := NewSelectionManager(ta)
				sm.SelectAll()
				return sm
			},
			when: func(sm *SelectionManager) {
				sm.Clear()
			},
			then: func(sm *SelectionManager, t *testing.T) {
				assert.False(t, sm.HasSelection(), "Should have no selection after Clear")
				assert.Empty(t, sm.GetSelectedText(), "Selected text should be empty after Clear")
			},
		},
		{
			description: "setting valid selection should create proper selection",
			given: func() *SelectionManager {
				ta := mockTextarea("hello world")
				return NewSelectionManager(ta)
			},
			when: func(sm *SelectionManager) {
				sm.SetSelection(6, 11) // "world"
			},
			then: func(sm *SelectionManager, t *testing.T) {
				assert.True(t, sm.HasSelection(), "Should have selection")
				assert.Equal(t, "world", sm.GetSelectedText(), "Should select 'world'")
			},
		},
		{
			description: "setting invalid selection should clear selection",
			given: func() *SelectionManager {
				ta := mockTextarea("hello")
				return NewSelectionManager(ta)
			},
			when: func(sm *SelectionManager) {
				sm.SetSelection(-5, 10) // Invalid bounds
			},
			then: func(sm *SelectionManager, t *testing.T) {
				assert.False(t, sm.HasSelection(), "Should have no selection for invalid bounds")
				assert.Empty(t, sm.GetSelectedText(), "Selected text should be empty")
			},
		},
		{
			description: "unicode text should be handled correctly",
			given: func() *SelectionManager {
				ta := mockTextarea("🌟 Hello 🌍")
				return NewSelectionManager(ta)
			},
			when: func(sm *SelectionManager) {
				sm.SetSelection(2, 7) // "Hello" - 🌟(rune 0) space(1) H(2) e(3) l(4) l(5) o(6)
			},
			then: func(sm *SelectionManager, t *testing.T) {
				assert.True(t, sm.HasSelection(), "Should have selection in unicode text")
				assert.Equal(t, "Hello", sm.GetSelectedText(), "Should select 'Hello' correctly")
			},
		},
		{
			description: "empty textarea select all should not error",
			given: func() *SelectionManager {
				ta := mockTextarea("")
				return NewSelectionManager(ta)
			},
			when: func(sm *SelectionManager) {
				sm.SelectAll()
			},
			then: func(sm *SelectionManager, t *testing.T) {
				assert.False(t, sm.HasSelection(), "Empty textarea should have no selection")
				assert.Empty(t, sm.GetSelectedText(), "Selected text should be empty")
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			t.Parallel()

			// Given
			sm := scenario.given()

			// When
			scenario.when(sm)

			// Then
			scenario.then(sm, t)
		})
	}
}

// mockTextarea creates a mock textarea for testing
func mockTextarea(content string) *textarea.Model {
	ta := textarea.New()
	ta.SetValue(content)
	return ta
}

// TestSelectionStateTransitionsBDD tests selection state transitions with BDD approach
func TestSelectionStateTransitionsBDD(t *testing.T) {
	t.Parallel()

	t.Run("selection lifecycle should follow expected transitions", func(t *testing.T) {
		t.Parallel()
		// Given: Empty textarea
		ta := mockTextarea("test content")
		sm := NewSelectionManager(ta)

		// When: Select all
		sm.SelectAll()
		// Then: Should have selection
		require.True(t, sm.HasSelection(), "Should have selection after SelectAll")

		// When: Clear selection
		sm.Clear()
		// Then: Should have no selection
		require.False(t, sm.HasSelection(), "Should have no selection after Clear")
		require.Empty(t, sm.GetSelectedText(), "Selected text should be empty")
	})

	t.Run("selection bounds should be properly managed", func(t *testing.T) {
		// Given: Textarea with content
		ta := mockTextarea("0123456789")
		sm := NewSelectionManager(ta)

		// When: Set selection in middle
		sm.SetSelection(3, 7) // "3456"
		// Then: Should have correct selection
		require.True(t, sm.HasSelection())
		require.Equal(t, "3456", sm.GetSelectedText())

		// When: Set selection to same position
		sm.SetSelection(3, 3)
		// Then: Should have no selection (zero length)
		require.False(t, sm.HasSelection())
		require.Empty(t, sm.GetSelectedText())
	})
}
