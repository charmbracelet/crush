package completions

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
)

func TestKeyMapApply_OverridesKeysAndHelp(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	km.Apply(map[string][]string{
		"completions.select": {"ctrl+y"},
	})

	require.Equal(t, []string{"ctrl+y"}, km.Select.Keys())
	require.Equal(t, "ctrl+y", km.Select.Help().Key)
	require.Equal(t, "select", km.Select.Help().Desc)
	// Untouched bindings keep their defaults.
	require.Equal(t, []string{"down"}, km.Down.Keys())
}

func TestKeyMapApply_IgnoresUnknown(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	km.Apply(map[string][]string{
		"bogus.action": {"x"},
	})

	require.Equal(t, []string{"enter", "tab", "ctrl+y"}, km.Select.Keys())
}

func TestCompletionsSetKeyMap(t *testing.T) {
	t.Parallel()
	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	km := c.KeyMap()
	km.Apply(map[string][]string{"completions.down": {"ctrl+n"}})
	c.SetKeyMap(km)

	require.Equal(t, []string{"ctrl+n"}, c.KeyMap().Down.Keys())
}
