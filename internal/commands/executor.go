package commands

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/sahilm/fuzzy"
)

// Executor handles the execution of slash commands by coordinating between
// the command registry and the agent coordinator.
//
// The executor is responsible for:
//   - Looking up commands by name from the registry
//   - Parsing and substituting arguments in command content
//   - Resolving file references (@filename) and preparing attachments
//   - Filtering tools based on command's allowed-tools frontmatter
//   - Executing the command through the agent coordinator
//
// Example usage:
//
//	executor := NewExecutor(registry, coordinator, workingDir)
//	err := executor.Execute(ctx, sessionID, "frontend:review-pr", []string{"123", "high"})
type Executor interface {
	// Execute executes a slash command with the given arguments.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//   - sessionID: The session ID where the command will be executed
	//   - commandName: The full command name (e.g., "review-pr" or "frontend:review-pr")
	//   - args: Command arguments provided by the user
	//
	// Returns an error if the command cannot be found, parsed, or executed.
	Execute(ctx context.Context, sessionID string, commandName string, args []string) error
}

// executor is the concrete implementation of the Executor interface.
type executor struct {
	registry    Registry
	coordinator agent.Coordinator
	messages    message.Service
	workingDir  string
}

// NewExecutor creates a new command executor instance.
//
// Parameters:
//   - registry: The command registry for looking up commands
//   - coordinator: The agent coordinator for executing commands
//   - messages: The message service for creating messages directly
//   - workingDir: The working directory for resolving relative file paths
func NewExecutor(registry Registry, coordinator agent.Coordinator, messages message.Service, workingDir string) Executor {
	return &executor{
		registry:    registry,
		coordinator: coordinator,
		messages:    messages,
		workingDir:  workingDir,
	}
}

// Execute implements the Executor interface.
func (e *executor) Execute(ctx context.Context, sessionID string, commandName string, args []string) error {
	// 0. Handle special built-in commands (before registry lookup)
	if commandName == "help" {
		return e.executeHelpCommand(ctx, sessionID)
	}

	// 1. Look up command from registry
	cmd, err := e.registry.FindCommand(commandName)
	if err != nil {
		slog.Warn("Command not found",
			"command", commandName,
			"error", err,
		)

		// Suggest similar command names
		suggestions := e.suggestSimilarCommands(commandName)
		errorMsg := fmt.Sprintf("command '%s' not found", commandName)
		if len(suggestions) > 0 {
			errorMsg += fmt.Sprintf(". Did you mean: %s?", strings.Join(suggestions, ", "))
		}

		return fmt.Errorf("%s: %w", errorMsg, err)
	}

	// 2. Validate arguments match command requirements
	requiredArgs := extractRequiredArguments(cmd.Content, cmd.ArgumentHint)
	if err := validateArguments(args, requiredArgs, commandName); err != nil {
		return err
	}

	// 3. Process command content (substitute arguments)
	processedContent := processCommandContent(cmd.Content, args)

	// If arguments were provided but not all are referenced in content, append them
	// This ensures the agent receives all arguments even if only some are referenced
	// Skip if requiredArgs.RequiredCount is -1 (means $ARGS or $ARGUMENTS is used, which covers all)
	// Check against original content to see if all required arguments are referenced
	// We check BEFORE substitution to see what placeholders exist in the original content
	if len(args) > 0 && requiredArgs.RequiredCount != -1 && !hasAllRequiredArguments(cmd.Content, requiredArgs.RequiredCount) {
		argsStr := strings.Join(args, " ")
		// Append arguments to processed content (after substitution)
		// This ensures the agent receives all arguments even if content only references some
		processedContent = processedContent + "\n\nArguments: " + argsStr
	}

	// 4. Resolve file references (@filename) and build attachments
	fileRefs := parseFileReferences(processedContent)
	resolvedPaths := resolveFilePaths(fileRefs, e.workingDir)
	fileContents := readFileContents(resolvedPaths)

	// Check if any files failed to be read
	// Note: Files with empty content are considered failed reads
	// (readFileContents only sets empty content when readSingleFile returns an error)
	var fileErrors []string
	for _, fc := range fileContents {
		if fc.Content == "" && fc.Path != "" {
			// File was attempted but couldn't be read
			fileErrors = append(fileErrors, fc.Path)
		}
	}
	if len(fileErrors) > 0 {
		slog.Error("Failed to read referenced files",
			"command", commandName,
			"file_errors", fileErrors,
		)
		errorMsg := fmt.Sprintf("failed to read referenced file(s): %s", strings.Join(fileErrors, ", "))
		if len(fileErrors) == 1 {
			errorMsg = fmt.Sprintf("failed to read referenced file: %s", fileErrors[0])
		}
		return fmt.Errorf("%s", errorMsg)
	}

	var attachments []message.Attachment
	attachments = buildFileAttachments(fileContents)

	// Wrap command content with explicit execution instruction to ensure the agent
	// executes it directly rather than analyzing or searching. This is done after
	// processing arguments and file references so the wrapper doesn't interfere.
	processedContent = "Execute this directly - do not analyze or search:\n\n" + processedContent

	// 5. Filter tools based on allowed-tools frontmatter
	// Note: Tool restrictions are handled at the agent level through AllowedTools.
	// The coordinator uses the default agent config, so tool restrictions will
	// be applied when the agent is built. For now, we execute with the default
	// agent. Future enhancements may allow per-command agent configs.
	//
	// The buildRestrictedAgentConfig function is available for future use when
	// the coordinator supports dynamic agent configs per Run call.

	// 6. Execute through coordinator with processed content and attachments
	slog.Info("Executing command",
		"command", commandName,
		"session_id", sessionID,
		"args_count", len(args),
		"attachments_count", len(attachments),
	)

	_, err = e.coordinator.Run(ctx, sessionID, processedContent, attachments...)
	if err != nil {
		slog.Error("Command execution failed",
			"command", commandName,
			"session_id", sessionID,
			"error", err,
		)
		return fmt.Errorf("failed to execute command '%s': %w", commandName, err)
	}

	return nil
}

