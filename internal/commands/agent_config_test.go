package commands

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestBuildRestrictedAgentConfig_EmptyAllowedTools(t *testing.T) {
	baseAgent := config.Agent{
		ID:          "test",
		AllowedTools: []string{"view", "edit"},
	}

	result := buildRestrictedAgentConfig(baseAgent, []string{})

	// Should have all available tools
	allTools := AllAvailableTools()
	assert.Equal(t, allTools, result.AllowedTools)
	// Other fields should be preserved
	assert.Equal(t, baseAgent.ID, result.ID)
}

func TestBuildRestrictedAgentConfig_SpecificTools(t *testing.T) {
	baseAgent := config.Agent{
		ID:          "test",
		Name:        "Test Agent",
		AllowedTools: []string{"view", "edit", "bash"},
	}

	allowed := []string{"view", "edit"}

	result := buildRestrictedAgentConfig(baseAgent, allowed)

	// Should only have allowed tools (order may differ)
	assert.ElementsMatch(t, allowed, result.AllowedTools)
	// Other fields should be preserved
	assert.Equal(t, baseAgent.ID, result.ID)
	assert.Equal(t, baseAgent.Name, result.Name)
}

func TestBuildRestrictedAgentConfig_PreservesOtherFields(t *testing.T) {
	baseAgent := config.Agent{
		ID:           "test",
		Name:         "Test Agent",
		Description:  "Test description",
		Model:        config.SelectedModelTypeLarge,
		AllowedTools: []string{"view", "edit"},
		AllowedMCP:   map[string][]string{"mcp1": {"tool1"}},
	}

	result := buildRestrictedAgentConfig(baseAgent, []string{"view"})

	// AllowedTools should be filtered
	assert.Equal(t, []string{"view"}, result.AllowedTools)
	// Other fields should be preserved
	assert.Equal(t, baseAgent.ID, result.ID)
	assert.Equal(t, baseAgent.Name, result.Name)
	assert.Equal(t, baseAgent.Description, result.Description)
	assert.Equal(t, baseAgent.Model, result.Model)
	assert.Equal(t, baseAgent.AllowedMCP, result.AllowedMCP)
}

func TestBuildRestrictedAgentConfig_InvalidTools(t *testing.T) {
	baseAgent := config.Agent{
		ID:          "test",
		AllowedTools: []string{"view", "edit"},
	}

	// Include invalid tool names
	allowed := []string{"view", "InvalidTool"}

	result := buildRestrictedAgentConfig(baseAgent, allowed)

	// Should only include valid tools
	assert.Contains(t, result.AllowedTools, "view")
	assert.NotContains(t, result.AllowedTools, "InvalidTool")
}

