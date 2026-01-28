package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/stretchr/testify/require"
)

func TestIsDockerMCPEnabled(t *testing.T) {
	t.Parallel()

	t.Run("returns false when MCP is nil", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			MCP: nil,
		}
		require.False(t, cfg.IsDockerMCPEnabled())
	})

	t.Run("returns false when docker mcp not configured", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			MCP: make(map[string]MCPConfig),
		}
		require.False(t, cfg.IsDockerMCPEnabled())
	})

	t.Run("returns true when docker mcp is configured", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			MCP: map[string]MCPConfig{
				DockerMCPName: {
					Type:    MCPStdio,
					Command: "docker",
				},
			},
		}
		require.True(t, cfg.IsDockerMCPEnabled())
	})
}

func TestEnableDockerMCP(t *testing.T) {
	t.Parallel()

	t.Run("adds docker mcp to config", func(t *testing.T) {
		t.Parallel()

		// Create a temporary directory for config.
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "crush.json")

		cfg := &Config{
			MCP:            make(map[string]MCPConfig),
			dataConfigDir:  configPath,
			resolver:       NewShellVariableResolver(env.New()),
			knownProviders: []catwalk.Provider{},
		}

		// Only run this test if docker mcp is available.
		if !IsDockerMCPAvailable() {
			t.Skip("Docker MCP not available, skipping test")
		}

		err := cfg.EnableDockerMCP()
		require.NoError(t, err)

		// Check in-memory config.
		require.True(t, cfg.IsDockerMCPEnabled())
		mcpConfig, exists := cfg.MCP[DockerMCPName]
		require.True(t, exists)
		require.Equal(t, MCPStdio, mcpConfig.Type)
		require.Equal(t, "docker", mcpConfig.Command)
		require.Equal(t, []string{"mcp", "gateway", "run"}, mcpConfig.Args)
		require.False(t, mcpConfig.Disabled)

		// Check persisted config.
		data, err := os.ReadFile(configPath)
		require.NoError(t, err)
		require.Contains(t, string(data), "docker")
		require.Contains(t, string(data), "gateway")
	})

	t.Run("fails when docker mcp not available", func(t *testing.T) {
		t.Parallel()

		// Create a temporary directory for config.
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "crush.json")

		cfg := &Config{
			MCP:            make(map[string]MCPConfig),
			dataConfigDir:  configPath,
			resolver:       NewShellVariableResolver(env.New()),
			knownProviders: []catwalk.Provider{},
		}

		// Skip if docker mcp is actually available.
		if IsDockerMCPAvailable() {
			t.Skip("Docker MCP is available, skipping unavailable test")
		}

		err := cfg.EnableDockerMCP()
		require.Error(t, err)
		require.Contains(t, err.Error(), "docker mcp is not available")
	})
}

func TestDisableDockerMCP(t *testing.T) {
	t.Parallel()

	t.Run("removes docker mcp from config", func(t *testing.T) {
		t.Parallel()

		// Create a temporary directory for config.
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "crush.json")

		cfg := &Config{
			MCP: map[string]MCPConfig{
				DockerMCPName: {
					Type:     MCPStdio,
					Command:  "docker",
					Args:     []string{"mcp", "gateway", "run"},
					Disabled: false,
				},
			},
			dataConfigDir:  configPath,
			resolver:       NewShellVariableResolver(env.New()),
			knownProviders: []catwalk.Provider{},
		}

		// Verify it's enabled first.
		require.True(t, cfg.IsDockerMCPEnabled())

		err := cfg.DisableDockerMCP()
		require.NoError(t, err)

		// Check in-memory config.
		require.False(t, cfg.IsDockerMCPEnabled())
		_, exists := cfg.MCP[DockerMCPName]
		require.False(t, exists)
	})

	t.Run("does nothing when MCP is nil", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{
			MCP:            nil,
			dataConfigDir:  t.TempDir() + "/crush.json",
			resolver:       NewShellVariableResolver(env.New()),
			knownProviders: []catwalk.Provider{},
		}

		err := cfg.DisableDockerMCP()
		require.NoError(t, err)
	})
}
