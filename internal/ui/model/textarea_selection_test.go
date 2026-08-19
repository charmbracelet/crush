package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func newSelectionTestUI() *UI {
	u := newTestUI()
	u.dialog = dialog.NewOverlay()
	sty := u.com.Styles.Attachments
	u.attachments = attachments.New(
		attachments.NewRenderer(sty.Normal, sty.Deleting, sty.Image, sty.Text, sty.Skill, sty.Remove),
		attachments.Keymap{},
	)
	u.updateLayoutAndSize()
	return u
}

func TestTextareaSelectionKeys(t *testing.T) {
	t.Parallel()

	u := newSelectionTestUI()

	for _, r := range "hello" {
		u.textarea.InsertRune(r)
	}
	u.textarea.CursorStart()

	_, _ = u.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	_, _ = u.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})

	require.True(t, u.textarea.HasSelection())
	require.Equal(t, "he", u.textarea.SelectedText())
}

func TestTextareaCutSelection(t *testing.T) {
	t.Parallel()

	u := newSelectionTestUI()
	u.keyMap = DefaultKeyMap()

	for _, r := range "hello" {
		u.textarea.InsertRune(r)
	}
	u.textarea.CursorStart()

	_, _ = u.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	_, _ = u.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	require.True(t, u.textarea.HasSelection())

	_, _ = u.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl | tea.ModShift})

	require.False(t, u.textarea.HasSelection())
	require.Equal(t, "llo", u.textarea.Value())
}

func TestTextareaMouseSelection(t *testing.T) {
	t.Parallel()

	u := newSelectionTestUI()

	for _, r := range "hello world" {
		u.textarea.InsertRune(r)
	}
	u.textarea.CursorStart()

	// The textarea renders one row below the editor top (the attachments
	// row is always reserved, even when empty). The default prompt
	// ("┃ ") is 2 cells wide.
	startX := u.layout.editor.Min.X + 2
	y := u.layout.editor.Min.Y + 1

	_, _ = u.Update(tea.MouseClickMsg(tea.Mouse{X: startX, Y: y, Button: uv.MouseLeft}))
	require.True(t, u.textareaMouseSelecting)

	// Drag a few cells to the right.
	_, _ = u.Update(tea.MouseMotionMsg(tea.Mouse{X: startX + 5, Y: y, Button: uv.MouseLeft}))

	// Release to end the gesture.
	_, _ = u.Update(tea.MouseReleaseMsg(tea.Mouse{X: startX + 5, Y: y, Button: uv.MouseLeft}))
	require.False(t, u.textareaMouseSelecting)

	require.True(t, u.textarea.HasSelection())
	require.Equal(t, "hello", u.textarea.SelectedText())
}

func TestTextareaMouseClickOutsideDoesNotSelect(t *testing.T) {
	t.Parallel()

	u := newSelectionTestUI()

	for _, r := range "hello" {
		u.textarea.InsertRune(r)
	}

	// Click in the chat area, far from the editor.
	_, _ = u.Update(tea.MouseClickMsg(tea.Mouse{
		X:      u.layout.main.Min.X + 1,
		Y:      u.layout.main.Min.Y + 1,
		Button: uv.MouseLeft,
	}))

	require.False(t, u.textareaMouseSelecting)
	require.False(t, u.textarea.HasSelection())
}
