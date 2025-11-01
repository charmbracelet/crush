package commands

import (
	"log/slog"
	"slices"
	"strings"
)

// AllAvailableTools returns the list of all available Crush tool names.
// This list should match the tools defined in internal/config/config.go allToolNames().
func AllAvailableTools() []string {
	return []string{
		"agent",
		"bash",
		"download",
		"edit",
		"multiedit",
		"lsp_diagnostics",
		"lsp_references",
		"fetch",
		"glob",
		"grep",
		"ls",
		"sourcegraph",
		"view",
		"write",
	}
}

// validateAllowedTools validates the allowed-tools frontmatter values against Crush's available tools.
// Invalid tool names are logged as warnings and filtered out.
// Returns the filtered list containing only valid tool names.
func validateAllowedTools(allowedTools []string, commandPath string) []string {
	if len(allowedTools) == 0 {
		return allowedTools
	}

	availableTools := AllAvailableTools()
	var validTools []string
	var invalidTools []string

	for _, tool := range allowedTools {
		// Trim whitespace in case tools were specified as comma-separated string
		tool = strings.TrimSpace(tool)
		if tool == "" {
			continue
		}

		if slices.Contains(availableTools, tool) {
			validTools = append(validTools, tool)
		} else {
			invalidTools = append(invalidTools, tool)
		}
	}

	// Log warnings for invalid tools
	if len(invalidTools) > 0 {
		slog.Warn("Invalid tool names in allowed-tools",
			"command_path", commandPath,
			"invalid_tools", invalidTools,
			"valid_tools", validTools,
		)
	}

	return validTools
}

