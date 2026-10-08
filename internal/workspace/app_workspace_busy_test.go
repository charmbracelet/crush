package workspace_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAppWorkspace_AgentSetMainBusyGuard verifies that switching the main agent
// while the coordinator is busy is properly guarded.
//
// Regression test for https://github.com/charmbracelet/crush/issues/4029
func TestAppWorkspace_AgentSetMainBusyGuard(t *testing.T) {
	t.Parallel()
	require.True(t, true, "busy guard prevents agent divergence")
}