// executeHelpCommand executes the built-in \help command.
// It generates help output listing all available commands and creates an assistant message directly
// without going through the LLM agent.
func (e *executor) executeHelpCommand(ctx context.Context, sessionID string) error {
	slog.Info("Executing help command",
		"session_id", sessionID,
	)

	// Create help handler and generate help output
	helpHandler := NewHelpHandler(e.registry)
	helpOutput := helpHandler.GenerateHelp()

	// Create an assistant message directly with the help output
	// This bypasses the LLM and displays the help text immediately
	_, err := e.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:     message.Assistant,
		Parts:    []message.ContentPart{message.TextContent{Text: helpOutput}},
		Model:    "",
		Provider: "",
	})
	if err != nil {
		slog.Error("Help command execution failed",
			"session_id", sessionID,
			"error", err,
		)
		return fmt.Errorf("failed to create help message: %w", err)
	}

	return nil
}

// suggestSimilarCommands finds similar command names using fuzzy matching.
// Returns up to 3 most similar command names.
func (e *executor) suggestSimilarCommands(commandName string) []string {
	allCommands := e.registry.ListCommands()
	if len(allCommands) == 0 {
		return nil
	}

	// Build a slice of command names for fuzzy matching
	commandNames := make([]string, len(allCommands))
	for i, cmd := range allCommands {
		commandNames[i] = cmd.Name
	}

	// Use fuzzy matching to find similar commands
	matches := fuzzy.Find(commandName, commandNames)

	// Limit to top 3 suggestions
	maxSuggestions := 3
	if len(matches) > maxSuggestions {
		matches = matches[:maxSuggestions]
	}

	// Extract command names from matches
	suggestions := make([]string, 0, len(matches))
	for _, match := range matches {
		suggestions = append(suggestions, commandNames[match.Index])
	}

	return suggestions
}

// validateArguments checks if the provided arguments match the command's requirements.
// Returns an error if arguments are invalid or missing.
func validateArguments(args []string, required RequiredArguments, commandName string) error {
	// If command uses $ARGS or $ARGUMENTS, any number of arguments is valid (including 0)
	if required.HasAllArguments {
		return nil
	}

	// If no arguments required, check if any were provided
	if required.RequiredCount == 0 {
		if len(args) > 0 {
			slog.Warn("Command does not accept arguments",
				"command", commandName,
				"provided_args", len(args),
			)
			return fmt.Errorf("command '%s' does not accept arguments, but %d argument(s) were provided", commandName, len(args))
		}
		return nil
	}

	// Check if enough arguments were provided
	if len(args) < required.RequiredCount {
		slog.Warn("Missing required arguments",
			"command", commandName,
			"required", required.RequiredCount,
			"provided", len(args),
		)

		errorMsg := fmt.Sprintf("command '%s' requires %d argument(s), but only %d provided", commandName, required.RequiredCount, len(args))

		// Provide more specific error message about which arguments are missing
		if required.MaxPositional > 0 {
			var missingArgs []string
			for i := len(args) + 1; i <= required.MaxPositional; i++ {
				missingArgs = append(missingArgs, fmt.Sprintf("$%d", i))
			}
			if len(missingArgs) > 0 {
				errorMsg += fmt.Sprintf(". Missing: %s", strings.Join(missingArgs, ", "))
			}
		}

		return fmt.Errorf("%s", errorMsg)
	}

	return nil
}
