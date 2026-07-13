package tools

import (
	"context"
	"errors"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func allowMCPAdd(context.Context, fantasy.ToolCall, config.Scope, MCPAddPermissionParams) (bool, error) {
	return true, nil
}

func verifiedMCPAddContext(t *testing.T) context.Context {
	t.Helper()
	ctx := WithMCPSourceEvidence(t.Context(), "install MCP")
	recordMCPSearchResults(ctx, []SearchResult{
		{Link: "https://example.com/docs"},
		{Link: "https://github.com/github/github-mcp-server"},
	})
	content := "MCP server installation and configuration with command and args."
	require.Empty(t, recordMCPSourceEvidence(ctx, "https://example.com/docs", content))
	require.Empty(t, recordMCPSourceEvidence(ctx, "https://github.com/github/github-mcp-server", content))
	return ctx
}

func TestMCPAddRequiresExactlyOneTransport(t *testing.T) {
	t.Parallel()
	tool := newMCPAddTool(config.NewTestStore(&config.Config{}), mcpAddDeps{})

	response, err := tool.Run(verifiedMCPAddContext(t), fantasy.ToolCall{Name: MCPAddToolName, Input: `{"name":"browser","source_url":"https://example.com/docs"}`})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.Contains(t, response.Content, "set exactly one of stdio, http, or sse")
}

func TestMCPAddSchemaSeparatesTransportFields(t *testing.T) {
	t.Parallel()
	tool := newMCPAddTool(config.NewTestStore(&config.Config{}), mcpAddDeps{})
	parameters := tool.Info().Parameters
	require.NotContains(t, tool.Info().Required, "source_url")

	stdio := parameters["stdio"].(map[string]any)
	stdioProperties := stdio["properties"].(map[string]any)
	require.Contains(t, stdioProperties, "command")
	require.Contains(t, stdioProperties, "args")
	require.NotContains(t, stdioProperties, "url")

	http := parameters["http"].(map[string]any)
	httpProperties := http["properties"].(map[string]any)
	require.Contains(t, httpProperties, "url")
	require.NotContains(t, httpProperties, "command")
	require.NotContains(t, httpProperties, "args")
}

func TestMCPAddRejectsMixedTransportFieldBeforePermissionOrWrite(t *testing.T) {
	t.Parallel()
	permissionCalled := false
	tool := newMCPAddTool(config.NewTestStore(&config.Config{}), mcpAddDeps{
		authorize: func(context.Context, fantasy.ToolCall, config.Scope, MCPAddPermissionParams) (bool, error) {
			permissionCalled = true
			return true, nil
		},
		setConfig: func(config.Scope, string, any) error {
			t.Fatal("invalid config must not be written")
			return nil
		},
	})

	response, err := tool.Run(verifiedMCPAddContext(t), fantasy.ToolCall{Name: MCPAddToolName, Input: `{"name":"browser","source_url":"https://example.com/docs","stdio":{"command":"npx","url":"package-name"}}`})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.Contains(t, response.Content, `unknown field "url"`)
	require.False(t, permissionCalled)
}

func TestMCPAddAllowsConfigurationWithoutSourceEvidence(t *testing.T) {
	t.Parallel()
	permissionCalled := false
	var written config.MCPConfig
	tool := newMCPAddTool(config.NewTestStore(&config.Config{}), mcpAddDeps{
		authorize: func(context.Context, fantasy.ToolCall, config.Scope, MCPAddPermissionParams) (bool, error) {
			permissionCalled = true
			return true, nil
		},
		setConfig: func(_ config.Scope, _ string, value any) error {
			written = value.(config.MCPConfig)
			return nil
		},
		refresh: func(context.Context, *config.ConfigStore, string) (map[string]error, error) {
			return map[string]error{"browser": nil}, nil
		},
		getState: func(string) (mcp.ClientInfo, bool) {
			return mcp.ClientInfo{State: mcp.StateConnected}, true
		},
	})

	response, err := tool.Run(t.Context(), fantasy.ToolCall{Name: MCPAddToolName, Input: `{"name":"browser","stdio":{"command":"npx","args":["package"]}}`})
	require.NoError(t, err)
	require.False(t, response.IsError)
	require.True(t, permissionCalled)
	require.Equal(t, "npx", written.Command)
	require.Equal(t, []string{"package"}, written.Args)
}

func TestMCPAddCreatesStdioConfigurationAfterPermission(t *testing.T) {
	t.Parallel()
	cfg := config.NewTestStore(&config.Config{})
	var permissionParams MCPAddPermissionParams
	var written config.MCPConfig
	tool := newMCPAddTool(cfg, mcpAddDeps{
		authorize: func(_ context.Context, _ fantasy.ToolCall, scope config.Scope, params MCPAddPermissionParams) (bool, error) {
			require.Equal(t, config.ScopeGlobal, scope)
			permissionParams = params
			return true, nil
		},
		setConfig: func(scope config.Scope, key string, value any) error {
			require.Equal(t, config.ScopeGlobal, scope)
			require.Equal(t, "mcp.browser", key)
			written = value.(config.MCPConfig)
			return nil
		},
		refresh: func(context.Context, *config.ConfigStore, string) (map[string]error, error) {
			return map[string]error{"browser": nil}, nil
		},
		getState: func(string) (mcp.ClientInfo, bool) {
			return mcp.ClientInfo{State: mcp.StateConnected, Counts: mcp.Counts{Tools: 2}}, true
		},
	})

	response, err := tool.Run(verifiedMCPAddContext(t), fantasy.ToolCall{Name: MCPAddToolName, Input: `{"name":"browser","source_url":"https://example.com/docs","stdio":{"command":"npx","args":["-y","package-name"],"env":{"TOKEN":"secret"}}}`})
	require.NoError(t, err)
	require.False(t, response.IsError)
	require.Equal(t, config.MCPStdio, written.Type)
	require.Equal(t, "npx", written.Command)
	require.Equal(t, []string{"-y", "package-name"}, written.Args)
	require.Empty(t, written.URL)
	require.Equal(t, []string{"TOKEN"}, permissionParams.EnvKeys)
	require.Equal(t, "https://example.com/docs", permissionParams.SourceURL)
	require.Contains(t, response.Content, "config=created")
}

func TestMCPAddCreatesHTTPConfiguration(t *testing.T) {
	t.Parallel()
	cfg := config.NewTestStore(&config.Config{})
	var written config.MCPConfig
	tool := newMCPAddTool(cfg, mcpAddDeps{
		authorize: allowMCPAdd,
		setConfig: func(_ config.Scope, _ string, value any) error {
			written = value.(config.MCPConfig)
			return nil
		},
		refresh: func(context.Context, *config.ConfigStore, string) (map[string]error, error) {
			return map[string]error{"remote": nil}, nil
		},
		getState: func(string) (mcp.ClientInfo, bool) {
			return mcp.ClientInfo{State: mcp.StateConnected}, true
		},
	})

	response, err := tool.Run(verifiedMCPAddContext(t), fantasy.ToolCall{Name: MCPAddToolName, Input: `{"name":"remote","source_url":"https://example.com/docs","http":{"url":"https://example.com/mcp","headers":{"Authorization":"Bearer secret"}}}`})
	require.NoError(t, err)
	require.False(t, response.IsError)
	require.Equal(t, config.MCPHttp, written.Type)
	require.Equal(t, "https://example.com/mcp", written.URL)
	require.Empty(t, written.Command)
}

func TestMCPAddPermissionDenialDoesNotWriteOrStart(t *testing.T) {
	t.Parallel()
	tool := newMCPAddTool(config.NewTestStore(&config.Config{}), mcpAddDeps{
		authorize: func(context.Context, fantasy.ToolCall, config.Scope, MCPAddPermissionParams) (bool, error) {
			return false, nil
		},
		setConfig: func(config.Scope, string, any) error {
			t.Fatal("denied config must not be written")
			return nil
		},
		refresh: func(context.Context, *config.ConfigStore, string) (map[string]error, error) {
			t.Fatal("denied server must not be started")
			return nil, nil
		},
	})

	response, err := tool.Run(verifiedMCPAddContext(t), fantasy.ToolCall{Name: MCPAddToolName, Input: `{"name":"browser","source_url":"https://example.com/docs","stdio":{"command":"server"}}`})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.True(t, response.StopTurn)
	require.Contains(t, response.Content, "denied permission")
}

func TestMCPAddReusesMatchingConnectedConfigurationWithoutPermission(t *testing.T) {
	t.Parallel()
	cfg := config.NewTestStore(&config.Config{MCP: map[string]config.MCPConfig{
		"browser": {Type: config.MCPStdio, Command: "server"},
	}})
	tool := newMCPAddTool(cfg, mcpAddDeps{
		authorize: func(context.Context, fantasy.ToolCall, config.Scope, MCPAddPermissionParams) (bool, error) {
			t.Fatal("connected matching config must not request permission")
			return false, nil
		},
		getState: func(string) (mcp.ClientInfo, bool) {
			return mcp.ClientInfo{State: mcp.StateConnected, Counts: mcp.Counts{Tools: 3}}, true
		},
	})

	response, err := tool.Run(verifiedMCPAddContext(t), fantasy.ToolCall{Name: MCPAddToolName, Input: `{"name":"browser","source_url":"https://example.com/docs","stdio":{"command":"server"}}`})
	require.NoError(t, err)
	require.False(t, response.IsError)
	require.Contains(t, response.Content, "config=reused")
}

func TestMCPAddProtectsDifferentExistingConfiguration(t *testing.T) {
	t.Parallel()
	cfg := config.NewTestStore(&config.Config{MCP: map[string]config.MCPConfig{
		"browser": {Type: config.MCPStdio, Command: "old-server"},
	}})
	tool := newMCPAddTool(cfg, mcpAddDeps{})

	response, err := tool.Run(verifiedMCPAddContext(t), fantasy.ToolCall{Name: MCPAddToolName, Input: `{"name":"browser","source_url":"https://example.com/docs","stdio":{"command":"new-server"}}`})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.Contains(t, response.Content, "already_configured_differently")
}

func TestMCPAddCorrectsExistingConfigurationWithReplace(t *testing.T) {
	t.Parallel()
	cfg := config.NewTestStore(&config.Config{MCP: map[string]config.MCPConfig{
		"browser": {Type: config.MCPStdio, Command: "npx", URL: "package-name"},
	}})
	var written config.MCPConfig
	tool := newMCPAddTool(cfg, mcpAddDeps{
		authorize: allowMCPAdd,
		setConfig: func(_ config.Scope, _ string, value any) error {
			written = value.(config.MCPConfig)
			return nil
		},
		refresh: func(context.Context, *config.ConfigStore, string) (map[string]error, error) {
			return map[string]error{"browser": nil}, nil
		},
		getState: func(string) (mcp.ClientInfo, bool) {
			return mcp.ClientInfo{State: mcp.StateConnected, Counts: mcp.Counts{Tools: 2}}, true
		},
	})

	response, err := tool.Run(verifiedMCPAddContext(t), fantasy.ToolCall{Name: MCPAddToolName, Input: `{"name":"browser","source_url":"https://example.com/docs","replace":true,"stdio":{"command":"npx","args":["-y","package-name"]}}`})
	require.NoError(t, err)
	require.False(t, response.IsError)
	require.Empty(t, written.URL)
	require.Equal(t, []string{"-y", "package-name"}, written.Args)
	require.Contains(t, response.Content, "config=updated")
}

func TestMCPAddReturnsExactStartupFailure(t *testing.T) {
	t.Parallel()
	cfg := config.NewTestStore(&config.Config{})
	removed := false
	tool := newMCPAddTool(cfg, mcpAddDeps{
		authorize: allowMCPAdd,
		setConfig: func(config.Scope, string, any) error { return nil },
		removeConfig: func(_ config.Scope, key string) error {
			require.Equal(t, "mcp.github", key)
			removed = true
			return nil
		},
		refresh: func(context.Context, *config.ConfigStore, string) (map[string]error, error) {
			return map[string]error{"github": errors.New("GITHUB_PERSONAL_ACCESS_TOKEN is required")}, nil
		},
	})

	response, err := tool.Run(verifiedMCPAddContext(t), fantasy.ToolCall{Name: MCPAddToolName, Input: `{"name":"github","source_url":"https://github.com/github/github-mcp-server","stdio":{"command":"github-mcp-server"}}`})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.True(t, response.StopTurn)
	require.True(t, removed)
	require.Contains(t, response.Content, "GITHUB_PERSONAL_ACCESS_TOKEN is required")
	require.Contains(t, response.Content, "configuration_present=false")
	require.Contains(t, response.Content, "configuration_changed=false")
	require.Contains(t, response.Content, "rollback=removed")
}

func TestMCPAddRejectsUnknownRuntimeToolFilterAndRollsBack(t *testing.T) {
	t.Parallel()

	cfg := config.NewTestStore(&config.Config{})
	removed := false
	tool := newMCPAddTool(cfg, mcpAddDeps{
		authorize: allowMCPAdd,
		setConfig: func(config.Scope, string, any) error { return nil },
		removeConfig: func(_ config.Scope, key string) error {
			require.Equal(t, "mcp.search", key)
			removed = true
			return nil
		},
		refresh: func(context.Context, *config.ConfigStore, string) (map[string]error, error) {
			return map[string]error{"search": nil}, nil
		},
		getState: func(string) (mcp.ClientInfo, bool) {
			return mcp.ClientInfo{State: mcp.StateConnected, Counts: mcp.Counts{Tools: 1}}, true
		},
		getToolFilterInfo: func(string) (mcp.ToolFilterInfo, bool) {
			return mcp.ToolFilterInfo{
				Advertised:        []string{"searchCode"},
				Usable:            []string{"searchCode"},
				UnmatchedDisabled: []string{"search"},
			}, true
		},
	})

	response, err := tool.Run(verifiedMCPAddContext(t), fantasy.ToolCall{Name: MCPAddToolName, Input: `{"name":"search","http":{"url":"https://example.com/mcp"},"disabled_tools":["search"]}`})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.True(t, response.StopTurn)
	require.True(t, removed)
	require.Contains(t, response.Content, "unknown tool filters")
	require.Contains(t, response.Content, `advertised_tools=["searchCode"]`)
	require.Contains(t, response.Content, "rollback=removed")
}

func TestMCPAddRestoresExistingConfigurationAfterStartupFailure(t *testing.T) {
	t.Parallel()
	existing := config.MCPConfig{Type: config.MCPStdio, Command: "old-server"}
	cfg := config.NewTestStore(&config.Config{MCP: map[string]config.MCPConfig{"browser": existing}})
	var writes []config.MCPConfig
	tool := newMCPAddTool(cfg, mcpAddDeps{
		authorize: allowMCPAdd,
		setConfig: func(_ config.Scope, _ string, value any) error {
			writes = append(writes, value.(config.MCPConfig))
			return nil
		},
		refresh: func(context.Context, *config.ConfigStore, string) (map[string]error, error) {
			return map[string]error{"browser": errors.New("candidate failed")}, nil
		},
	})

	response, err := tool.Run(verifiedMCPAddContext(t), fantasy.ToolCall{Name: MCPAddToolName, Input: `{"name":"browser","source_url":"https://example.com/docs","replace":true,"stdio":{"command":"new-server"}}`})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.True(t, response.StopTurn)
	require.Equal(t, []config.MCPConfig{{Type: config.MCPStdio, Command: "new-server"}, existing}, writes)
	require.Contains(t, response.Content, "rollback=restored")
	require.Contains(t, response.Content, "configuration_changed=false")
}
