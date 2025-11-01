package commands

import (
	"slices"
)

// buildFilteredTools builds a filtered list of tool names based on allowed-tools frontmatter.
//
// The function:
//   - Gets all available Crush tools using AllAvailableTools()
//   - If allowedTools is empty, returns all available tools (default behavior)
//   - Otherwise, filters tools to only include those in the allowedTools list
//   - Returns a slice of tool names ready to be used for agent configuration
//
// Parameters:
//   - allowedTools: List of tool names from command frontmatter (may be empty)
//
// Returns a slice of filtered tool names.
// If allowedTools is empty, returns all available tools.
func buildFilteredTools(allowedTools []string) []string {
	// Get all available tools
	allTools := AllAvailableTools()

	// If no restrictions, return all tools
	if len(allowedTools) == 0 {
		return allTools
	}

	// Filter tools to only include those in allowedTools
	filtered := make([]string, 0, len(allowedTools))
	for _, tool := range allTools {
		if slices.Contains(allowedTools, tool) {
			filtered = append(filtered, tool)
		}
	}

	return filtered
}

