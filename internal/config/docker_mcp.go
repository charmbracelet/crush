package config

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// DockerMCPName is the name of the Docker MCP configuration.
const DockerMCPName = "docker"

// IsDockerMCPAvailable checks if Docker MCP is available by running
// 'docker mcp version'.
func IsDockerMCPAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "mcp", "version")
	err := cmd.Run()
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
func (c *Config) EnableDockerMCP() error {
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
	if c.MCP == nil {
		c.MCP = make(map[string]MCPConfig)
	}
	c.MCP[DockerMCPName] = mcpConfig

	// Persist to config file.
	if err := c.SetConfigField("mcp."+DockerMCPName, mcpConfig); err != nil {
		return fmt.Errorf("failed to persist docker mcp configuration: %w", err)
	}

	return nil
}

// DisableDockerMCP removes Docker MCP configuration and persists the change.
func (c *Config) DisableDockerMCP() error {
	if c.MCP == nil {
		return nil
	}

	// Remove from in-memory config.
	delete(c.MCP, DockerMCPName)

	// Persist to config file by setting to null.
	if err := c.SetConfigField("mcp", c.MCP); err != nil {
		return fmt.Errorf("failed to persist docker mcp removal: %w", err)
	}

	return nil
}
