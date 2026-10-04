package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

// TestCompactClosesTheDialogItWasSubmittedFrom pins the reported bug: the
// instructions box stayed open over a compaction that was already running,
// because the handler closed the command palette rather than whatever dialog
// the user had actually submitted.
func TestCompactClosesTheDialogItWasSubmittedFrom(t *testing.T) {
	pinTTLs(t)

	m := newBusyUI(&countingWorkspace{ready: true})
	arguments := []commands.Argument{{ID: dialog.CompactInstructionsArg, Title: "Instructions"}}
	m.dialog.OpenDialog(dialog.NewArguments(
		m.com,
		"Compact Session",
		"Optional: tell the checkpoint what to emphasize.",
		arguments,
		dialog.ActionSummarize{SessionID: "s1", Arguments: arguments},
	))
	require.True(t, m.dialog.HasDialogs())

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.False(t, m.dialog.HasDialogs(),
		"a submitted compaction must not leave its box on screen")
	require.NotNil(t, cmd, "and it must hand the compaction to the workspace")
}
