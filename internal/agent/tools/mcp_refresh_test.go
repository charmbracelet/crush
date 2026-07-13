package tools

import (
	"context"
	"errors"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestMCPRefreshToolRequiresExplicitTarget(t *testing.T) {
	t.Parallel()

	tool := newMCPRefreshTool(config.NewTestStore(&config.Config{}), func(context.Context, *config.ConfigStore, string) (map[string]error, error) {
		return map[string]error{}, nil
	})
	response, err := tool.Run(t.Context(), fantasy.ToolCall{Name: MCPRefreshToolName, Input: `{}`})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.Contains(t, response.Content, "name is required")
}

func TestMCPRefreshToolReportsNoConfiguredClientsForExplicitAll(t *testing.T) {
	t.Parallel()

	tool := newMCPRefreshTool(config.NewTestStore(&config.Config{}), func(context.Context, *config.ConfigStore, string) (map[string]error, error) {
		return map[string]error{}, nil
	})
	response, err := tool.Run(t.Context(), fantasy.ToolCall{Name: MCPRefreshToolName, Input: `{"all":true}`})
	require.NoError(t, err)
	require.False(t, response.IsError)
	require.Contains(t, response.Content, "no MCP clients")
}

func TestMCPRefreshToolRejectsMissingNamedResult(t *testing.T) {
	t.Parallel()

	tool := newMCPRefreshTool(config.NewTestStore(&config.Config{}), func(context.Context, *config.ConfigStore, string) (map[string]error, error) {
		return map[string]error{}, nil
	})
	response, err := tool.Run(t.Context(), fantasy.ToolCall{Name: MCPRefreshToolName, Input: `{"name":"playwright-test-missing"}`})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.Contains(t, response.Content, "not configured or initialized")
}

func TestMCPRefreshToolReturnsRefreshError(t *testing.T) {
	t.Parallel()

	tool := newMCPRefreshTool(config.NewTestStore(&config.Config{}), func(context.Context, *config.ConfigStore, string) (map[string]error, error) {
		return nil, errors.New("reload failed")
	})
	response, err := tool.Run(t.Context(), fantasy.ToolCall{Name: MCPRefreshToolName, Input: `{"all":true}`})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.Contains(t, response.Content, "reload failed")
}
