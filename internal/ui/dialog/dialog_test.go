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

	require.Equal(t, "exit", escLabel(commandsDialog.ShortHelp()))
	require.Equal(t, "exit", escLabel(newTestChannels(t, nil).ShortHelp()))
	require.Equal(t, "exit", escLabel(fp.ShortHelp()))
	require.Equal(t, "cancel", escLabel(NewThemeEditor(com, "charmtone-panther").ShortHelp()))
	require.Equal(t, "cancel", escLabel(NewThemeNew(com, "").ShortHelp()))
}

// TestPickerHelpPutsEscNearTheFront verifies every picker lists esc within
// its first two help entries, right after the main action, so a narrow
// help line truncates optional hints before the way out.
func TestPickerHelpPutsEscNearTheFront(t *testing.T) {
	t.Parallel()
	com := newPickerTestCommon()
	commandsDialog, err := NewCommands(com, "", false, false, false, false, nil, nil)
	require.NoError(t, err)
	fp, _ := NewFilePicker(com)

	helps := map[string][]key.Binding{
		"commands":     commandsDialog.ShortHelp(),
		"channels":     newTestChannels(t, nil).ShortHelp(),
		"file picker":  fp.ShortHelp(),
		"theme editor": NewThemeEditor(com, "charmtone-panther").ShortHelp(),
		"new theme":    NewThemeNew(com, "").ShortHelp(),
		"themes":       NewTheme(com).ShortHelp(),
		"mcp toggles":  newMCPTogglesForTest([]MCPToggleItem{{Name: "a", Status: "connected"}}).ShortHelp(),
	}
	for name, h := range helps {
		pos := -1
		for i, b := range h {
			if b.Help().Key == "esc" {
				pos = i
				break
			}
		}
		require.GreaterOrEqual(t, pos, 0, "%s has no esc hint", name)
		require.LessOrEqual(t, pos, 1, "%s lists esc too late", name)
	}
}

// TestArrowHelpNotation verifies pickers write arrow keys as "↑/↓" (and
// "←/→") rather than mixing "↑↓", "↑/k up" and similar forms.
func TestArrowHelpNotation(t *testing.T) {
	t.Parallel()
	com := newPickerTestCommon()
	fp, _ := NewFilePicker(com)
	keys := func(km interface{ ShortHelp() []key.Binding }) []string {
		var out []string
		for _, b := range km.ShortHelp() {
			out = append(out, b.Help().Key)
		}
		return out
	}
	items := []MCPToggleItem{{Name: "docker", Status: "connected"}}
	require.Contains(t, keys(NewMCPToggles(com, items)), "↑/↓")
	require.Contains(t, keys(NewSkillsToggles(com, []SkillToggleItem{{Name: "s"}})), "↑/↓")
	require.Contains(t, keys(fp), "↑/↓/←/→")
}
