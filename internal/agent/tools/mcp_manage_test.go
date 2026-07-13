package tools

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestMCPManageStatusUsesStructuredState(t *testing.T) {
	t.Parallel()

	cfg := config.NewTestStore(&config.Config{MCP: map[string]config.MCPConfig{
		"github": {Disabled: true},
	}})
	tool := newMCPManageTool(cfg, mcpManageDeps{
		getState: func(string) (mcp.ClientInfo, bool) {
			return mcp.ClientInfo{State: mcp.StateDisabled}, true
		},
	})

	response, err := tool.Run(t.Context(), fantasy.ToolCall{Name: MCPManageToolName, Input: `{"action":"status","name":"github"}`})
	require.NoError(t, err)
	require.False(t, response.IsError)
	require.Equal(t, "github: disabled; disabled=true", response.Content)
}

func TestMCPManageEnablesExistingServerWithoutRebuildingIt(t *testing.T) {
	t.Parallel()

	cfg := config.NewTestStore(&config.Config{MCP: map[string]config.MCPConfig{
		"github": {Type: config.MCPStdio, Command: "server", Disabled: true},
	}})
	var key string
	var value any
	tool := newMCPManageTool(cfg, mcpManageDeps{
		setConfig: func(_ config.Scope, gotKey string, gotValue any) error {
			key = gotKey
			value = gotValue
			return nil
		},
		refresh: func(context.Context, *config.ConfigStore, string) (map[string]error, error) {
			return map[string]error{"github": nil}, nil
		},
		getState: func(string) (mcp.ClientInfo, bool) {
			return mcp.ClientInfo{State: mcp.StateConnected}, true
		},
	})

	response, err := tool.Run(t.Context(), fantasy.ToolCall{Name: MCPManageToolName, Input: `{"action":"enable","name":"github"}`})
	require.NoError(t, err)
	require.False(t, response.IsError)
	require.Equal(t, "mcp.github.disabled", key)
	require.Equal(t, false, value)
	require.Contains(t, response.Content, "github: enable")
	require.Contains(t, response.Content, "github: connected")
}

func TestMCPManageRejectsMutationOfMissingServer(t *testing.T) {
	t.Parallel()

	tool := newMCPManageTool(config.NewTestStore(&config.Config{}), mcpManageDeps{})
	response, err := tool.Run(t.Context(), fantasy.ToolCall{Name: MCPManageToolName, Input: `{"action":"enable","name":"missing"}`})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.Contains(t, response.Content, "use mcp_add to create it")
}
