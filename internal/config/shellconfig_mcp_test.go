package config_test

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestShellConfigMCPAdd(t *testing.T) {
	store := loadCrushSh(t, `mcp add github --type http --url "https://api.githubcopilot.com/mcp/" --header Authorization "Bearer xyz"
mcp add fs --command node --args server.js --args --stdio`)

	mcps := store.Config().MCP

	gh, ok := mcps["github"]
	require.True(t, ok, "github MCP should be configured")
	require.Equal(t, config.MCPHttp, gh.Type)
	require.Equal(t, "https://api.githubcopilot.com/mcp/", gh.URL)
	require.Equal(t, "Bearer xyz", gh.Headers["Authorization"])

	fs, ok := mcps["fs"]
	require.True(t, ok, "fs MCP should be configured")
	require.Equal(t, config.MCPStdio, fs.Type)
	require.Equal(t, "node", fs.Command)
	require.Equal(t, []string{"server.js", "--stdio"}, fs.Args)
}

func TestShellConfigMCPRemove(t *testing.T) {
	store := loadCrushSh(t, `mcp add keep --command x
mcp add drop --command y
mcp remove drop`)

	mcps := store.Config().MCP
	require.Contains(t, mcps, "keep")
	require.NotContains(t, mcps, "drop")
}
