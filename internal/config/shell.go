package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/llm/tools/shell"
	"github.com/charmbracelet/crush/internal/logging"
)

// ExecuteCommand executes a shell command and returns the output
// This is a shared utility that can be used by both provider config and tools
func ExecuteCommand(ctx context.Context, command string, workingDir string) (string, error) {
	if workingDir == "" {
		workingDir = WorkingDirectory()
	}

	persistentShell := shell.GetPersistentShell(workingDir)

	stdout, stderr, err := persistentShell.Exec(ctx, command)
	if err != nil {
		logging.Debug("Command execution failed", "command", command, "error", err, "stderr", stderr)
		return "", fmt.Errorf("command execution failed: %w", err)
	}

	return strings.TrimSpace(stdout), nil
}

// ResolveAPIKey resolves an API key that can be either:
// - A direct string value
// - An environment variable (prefixed with $)
// - A shell command (wrapped in $(...))
func ResolveAPIKey(apiKey string) (string, error) {
	if !strings.HasPrefix(apiKey, "$") {
		return apiKey, nil
	}

	if strings.HasPrefix(apiKey, "$(") && strings.HasSuffix(apiKey, ")") {
		command := strings.TrimSuffix(strings.TrimPrefix(apiKey, "$("), ")")
		logging.Debug("Resolving API key from command", "command", command)
		return resolveCommandAPIKey(command)
	}

	envVar := strings.TrimPrefix(apiKey, "$")
	if value := os.Getenv(envVar); value != "" {
		logging.Debug("Resolved environment variable", "envVar", envVar, "value", value)
		return value, nil
	}

	logging.Debug("Environment variable not found", "envVar", envVar)

	return "", fmt.Errorf("environment variable %s not found", envVar)
}

// resolveCommandAPIKey executes a command to get an API key, with caching support
func resolveCommandAPIKey(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logging.Debug("Executing command for API key", "command", command)

	workingDir := WorkingDirectory()

	result, err := ExecuteCommand(ctx, command, workingDir)
	if err != nil {
		return "", fmt.Errorf("failed to execute API key command: %w", err)
	}
	logging.Debug("Command executed successfully", "command", command, "result", result)
	return result, nil
}

