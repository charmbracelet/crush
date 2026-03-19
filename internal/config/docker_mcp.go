package config

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

var dockerMCPVersionRunner = func(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "mcp", "version")
	return cmd.Run()
}

const dockerMCPAvailabilityTTL = 10 * time.Second

var dockerMCPAvailabilityCache struct {
	mu        sync.Mutex
	available bool
	checkedAt time.Time
	known     bool
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

func DockerMCPAvailabilityCached() (available bool, known bool) {
	dockerMCPAvailabilityCache.mu.Lock()
	defer dockerMCPAvailabilityCache.mu.Unlock()

	if !dockerMCPAvailabilityCache.known {
		return false, false
	}
	if time.Since(dockerMCPAvailabilityCache.checkedAt) > dockerMCPAvailabilityTTL {
		return dockerMCPAvailabilityCache.available, false
	}
	return dockerMCPAvailabilityCache.available, true
}

func RefreshDockerMCPAvailability() bool {
	available := IsDockerMCPAvailable()
	dockerMCPAvailabilityCache.mu.Lock()
	dockerMCPAvailabilityCache.available = available
	dockerMCPAvailabilityCache.checkedAt = time.Now()
	dockerMCPAvailabilityCache.known = true
	dockerMCPAvailabilityCache.mu.Unlock()
	return available
}

// IsDockerMCPEnabled checks if Docker MCP is already configured.
func (c *Config) IsDockerMCPEnabled() bool {
	if c.MCP == nil {
		return false
	}
	_, exists := c.MCP[DockerMCPName]
	return exists
}

func DockerMCPConfig() MCPConfig {
	return MCPConfig{
		Type:     MCPStdio,
		Command:  "docker",
		Args:     []string{"mcp", "gateway", "run"},
		Disabled: false,
	}
}

func (s *ConfigStore) PrepareDockerMCPConfig() (MCPConfig, error) {
	if !IsDockerMCPAvailable() {
		return MCPConfig{}, fmt.Errorf("docker mcp is not available, please ensure docker is installed and 'docker mcp version' succeeds")
	}

	mcpConfig := DockerMCPConfig()
	if s.config.MCP == nil {
		s.config.MCP = make(map[string]MCPConfig)
	}
	s.config.MCP[DockerMCPName] = mcpConfig
	return mcpConfig, nil
}

func (s *ConfigStore) PersistDockerMCPConfig(mcpConfig MCPConfig) error {
	if err := s.SetConfigField(ScopeGlobal, "mcp."+DockerMCPName, mcpConfig); err != nil {
		return fmt.Errorf("failed to persist docker mcp configuration: %w", err)
	}
	return nil
}

// EnableDockerMCP adds Docker MCP configuration and persists it.
func (s *ConfigStore) EnableDockerMCP() error {
	mcpConfig, err := s.PrepareDockerMCPConfig()
	if err != nil {
		return err
	}
	if err := s.PersistDockerMCPConfig(mcpConfig); err != nil {
		return err
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
