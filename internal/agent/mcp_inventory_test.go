package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/stretchr/testify/require"
)

func TestConnectedMCPInventoryIsCompactAndTruthful(t *testing.T) {
	t.Parallel()

	inventory := connectedMCPInventory(map[string]mcp.ClientInfo{
		"github": {
			State:  mcp.StateConnected,
			Counts: mcp.Counts{Tools: 44, Prompts: 2},
		},
		"weather": {
			State:  mcp.StateConnected,
			Counts: mcp.Counts{Tools: 9},
		},
		"offline": {
			State:  mcp.StateError,
			Counts: mcp.Counts{Tools: 12},
		},
	})

	require.Contains(t, inventory, "- github: 44 tools, 2 prompts")
	require.Contains(t, inventory, "- weather: 9 tools")
	require.NotContains(t, inventory, "offline")
	require.NotContains(t, inventory, "arguments_schema")
}
