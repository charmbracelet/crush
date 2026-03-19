package config

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

var dockerMCPVersionRunner = func(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "mcp", "version")
	return cmd.Run()
}

// DockerMCPName is the name of the Docker MCP configuration.
const DockerMCPName = "docker"

// IsDockerMCPAvailable checks if Docker MCP is available by running
// 'docker mcp version'.
func IsDockerMCPAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := dockerMCPVersionRunner(ctx)
	return err == nil
}

// IsDockerMCPEnabled checks if Docker MCP is already configured.
func (c *Config) IsDockerMCPEnabled() bool {
	if c.MCP == nil {
		return false
	}
	_, exists := c.MCP[DockerMCPName]
	return exists
}

// EnableDockerMCP adds Docker MCP configuration and persists it.
func (s *ConfigStore) EnableDockerMCP() error {
	if !IsDockerMCPAvailable() {
		return fmt.Errorf("docker mcp is not available, please ensure docker is installed and 'docker mcp version' succeeds")
	}

	mcpConfig := MCPConfig{
		Type:     MCPStdio,
		Command:  "docker",
		Args:     []string{"mcp", "gateway", "run"},
		Disabled: false,
	}

	// Add to in-memory config.
	if s.config.MCP == nil {
		s.config.MCP = make(map[string]MCPConfig)
	}
	s.config.MCP[DockerMCPName] = mcpConfig

	// Persist to config file.
	if err := s.SetConfigField(ScopeGlobal, "mcp."+DockerMCPName, mcpConfig); err != nil {
		return fmt.Errorf("failed to persist docker mcp configuration: %w", err)
	}

	return nil
}

// DisableDockerMCP removes Docker MCP configuration and persists the change.
func (s *ConfigStore) DisableDockerMCP() error {
	if s.config.MCP == nil {
		return nil
	}

	// Remove from in-memory config.
	delete(s.config.MCP, DockerMCPName)

	// Persist the updated MCP map to the config file.
	if err := s.SetConfigField(ScopeGlobal, "mcp", s.config.MCP); err != nil {
		return fmt.Errorf("failed to persist docker mcp removal: %w", err)
	}

	return nil
}
