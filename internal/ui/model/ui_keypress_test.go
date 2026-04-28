package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

func TestHandleKeyPressMsgCtrlDOpensQuitDialogWhenEditorIsEmpty(t *testing.T) {
	t.Parallel()

	ui := newTestUI()

	ui.handleKeyPressMsg(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})

	require.True(t, ui.dialog.ContainsDialog(dialog.QuitID))
}

func TestHandleKeyPressMsgCtrlDDoesNotOpenQuitDialogWhenEditorHasInput(t *testing.T) {
	t.Parallel()

	ui := newTestUI()
	ui.textarea.SetValue("hello")

	ui.handleKeyPressMsg(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})

	require.False(t, ui.dialog.ContainsDialog(dialog.QuitID))
}

func TestHandleKeyPressMsgCtrlDDoesNotOpenQuitDialogWhenAttachmentsExist(t *testing.T) {
	t.Parallel()

	ui := newTestUI()
	ui.attachments.Update(message.Attachment{
		FileName: "notes.txt",
		MimeType: "text/plain",
	})

	ui.handleKeyPressMsg(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})

	require.False(t, ui.dialog.ContainsDialog(dialog.QuitID))
}
