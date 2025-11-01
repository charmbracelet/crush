package commands

import (
	"github.com/charmbracelet/crush/internal/config"
)

// buildRestrictedAgentConfig creates a modified agent config with restricted AllowedTools.
//
// This function:
//   - Takes the base agent config and a list of allowed tools from command frontmatter
//   - Builds a filtered tool list using buildFilteredTools
//   - Creates a new Agent config with restricted AllowedTools
//   - Preserves all other agent config fields
//
// Parameters:
//   - baseAgent: The base agent config (typically from config.Agents[config.AgentCoder])
//   - allowedTools: List of allowed tool names from command frontmatter (may be empty for all tools)
//
// Returns a new Agent config with AllowedTools set to the filtered list.
// If allowedTools is empty, the returned config will have all tools allowed.
func buildRestrictedAgentConfig(baseAgent config.Agent, allowedTools []string) config.Agent {
	// Build filtered tool list
	filteredTools := buildFilteredTools(allowedTools)

	// Create modified agent config with restricted tools
	restrictedAgent := baseAgent
	restrictedAgent.AllowedTools = filteredTools

	return restrictedAgent
}

