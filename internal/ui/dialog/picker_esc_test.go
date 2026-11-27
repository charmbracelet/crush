package dialog

import (
	"testing"

	"charm.land/bubbles/v2/key"
	"github.com/stretchr/testify/require"
)

// TestPickerEscLabels verifies esc is labeled "exit" when it just closes a
// picker and "cancel" when it discards edits.
func TestPickerEscLabels(t *testing.T) {
	t.Parallel()
	com := newPickerTestCommon()
	commandsDialog, err := NewCommands(com, "", false, false, false, false, nil, nil)
	require.NoError(t, err)
	fp, _ := NewFilePicker(com)

	escLabel := func(bindings []key.Binding) string {
		for _, b := range bindings {
			if b.Help().Key == "esc" {
				return b.Help().Desc
			}
		}
		return ""
	}
	require.Equal(t, "exit", escLabel(commandsDialog.ShortHelp()))
	require.Equal(t, "exit", escLabel(newTestChannels(t, nil).ShortHelp()))
	require.Equal(t, "exit", escLabel(fp.ShortHelp()))
	require.Equal(t, "cancel", escLabel(NewThemeEditor(com, "charmtone-panther").ShortHelp()))
	require.Equal(t, "cancel", escLabel(NewThemeNew(com, "").ShortHelp()))
}
