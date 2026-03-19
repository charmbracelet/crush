package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/env"
	"github.com/stretchr/testify/require"
)

var errDockerUnavailable = errors.New("docker unavailable")

func setDockerMCPVersionRunner(t *testing.T, runner func(context.Context) error) {
	t.Helper()
	orig := dockerMCPVersionRunner
	dockerMCPVersionRunner = runner
	t.Cleanup(func() {
		dockerMCPVersionRunner = orig
	})
}

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
	t.Run("adds docker mcp to config", func(t *testing.T) {
		setDockerMCPVersionRunner(t, func(context.Context) error { return nil })

		// Create a temporary directory for config.
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "crush.json")

		cfg := &Config{
			MCP: make(map[string]MCPConfig),
		}
		store := &ConfigStore{
			config:         cfg,
			globalDataPath: configPath,
			resolver:       NewShellVariableResolver(env.New()),
		}

		err := store.EnableDockerMCP()
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
		setDockerMCPVersionRunner(t, func(context.Context) error { return errDockerUnavailable })

		// Create a temporary directory for config.
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "crush.json")

		cfg := &Config{
			MCP: make(map[string]MCPConfig),
		}
		store := &ConfigStore{
			config:         cfg,
			globalDataPath: configPath,
			resolver:       NewShellVariableResolver(env.New()),
		}

		err := store.EnableDockerMCP()
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
		}
		store := &ConfigStore{
			config:         cfg,
			globalDataPath: configPath,
			resolver:       NewShellVariableResolver(env.New()),
		}

		// Verify it's enabled first.
		require.True(t, cfg.IsDockerMCPEnabled())

		err := store.DisableDockerMCP()
		require.NoError(t, err)

		// Check in-memory config.
		require.False(t, cfg.IsDockerMCPEnabled())
		_, exists := cfg.MCP[DockerMCPName]
		require.False(t, exists)
	})

	t.Run("does nothing when MCP is nil", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{
			MCP: nil,
		}
		store := &ConfigStore{
			config:         cfg,
			globalDataPath: filepath.Join(t.TempDir(), "crush.json"),
			resolver:       NewShellVariableResolver(env.New()),
		}

		err := store.DisableDockerMCP()
		require.NoError(t, err)
	})
}

func TestEnableDockerMCPWithRealDockerWhenAvailable(t *testing.T) {
	t.Parallel()

	if !IsDockerMCPAvailable() {
		t.Skip("docker mcp not available on this machine")
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "crush.json")

	cfg := &Config{
		MCP: make(map[string]MCPConfig),
	}
	store := &ConfigStore{
		config:         cfg,
		globalDataPath: configPath,
		resolver:       NewShellVariableResolver(env.New()),
	}

	err := store.EnableDockerMCP()
	require.NoError(t, err)
	require.True(t, cfg.IsDockerMCPEnabled())
}
