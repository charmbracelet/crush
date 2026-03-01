package completions

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestSelectCurrent(t *testing.T) {
	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())

	files := []FileCompletionValue{
		{Path: "/tmp/test"},
		{Path: "/tmp/other"},
	}
	c.SetItems(files, nil)

	t.Run("select without continue", func(t *testing.T) {
		c.open = true
		msg := c.selectCurrent(false, false)

		sel, ok := msg.(SelectionMsg[FileCompletionValue])
		assert.True(t, ok)
		assert.Equal(t, "/tmp/test", sel.Value.Path)
		assert.False(t, sel.KeepOpen)
		assert.False(t, sel.Continue)
		assert.False(t, c.open, "completions should close")
	})

	t.Run("select with keepOpen", func(t *testing.T) {
		c.open = true
		msg := c.selectCurrent(true, false)

		sel, ok := msg.(SelectionMsg[FileCompletionValue])
		assert.True(t, ok)
		assert.True(t, sel.KeepOpen)
		assert.False(t, sel.Continue)
		assert.True(t, c.open, "completions should stay open")
	})

	t.Run("select with continue mode", func(t *testing.T) {
		c.open = true
		msg := c.selectCurrent(true, true)

		sel, ok := msg.(SelectionMsg[FileCompletionValue])
		assert.True(t, ok)
		assert.True(t, sel.KeepOpen)
		assert.True(t, sel.Continue)
		assert.True(t, c.open, "completions should stay open")
	})
}

func TestKeyMapContinueBinding(t *testing.T) {
	km := DefaultKeyMap()

	// Test that Continue is bound to Tab
	assert.True(t, key.Matches(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}), km.Continue))
	assert.False(t, key.Matches(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), km.Continue))

	// Test that Select is NOT bound to Tab anymore
	assert.False(t, key.Matches(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}), km.Select))
	assert.True(t, key.Matches(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), km.Select))
}

func TestUpdateWithTabKey(t *testing.T) {
	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())

	files := []FileCompletionValue{
		{Path: "/tmp/test"},
	}
	c.SetItems(files, nil)

	// Simulate Tab press
	msg, handled := c.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))

	assert.True(t, handled, "Tab should be handled")

	sel, ok := msg.(SelectionMsg[FileCompletionValue])
	assert.True(t, ok, "should return SelectionMsg")
	assert.Equal(t, "/tmp/test", sel.Value.Path)
	assert.True(t, sel.KeepOpen, "should keep open for Tab")
	assert.True(t, sel.Continue, "should have Continue=true for Tab")
}

func TestUpdateWithEnterKey(t *testing.T) {
	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())

	files := []FileCompletionValue{
		{Path: "/tmp/test"},
	}
	c.SetItems(files, nil)

	// Simulate Enter press
	msg, handled := c.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))

	assert.True(t, handled, "Enter should be handled")

	sel, ok := msg.(SelectionMsg[FileCompletionValue])
	assert.True(t, ok, "should return SelectionMsg")
	assert.Equal(t, "/tmp/test", sel.Value.Path)
	assert.False(t, sel.KeepOpen, "should close for Enter")
	assert.False(t, sel.Continue, "should have Continue=false for Enter")
}
