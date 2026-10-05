package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDefaultKeyMap_HasSubagentsBinding verifies that the global KeyMap
// includes an enabled Subagents binding bound to ctrl+q. ctrl+a would collide
// with readline start-of-line in the editor and with Allow in the permission
// prompt; ctrl+x with delete in the Sessions and Theme dialogs.
func TestDefaultKeyMap_HasSubagentsBinding(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()

	require.True(t, km.Subagents.Enabled(), "Subagents binding should be enabled")
	require.Equal(t, []string{"ctrl+q"}, km.Subagents.Keys(),
		"ctrl+a and ctrl+x are taken by the editor and dialogs")
}
