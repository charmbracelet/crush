package tools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The UI splits a tool name to show "server → tool". It parsed the legacy
// mcp_server_tool spelling, so after the rename to mcp__server__tool the
// server half came back empty and the arrow rendered with nothing before it.
func TestParseMCPToolName(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, in, server, tool string
		ok                     bool
	}{
		{"current spelling", "mcp__prickly__javascript_eval", "prickly", "javascript_eval", true},
		{"tool half keeps its underscores", "mcp__ssh__ssh_connect", "ssh", "ssh_connect", true},
		{"legacy spelling still resolves", "mcp_ssh_ssh_connect", "ssh", "ssh_connect", true},
		{"not an mcp tool", "bash", "", "", false},
		{"prefix only", "mcp__", "", "", false},
		{"no tool half", "mcp__server", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server, tool, ok := ParseMCPToolName(tc.in)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.Equal(t, tc.server, server)
				require.Equal(t, tc.tool, tool)
			}
		})
	}
}

// Whatever MCPToolName produces must parse back out, or the UI cannot label
// the tools the model is actually calling.
func TestMCPToolNameRoundTrips(t *testing.T) {
	t.Parallel()

	server, tool, ok := ParseMCPToolName(MCPToolName("prickly", "javascript_eval"))
	require.True(t, ok)
	require.Equal(t, "prickly", server)
	require.Equal(t, "javascript_eval", tool)
}
