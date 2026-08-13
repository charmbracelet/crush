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
func IsDockerMCPAvailable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := dockerMCPVersionRunner(ctx)
	return err == nil
}

// DockerMCPAvailabilityCached returns the cached Docker MCP availability and
// whether the cached value is still fresh.
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

// RefreshDockerMCPAvailability refreshes and caches Docker MCP availability.
func RefreshDockerMCPAvailability() bool {
	available := IsDockerMCPAvailable(context.Background())
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

// DockerMCPConfig returns the default Docker MCP stdio configuration.
func DockerMCPConfig() MCPConfig {
	return MCPConfig{
		Type:     MCPStdio,
		Command:  "docker",
		Args:     []string{"mcp", "gateway", "run"},
		Disabled: false,
	}
}

// PrepareDockerMCPConfig validates Docker MCP availability and stages the
// Docker MCP configuration in memory.
func (s *ConfigStore) PrepareDockerMCPConfig(ctx context.Context) (MCPConfig, error) {
	if !IsDockerMCPAvailable(ctx) {
		return MCPConfig{}, fmt.Errorf("docker mcp is not available, please ensure docker is installed and 'docker mcp version' succeeds")
	}

	mcpConfig := DockerMCPConfig()
	// In-memory only; persistence happens in PersistDockerMCPConfig.
	s.mutateInMemory(func(c *Config) {
		if c.MCP == nil {
			c.MCP = make(map[string]MCPConfig)
		}
		c.MCP[DockerMCPName] = mcpConfig
	})
	return mcpConfig, nil
}

// PersistDockerMCPConfig persists a previously prepared Docker MCP
// configuration to the global config file.
func (s *ConfigStore) PersistDockerMCPConfig(ctx context.Context, mcpConfig MCPConfig) error {
	if err := s.SetConfigField(ctx, ScopeGlobal, "mcp."+DockerMCPName, mcpConfig); err != nil {
		return fmt.Errorf("failed to persist docker mcp configuration: %w", err)
	}
	return nil
}

// EnableDockerMCP adds Docker MCP configuration and persists it.
func (s *ConfigStore) EnableDockerMCP(ctx context.Context) error {
	mcpConfig, err := s.PrepareDockerMCPConfig(ctx)
	if err != nil {
		return err
	}
	if err := s.PersistDockerMCPConfig(ctx, mcpConfig); err != nil {
		return err
	}
	return nil
}

// DisableDockerMCP removes Docker MCP configuration and persists the change.
func (s *ConfigStore) DisableDockerMCP(ctx context.Context) error {
	return s.update(ctx, ScopeGlobal, func(c *Config) map[string]any {
		if c.MCP == nil {
			return nil
		}
		delete(c.MCP, DockerMCPName)
		return map[string]any{"mcp": c.MCP}
	})
}

// RemoveDockerMCPInMemory removes the Docker MCP entry from the in-memory
// config via copy-on-write, without persisting to disk. It rolls back a
// staged PrepareDockerMCPConfig when starting or persisting the server
// fails, so callers must not mutate Config().MCP directly.
func (s *ConfigStore) RemoveDockerMCPInMemory() {
	s.mutateInMemory(func(c *Config) {
		delete(c.MCP, DockerMCPName)
	})
}
