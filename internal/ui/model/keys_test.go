package model

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

func TestSourceAndMemoryShortcutsAreDistinct(t *testing.T) {
	t.Parallel()

	keys := DefaultKeyMap()
	sourceKey := tea.KeyPressMsg{Code: 's', Mod: tea.ModAlt}
	memoryKey := tea.KeyPressMsg{Code: 'r', Mod: tea.ModAlt}

	require.True(t, key.Matches(sourceKey, keys.Sources))
	require.False(t, key.Matches(sourceKey, keys.Memory))
	require.True(t, key.Matches(memoryKey, keys.Memory))
	require.False(t, key.Matches(memoryKey, keys.Sources))
}
