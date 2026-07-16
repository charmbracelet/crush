package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// newTestAppWorkspace creates an AppWorkspace backed by a ConfigStore
// whose workingDir points at the given temp directory. The app field is
// nil because MCPReconnect only touches w.store.
func newTestAppWorkspace(t *testing.T, workingDir string, cfg *config.Config) *AppWorkspace {
	t.Helper()
	store := config.NewTestStore(cfg)
	config.SetTestStoreWorkingDir(store, workingDir)
	return &AppWorkspace{store: store}
}

// newLoadedAppWorkspace loads a real ConfigStore from workingDir (so
// MCPReconnect's ReloadFromDisk reads the on-disk crush.json) and isolates
// the global config from the host/CI so only test-provided config is visible.
// The app field is nil because MCPReconnect only touches w.store.
func newLoadedAppWorkspace(t *testing.T, workingDir string) *AppWorkspace {
	t.Helper()
	globalDir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_CONFIG", globalDir)
	t.Setenv("CRUSH_GLOBAL_DATA", globalDir)
	store, err := config.Load(workingDir, workingDir, false)
	require.NoError(t, err)
	return &AppWorkspace{store: store}
}

// writeCrushConfig writes a crush.json containing a valid provider and model
// selection (so ReloadFromDisk's provider/model setup succeeds) plus the given
// MCP section. A nil mcp writes a config with no MCP servers.
func writeCrushConfig(t *testing.T, path string, mcp map[string]any) {
	t.Helper()
	cfg := map[string]any{
		"providers": map[string]any{
			"openai": map[string]any{
				"api_key": "test-key",
				"models":  []any{map[string]any{"id": "gpt-4", "name": "GPT-4"}},
			},
		},
		"models": map[string]any{
			"large": map[string]any{"provider": "openai", "model": "gpt-4"},
		},
	}
	if mcp != nil {
		cfg["mcp"] = mcp
	}
	writeJSON(t, path, cfg)
}

// disabledStdioMCP is a minimal disabled stdio MCP entry — disabled so
// InitializeSingle marks it StateDisabled without spawning a process.
func disabledStdioMCP() map[string]any {
	return map[string]any{"type": "stdio", "command": "echo", "disabled": true}
}

// TestMCPReconnect_HappyPath verifies that MCPReconnect reloads the
// config from disk before re-initialising the MCP server. We write a
// config with a disabled MCP, then confirm the reloaded config is used.
// Not parallel: uses t.Setenv to isolate the global config.
func TestMCPReconnect_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "crush.json")

	writeCrushConfig(t, configPath, map[string]any{
		"test-server": disabledStdioMCP(),
	})

	// Load a real store from disk; MCPReconnect reloads the same crush.json.
	w := newLoadedAppWorkspace(t, tmpDir)

	err := w.MCPReconnect(t.Context(), "test-server")
	require.NoError(t, err, "MCPReconnect should succeed when config reloads from disk")

	// After reconnect the on-disk config (with test-server disabled)
	// should be the live config.
	reloaded := w.store.Config()
	mcp, ok := reloaded.MCP["test-server"]
	require.True(t, ok, "test-server should exist in reloaded config")
	require.True(t, mcp.Disabled, "test-server should be disabled per on-disk config")

	// A disabled MCP is set to StateDisabled by InitializeSingle.
	info, ok := mcptools.GetState("test-server")
	require.True(t, ok, "test-server should have a state after reconnect")
	require.Equal(t, mcptools.StateDisabled, info.State)

	t.Cleanup(func() {
		_ = mcptools.DisableSingle(w.store, "test-server")
	})
}

// TestMCPReconnect_ConfigChangedOnDisk verifies that changes to the
// config file made AFTER the store was initialised are picked up by
// MCPReconnect — the core behaviour this feature adds.
// Not parallel: uses t.Setenv to isolate the global config.
func TestMCPReconnect_ConfigChangedOnDisk(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "crush.json")

	// Load a store from a config that has no MCP servers yet.
	writeCrushConfig(t, configPath, nil)
	w := newLoadedAppWorkspace(t, tmpDir)

	// The added server must not be visible until the reload picks it up.
	require.NotContains(t, w.store.Config().MCP, "added-server",
		"added-server should not be in the initially-loaded config")

	// Now write a new config that adds a disabled MCP server.
	writeCrushConfig(t, configPath, map[string]any{
		"added-server": disabledStdioMCP(),
	})

	// Reconnect should pick up the newly-added server from disk.
	err := w.MCPReconnect(t.Context(), "added-server")
	require.NoError(t, err, "MCPReconnect should reload config and find the new MCP server")

	reloaded := w.store.Config()
	_, ok := reloaded.MCP["added-server"]
	require.True(t, ok, "added-server should be in config after disk reload")

	t.Cleanup(func() {
		_ = mcptools.DisableSingle(w.store, "added-server")
	})
}

// TestMCPReconnect_ReloadFails verifies that when ReloadFromDisk fails
// (e.g., workingDir is empty), MCPReconnect falls back gracefully to
// the existing in-memory config instead of returning an error.
func TestMCPReconnect_ReloadFails(t *testing.T) {
	t.Parallel()

	// Config has the MCP in-memory but workingDir is empty so
	// ReloadFromDisk will fail. InitializeSingle should still find the
	// server in the in-memory config and proceed.
	cfg := &config.Config{
		MCP: map[string]config.MCPConfig{
			"fallback-server": {
				Type:     "stdio",
				Command:  "echo",
				Disabled: true,
			},
		},
	}
	// workingDir is "" → ReloadFromDisk returns an error.
	w := newTestAppWorkspace(t, "", cfg)

	err := w.MCPReconnect(t.Context(), "fallback-server")
	require.NoError(t, err, "MCPReconnect should not fail even when ReloadFromDisk errors")

	// The MCP should have been initialised from the in-memory config.
	info, ok := mcptools.GetState("fallback-server")
	require.True(t, ok, "fallback-server should have state after reconnect")
	require.Equal(t, mcptools.StateDisabled, info.State)

	t.Cleanup(func() {
		_ = mcptools.DisableSingle(w.store, "fallback-server")
	})
}

// TestMCPReconnect_ReloadFailsThenServerNotInConfig verifies the
// unhappy path where ReloadFromDisk fails AND the server doesn't exist
// in the in-memory config. InitializeSingle should return an error.
func TestMCPReconnect_ReloadFailsThenServerNotInConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{MCP: map[string]config.MCPConfig{}}
	w := newTestAppWorkspace(t, "", cfg)

	err := w.MCPReconnect(t.Context(), "nonexistent-server")
	require.Error(t, err, "MCPReconnect should error when server not in config after failed reload")
	require.Contains(t, err.Error(), "nonexistent-server")
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))
}
