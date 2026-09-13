package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

type dialogMouseActionMsg struct{}

type mouseActionDialog struct{}

func (*mouseActionDialog) ID() string {
	return "mouse-action"
}

func (*mouseActionDialog) HandleMsg(msg tea.Msg) dialog.Action {
	if _, ok := msg.(tea.MouseClickMsg); !ok {
		return nil
	}
	return dialog.ActionCmd{Cmd: func() tea.Msg { return dialogMouseActionMsg{} }}
}

func (*mouseActionDialog) Draw(uv.Screen, uv.Rectangle) *tea.Cursor {
	return nil
}

func TestMouseClickExecutesDialogActionCommand(t *testing.T) {
	t.Parallel()

	m := newTestUI()
	m.dialog = dialog.NewOverlay()
	m.dialog.OpenDialog(new(mouseActionDialog))

	_, cmd := m.Update(tea.MouseClickMsg(tea.Mouse{Button: tea.MouseLeft}))
	require.NotNil(t, cmd)
	require.IsType(t, dialogMouseActionMsg{}, cmd())
}

var _ dialog.Dialog = (*mouseActionDialog)(nil)
