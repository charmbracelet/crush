package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDefaultKeyMap_HasSubagentsBinding verifies that the global KeyMap
// includes an enabled Subagents binding bound to ctrl+x. ctrl+x avoids the
// readline start-of-line collision that ctrl+a caused in the editor.
func TestDefaultKeyMap_HasSubagentsBinding(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()

	require.True(t, km.Subagents.Enabled(), "Subagents binding should be enabled")
	require.Contains(t, km.Subagents.Keys(), "ctrl+x")
	require.NotContains(t, km.Subagents.Keys(), "ctrl+a",
		"ctrl+a collides with readline start-of-line in the editor")
}
