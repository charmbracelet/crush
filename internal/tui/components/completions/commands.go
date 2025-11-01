package completions

import (
	"fmt"

	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
)

// CommandCompletionProvider provides command completions for the editor.
// It integrates with the existing completion system and loads commands from the registry.
//
// The provider converts Command structs from the registry into Completion items
// that can be displayed in the completion popup. It supports fuzzy matching
// across command names and namespaces for efficient command discovery.
//
// Usage:
//
//	registry := commands.NewRegistry(projectDir)
//	_, err := registry.LoadCommands()
//	if err != nil {
//	    // handle error
//	}
//
//	provider := NewCommandCompletionProvider(registry)
//	completions := provider.GetCompletions() // Returns []Completion
type CommandCompletionProvider struct {
	registry commands.Registry
}

// NewCommandCompletionProvider creates a new command completion provider
// with the given command registry.
//
// Parameters:
//   - registry: The command registry to load commands from
//
// Returns a new CommandCompletionProvider instance.
func NewCommandCompletionProvider(registry commands.Registry) *CommandCompletionProvider {
	return &CommandCompletionProvider{
		registry: registry,
	}
}

// commandToCompletionItem converts a Command struct into a CompletionItem that can be
// displayed in the completion popup.
//
// The completion item includes:
//   - Title: Command name (including namespace if applicable) with description
//   - Value: The Command struct itself
//   - FilterValue: Full command name for fuzzy matching (e.g., "frontend:review-pr")
//
// Parameters:
//   - cmd: The Command struct to convert
//
// Returns a CompletionItem[commands.Command] ready for display in the completion popup.
func commandToCompletionItem(cmd commands.Command) list.CompletionItem[commands.Command] {
	// Build the display text: command name with description if available
	displayText := cmd.Name
	if cmd.Description != "" {
		displayText = fmt.Sprintf("%s - %s", cmd.Name, cmd.Description)
	}

	// Create completion item with command as the value
	// The displayText starts with the command name (including namespace), which ensures
	// that FilterValue() (which returns the text) uses the full command name for fuzzy matching.
	// This allows matching across namespace and command name (e.g., "cbut" matches "frontend:components:button")
	t := styles.CurrentTheme()
	item := list.NewCompletionItem(
		displayText,
		cmd,
		list.WithCompletionBackgroundColor(t.BgSubtle),
	)

	return item
}

// loadCommandCompletions loads all commands from the registry and converts them
// to completion items ready for display in the completion popup.
//
// The function:
//   - Calls registry.ListCommands() to get all available commands
//   - Converts each command to a CompletionItem using commandToCompletionItem
//   - Returns a slice of completion items, ordered as returned by the registry
//   - Handles empty command lists gracefully (returns empty slice)
//
// Returns a slice of CompletionItem[commands.Command] ready for use in the completion system.
func (p *CommandCompletionProvider) loadCommandCompletions() []list.CompletionItem[commands.Command] {
	// Get all commands from registry
	allCommands := p.registry.ListCommands()

	// Handle empty command list
	if len(allCommands) == 0 {
		return []list.CompletionItem[commands.Command]{}
	}

	// Convert all commands to completion items
	completionItems := make([]list.CompletionItem[commands.Command], 0, len(allCommands))
	for _, cmd := range allCommands {
		item := commandToCompletionItem(cmd)
		completionItems = append(completionItems, item)
	}

	return completionItems
}

