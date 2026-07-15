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

// TestMCPReconnect_HappyPath verifies that MCPReconnect reloads the
// config from disk before re-initialising the MCP server. We write a
// config with a disabled MCP, then confirm the reloaded config is used.
func TestMCPReconnect_HappyPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "crush.json")

	writeJSON(t, configPath, map[string]any{
		"mcp": map[string]any{
			"test-server": map[string]any{
				"type":     "stdio",
				"command":  "echo",
				"disabled": true,
			},
		},
	})

	// Seed the store with an empty config; the on-disk version will be
	// loaded during MCPReconnect via ReloadFromDisk.
	cfg := &config.Config{MCP: map[string]config.MCPConfig{}}
	w := newTestAppWorkspace(t, tmpDir, cfg)

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
func TestMCPReconnect_ConfigChangedOnDisk(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "crush.json")

	// Write config with no MCP servers.
	writeJSON(t, configPath, map[string]any{})

	cfg := &config.Config{MCP: map[string]config.MCPConfig{}}
	w := newTestAppWorkspace(t, tmpDir, cfg)

	// Now write a new config that adds a disabled MCP server.
	writeJSON(t, configPath, map[string]any{
		"mcp": map[string]any{
			"added-server": map[string]any{
				"type":     "stdio",
				"command":  "echo",
				"disabled": true,
			},
		},
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
