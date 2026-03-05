package agent

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoderPromptDockerMCPBias(t *testing.T) {
	t.Run("includes Docker MCP bias when enabled", func(t *testing.T) {
		cfg, err := config.Init(t.TempDir(), "", false)
		require.NoError(t, err)
		if cfg.MCP == nil {
			cfg.MCP = map[string]config.MCPConfig{}
		}
		cfg.MCP[config.DockerMCPName] = config.MCPConfig{Type: config.MCPStdio, Command: "docker"}

		prompt, err := coderPrompt()
		require.NoError(t, err)

		systemPrompt, err := prompt.Build(context.Background(), "", "", *cfg)
		require.NoError(t, err)
		assert.Contains(t, systemPrompt, "you must attempt Docker MCP discovery first")
	})

	t.Run("does not include Docker MCP bias when disabled", func(t *testing.T) {
		cfg, err := config.Init(t.TempDir(), "", false)
		require.NoError(t, err)
		delete(cfg.MCP, config.DockerMCPName)

		prompt, err := coderPrompt()
		require.NoError(t, err)

		systemPrompt, err := prompt.Build(context.Background(), "", "", *cfg)
		require.NoError(t, err)
		assert.NotContains(t, systemPrompt, "you must attempt Docker MCP discovery first")
	})
}
