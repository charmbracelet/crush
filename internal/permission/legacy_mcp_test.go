package permission

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// MCP tools were renamed to a doubled-underscore form so requests bill to a
// Claude subscription rather than extra usage. Configurations written against
// the old names must keep working; otherwise the rename silently turns an
// allowlisted tool back into a prompt.
func TestLegacyMCPToolKey(t *testing.T) {
	t.Parallel()

	require.Equal(t, "mcp_ssh_ssh_connect", legacyMCPToolKey("mcp__ssh__ssh_connect"))
	require.Equal(t, "mcp_docker_list:run", legacyMCPToolKey("mcp__docker__list:run"))

	// Only the server separator collapses; underscores inside a tool name stay.
	require.Equal(t, "mcp_ssh_close_session", legacyMCPToolKey("mcp__ssh__close_session"))

	// Not an MCP tool key.
	require.Equal(t, "", legacyMCPToolKey("bash"))
	require.Equal(t, "", legacyMCPToolKey("mcp_old_style"))
}

func TestAllowlistAcceptsBothMCPSpellings(t *testing.T) {
	t.Parallel()

	s := &permissionService{allowedTools: []string{"mcp_ssh_ssh_connect", "bash"}}

	require.True(t, s.isAllowed("mcp__ssh__ssh_connect"), "old config entry should still allow the renamed tool")
	require.True(t, s.isAllowed("bash"))
	require.False(t, s.isAllowed("mcp__ssh__ssh_close"))

	// A config already using the new spelling works too.
	s = &permissionService{allowedTools: []string{"mcp__ssh__ssh_connect"}}
	require.True(t, s.isAllowed("mcp__ssh__ssh_connect"))
}
