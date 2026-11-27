package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func compactDialog(t *testing.T) *Arguments {
	t.Helper()
	s := styles.CharmtonePantera()
	return NewArguments(
		&common.Common{Styles: &s},
		"Compact Session",
		"Optional: tell the checkpoint what to emphasize.",
		compactArguments(),
		ActionSummarize{SessionID: "s-1"},
	)
}

// TestCompactDialogSubmitsEmpty is the whole reason the field is optional: a
// user who wants an ordinary compaction presses enter and gets one, instead of
// being told a required argument is missing.
func TestCompactDialogSubmitsEmpty(t *testing.T) {
	action := compactDialog(t).HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	submitted, ok := action.(ActionSummarize)
	require.True(t, ok, "enter must hand the compaction back so the UI can start it")
	require.Equal(t, "s-1", submitted.SessionID)
	require.Empty(t, submitted.Instructions)
}

// TestCompactDialogCarriesInstructions checks the text reaches the compaction
// itself, not only the generic argument map.
func TestCompactDialogCarriesInstructions(t *testing.T) {
	dialog := compactDialog(t)
	dialog.inputs[0].SetValue("focus on the failing tests")

	submitted, ok := dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}).(ActionSummarize)
	require.True(t, ok)
	require.Equal(t, "focus on the failing tests", submitted.Instructions)
	require.Equal(t, "focus on the failing tests", submitted.Args[CompactInstructionsArg])
}

// TestCompactArgumentIsOptional is the data behind the label and the missing
// warning: both key off Required.
func TestCompactArgumentIsOptional(t *testing.T) {
	arguments := compactArguments()
	require.Len(t, arguments, 1)
	require.False(t, arguments[0].Required)
}
