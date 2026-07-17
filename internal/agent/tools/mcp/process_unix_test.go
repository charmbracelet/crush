//go:build !windows

package mcp

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// TestCreateTransport_StdioProcessGroup pins that a stdio MCP child is spawned
// as its own process-group leader with a cancel hook wired up. This is what
// lets Crush reap a server's descendant processes (e.g. signal-cli launched by
// signal-mcp) when the session context is cancelled, instead of orphaning them.
func TestCreateTransport_StdioProcessGroup(t *testing.T) {
	t.Parallel()

	m := config.MCPConfig{Type: config.MCPStdio, Command: "echo", Args: []string{"hi"}}
	tr, err := createTransport(t.Context(), m, shellResolverWithPath(t, nil))
	require.NoError(t, err)

	ct, ok := tr.(*mcp.CommandTransport)
	require.True(t, ok, "expected CommandTransport, got %T", tr)
	require.NotNil(t, ct.Command.SysProcAttr, "stdio child must set SysProcAttr")
	require.True(t, ct.Command.SysProcAttr.Setpgid,
		"stdio child must lead its own process group so cancellation kills the whole tree")
	require.NotNil(t, ct.Command.Cancel,
		"stdio child must set a Cancel hook that kills the process group")
}
