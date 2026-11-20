package permission

import (
	"testing"

	"github.com/charmbracelet/crush/internal/mcpname"
	"github.com/stretchr/testify/require"
)

// Allow-lists written with pre-rename MCP names are upgraded at load and
// still approve the renamed tool.
func TestUpgradedLegacyMCPAllowlist(t *testing.T) {
	t.Parallel()

	allowed := mcpname.UpgradeAllowList([]string{"mcp_docker.io_list:execute"}, []string{"docker.io"})
	s := NewPermissionService(t.TempDir(), allowed, nil)
	ok, err := s.Request(t.Context(), CreatePermissionRequest{
		SessionID:  "s1",
		ToolCallID: "c1",
		ToolName:   mcpname.Tool("docker.io", "list"),
		Action:     "execute",
		Path:       t.TempDir(),
	})
	require.NoError(t, err)
	require.True(t, ok)
}
