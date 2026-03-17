package model

import (
	"errors"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestMCPListShowsAuthenticationRequiredError(t *testing.T) {
	t.Parallel()

	theme := styles.DefaultStyles()
	rendered := mcpList(&theme, []mcp.ClientInfo{{
		Name:  "notion",
		State: mcp.StateNeedsAuth,
		Error: errors.New("login required"),
	}}, 120, 5)

	require.Contains(t, rendered, "notion")
	require.Contains(t, rendered, "authentication required: login required")
}

func TestMCPHTTPConfigSupportsInteractiveAuth(t *testing.T) {
	t.Parallel()

	require.True(t, config.MCPConfig{Type: config.MCPHttp}.SupportsInteractiveAuth())
	require.False(t, config.MCPConfig{Type: config.MCPStdio}.SupportsInteractiveAuth())
}
