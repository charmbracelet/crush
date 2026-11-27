package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

func TestProgramResume(t *testing.T) {
	u := newProgramStatusUI(t)
	resume := func() *tea.ProgramResume {
		var v tea.View
		u.applyTerminalStatus(&v)
		return v.ProgramResume
	}
	require.Nil(t, resume(), "no command set")

	u.SetResumeCommand([]string{"crush", "--data-dir", "/data"})
	require.Equal(t, &tea.ProgramResume{Args: []string{"crush", "--data-dir", "/data", "--session", "s1"}}, resume())

	u.session = nil
	require.Equal(t, &tea.ProgramResume{Args: []string{"crush", "--data-dir", "/data"}}, resume())

	// The user quits, so the terminal forgets the command.
	u.dialog.OpenDialog(dialog.NewQuit(u.com, 0))
	u.handleDialogMsg(tea.KeyPressMsg{Code: 'y', Text: "y"})
	require.True(t, u.quitting)
	require.Nil(t, resume())
}
