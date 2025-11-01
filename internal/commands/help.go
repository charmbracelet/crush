package commands

import (
	"fmt"
	"sort"
	"strings"
)

// HelpHandler handles the execution of the `\help` command, which lists all
// available commands with their descriptions and argument hints.
//
// The help handler:
//   - Loads all commands from the registry
//   - Groups commands by namespace for organized display
//   - Formats output with descriptions and argument hints
//   - Shows source indicators (project, user, XDG)
//
// Usage:
//
//	registry := NewRegistry(workingDir)
//	_, err := registry.LoadCommands()
//	if err != nil {
//	    // handle error
//	}
//
//	handler := NewHelpHandler(registry)
//	output := handler.GenerateHelp()
type HelpHandler struct {
	registry Registry
}

// NewHelpHandler creates a new help command handler with the given registry.
//
// Parameters:
//   - registry: The command registry to load commands from
//
// Returns a new HelpHandler instance.
func NewHelpHandler(registry Registry) *HelpHandler {
	return &HelpHandler{
		registry: registry,
	}
}

// GenerateHelp generates the help output listing all available commands.
//
// The output includes:
//   - Commands grouped by namespace
//   - Command descriptions
//   - Argument hints (if available)
//   - Source indicators
//   - Built-in help command
//
// Returns a formatted string ready for display.
func (h *HelpHandler) GenerateHelp() string {
	commands := h.registry.ListCommands()

	// Add built-in help command to the list
	helpCommand := Command{
		Name:        "help",
		Description: "Show a list of all available commands and their descriptions.",
	}
	commands = append(commands, helpCommand)

	// Group commands by namespace
	grouped := groupCommandsByNamespace(commands)

	// Build help output
	var output strings.Builder
	output.WriteString("Available Commands:\n\n")

	// Sort namespaces for consistent output
	namespaces := make([]string, 0, len(grouped))
	for ns := range grouped {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	// Output root commands first (no namespace)
	if rootCmds, hasRoot := grouped[""]; hasRoot {
		output.WriteString("Root Commands:\n\n")
		for _, cmd := range rootCmds {
			h.formatCommand(&output, cmd)
		}
	}

	// Output namespaced commands with clear section headers
	for _, ns := range namespaces {
		if ns == "" {
			continue // Already handled above
		}
		// Use clearer section header format
		// Capitalize first letter of namespace for better readability
		nsTitle := ns
		if len(ns) > 0 {
			nsTitle = strings.ToUpper(ns[:1]) + ns[1:]
		}
		output.WriteString(fmt.Sprintf("%s Commands:\n\n", nsTitle))
		for _, cmd := range grouped[ns] {
			h.formatCommand(&output, cmd)
		}
	}

	return output.String()
}

// formatCommand formats a single command for display in help output.
// Command names and arguments are styled using markdown inline code formatting.
func (h *HelpHandler) formatCommand(output *strings.Builder, cmd Command) {
	// Build the command name with arguments
	commandText := fmt.Sprintf("\\%s", cmd.Name)
	if cmd.ArgumentHint != "" {
		commandText += " " + cmd.ArgumentHint
	}

	// Format: command name/args (in inline code for styling) - description (source)
	// Using markdown inline code (backticks) which will be styled by the markdown renderer
	output.WriteString("  `")
	output.WriteString(commandText)
	output.WriteString("`")

	if cmd.Description != "" {
		output.WriteString(fmt.Sprintf(" - %s", cmd.Description))
	}

	// Add source indicator in muted style
	if cmd.Source != "" {
		output.WriteString(fmt.Sprintf(" (%s)", cmd.Source))
	}
	output.WriteString("\n\n")
}

// groupCommandsByNamespace groups commands by their namespace.
//
// Commands with the same namespace are grouped together. Root commands
// (no namespace) are grouped under an empty string key.
//
// Parameters:
//   - commands: List of commands to group
//
// Returns a map of namespace to commands.
func groupCommandsByNamespace(commands []Command) map[string][]Command {
	grouped := make(map[string][]Command)

	for _, cmd := range commands {
		ns := cmd.Namespace
		if ns == "" {
			ns = "" // Root commands
		}
		grouped[ns] = append(grouped[ns], cmd)
	}

	// Sort commands within each namespace lexicographically
	for ns := range grouped {
		sort.Slice(grouped[ns], func(i, j int) bool {
			return strings.Compare(grouped[ns][i].Name, grouped[ns][j].Name) < 0
		})
	}

	return grouped
}
