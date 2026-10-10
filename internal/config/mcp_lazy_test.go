package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// boolPtr builds the tri-state pointers the lazy flags use.
func boolPtr(v bool) *bool { return &v }

func TestOptionsGetLazyMCP(t *testing.T) {
	t.Parallel()

	require.True(t, (*Options)(nil).GetLazyMCP(), "a missing options block is still lazy")
	require.True(t, (&Options{}).GetLazyMCP(), "laziness is on by default")
	require.False(t, (&Options{LazyMCP: boolPtr(false)}).GetLazyMCP())
	require.True(t, (&Options{LazyMCP: boolPtr(true)}).GetLazyMCP())
}

func TestMCPConfigIsLazy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		server   MCPConfig
		options  *Options
		expected bool
	}{
		{
			name:     "defaults",
			server:   MCPConfig{},
			options:  &Options{},
			expected: true,
		},
		{
			name:     "global off hides nothing",
			server:   MCPConfig{},
			options:  &Options{LazyMCP: boolPtr(false)},
			expected: false,
		},
		{
			name:     "per-server pin wins over global",
			server:   MCPConfig{Lazy: boolPtr(false)},
			options:  &Options{},
			expected: false,
		},
		{
			name:     "per-server lazy wins over global off",
			server:   MCPConfig{Lazy: boolPtr(true)},
			options:  &Options{LazyMCP: boolPtr(false)},
			expected: true,
		},
		{
			name:     "nil options falls back to on",
			server:   MCPConfig{},
			options:  nil,
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.server.IsLazy(tc.options))
		})
	}
}

func TestConfigAnyMCPLazy(t *testing.T) {
	t.Parallel()

	// Nothing configured leaves no reason to register mcp_search at all.
	require.False(t, Config{Options: &Options{}}.AnyMCPLazy())

	// A disabled server contributes nothing: its tools are never registered.
	require.False(t, Config{
		MCP:     map[string]MCPConfig{"a": {Disabled: true}},
		Options: &Options{},
	}.AnyMCPLazy())

	// Every server pinned means the eager list already has the schemas.
	require.False(t, Config{
		MCP: map[string]MCPConfig{
			"a": {Lazy: boolPtr(false)},
			"b": {Lazy: boolPtr(false)},
		},
		Options: &Options{},
	}.AnyMCPLazy())

	require.True(t, Config{
		MCP:     map[string]MCPConfig{"a": {Lazy: boolPtr(false)}, "b": {}},
		Options: &Options{},
	}.AnyMCPLazy(), "one unpinned server is enough")
}

func TestSetLazyMCPConfigWrites(t *testing.T) {
	t.Parallel()

	globalPath := filepath.Join(t.TempDir(), "crush.json")
	store := &ConfigStore{
		config:         &Config{MCP: map[string]MCPConfig{"docker": {Type: MCPStdio}}},
		globalDataPath: globalPath,
	}

	require.NoError(t, store.SetLazyMCPConfig(ScopeGlobal, false))
	data, err := os.ReadFile(globalPath)
	require.NoError(t, err)
	require.Equal(t, false, gjson.GetBytes(data, "options.lazy_mcp").Bool())
	require.False(t, store.Config().Options.GetLazyMCP())

	// Flipping the default must leave per-server entries alone.
	require.False(t, gjson.GetBytes(data, "mcp.docker.lazy").Exists())

	require.NoError(t, store.SetLazyMCPConfig(ScopeGlobal, true))
	require.True(t, store.Config().Options.GetLazyMCP())
}

func TestSetMCPServerLazyConfigRequiresConfiguredServer(t *testing.T) {
	t.Parallel()

	store := NewTestStore(&Config{MCP: map[string]MCPConfig{"docker": {}}})
	err := store.SetMCPServerLazyConfig(ScopeGlobal, "nope", boolPtr(true))
	require.ErrorContains(t, err, "not configured", "unknown servers must not create empty config entries")
}

func TestSetMCPServerLazyConfigWritesAndClears(t *testing.T) {
	t.Parallel()

	globalPath := filepath.Join(t.TempDir(), "crush.json")
	store := &ConfigStore{
		config:         &Config{MCP: map[string]MCPConfig{"docker": {Type: MCPStdio}}},
		globalDataPath: globalPath,
	}

	require.NoError(t, store.SetMCPServerLazyConfig(ScopeGlobal, "docker", boolPtr(false)))
	data, err := os.ReadFile(globalPath)
	require.NoError(t, err)
	require.Equal(t, false, gjson.GetBytes(data, "mcp.docker.lazy").Bool())

	// A nil flag is "follow the global default", so the key goes away.
	require.NoError(t, store.SetMCPServerLazyConfig(ScopeGlobal, "docker", nil))
	data, err = os.ReadFile(globalPath)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(data, "mcp.docker.lazy").Exists())
}
