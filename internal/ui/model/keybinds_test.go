package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/keybinds"
	"github.com/stretchr/testify/require"
)

// Every registry action resolves to a binding. If someone adds an ID
// without a case in bindingFor, this fails.
func TestApplyKeybinds_CoversRegistry(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	for _, id := range keybinds.Actions() {
		switch id {
		case "completions.down", "completions.up", "completions.select",
			"completions.cancel", "completions.down_insert", "completions.up_insert",
			"dialog.close":
			continue
		default:
			require.NotNil(t, bindingFor(&km, id), "registry ID %q has no binding", id)
		}
	}
}

func TestApplyKeybinds_OverridesKeysAndHelp(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	warnings := ApplyKeybinds(&km, map[string][]string{
		"global.quit": {"ctrl+q"},
	})

	require.Empty(t, warnings)
	require.Equal(t, []string{"ctrl+q"}, km.Quit.Keys())
	require.Equal(t, "ctrl+q", km.Quit.Help().Key)
	require.Equal(t, "quit", km.Quit.Help().Desc)
}

func TestApplyKeybinds_NormalizesSpace(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	ApplyKeybinds(&km, map[string][]string{
		"chat.page_down": {"pgdown", " ", "f"},
	})

	require.Equal(t, []string{"pgdown", "space", "f"}, km.Chat.PageDown.Keys())
}

func TestApplyKeybinds_UnknownWarnsAndKeepsDefault(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	before := km.Quit.Keys()
	warnings := ApplyKeybinds(&km, map[string][]string{
		"bogus.action": {"x"},
	})

	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "bogus.action")
	require.Equal(t, before, km.Quit.Keys())
}

func TestApplyKeybinds_EmptyIsNoop(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	require.Empty(t, ApplyKeybinds(&km, nil))
	require.Empty(t, ApplyKeybinds(&km, map[string][]string{}))
	require.Empty(t, ApplyKeybinds(&km, map[string][]string{"global.quit": {}}))
	require.Equal(t, []string{"ctrl+c"}, km.Quit.Keys())
}

func TestApplyKeybinds_WarningsSorted(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	warnings := ApplyKeybinds(&km, map[string][]string{
		"z.action": {"x"},
		"a.action": {"y"},
	})

	require.Len(t, warnings, 2)
	require.Less(t, warnings[0], warnings[1])
}
