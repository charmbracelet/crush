package commands

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/crush/internal/home"
)

// loadProjectCommands loads all commands from the project directory (.crush/commands/**/*.md).
// It recursively walks subdirectories and parses each markdown file into a Command struct.
// Returns a slice of all commands found, with errors logged but not returned (partial success).
func loadProjectCommands(projectDir string) ([]Command, error) {
	commandsDir := filepath.Join(projectDir, ".crush", "commands")

	// Check if commands directory exists
	if _, err := os.Stat(commandsDir); os.IsNotExist(err) {
		// Directory doesn't exist - this is fine, just return empty slice
		return []Command{}, nil
	}

	var commands []Command
	var errors []error

	// Walk directory recursively
	err := filepath.WalkDir(commandsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Log but continue - don't stop on individual file errors
			slog.Warn("Error accessing path during command walk",
				"path", path,
				"error", err,
			)
			return nil
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only process .md files
		if !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}

		// Load and parse the command file
		cmd, err := loadCommandFile(path, commandsDir, SourceProject)
		if err != nil {
			// Log error but continue loading other commands
			slog.Warn("Failed to load command file",
				"path", path,
				"error", err,
			)
			errors = append(errors, err)
			return nil
		}

		commands = append(commands, cmd)
		return nil
	})

	if err != nil {
		return commands, err
	}

	// If we have some commands but also some errors, log a summary
	if len(errors) > 0 && len(commands) > 0 {
		slog.Warn("Some commands failed to load",
			"loaded", len(commands),
			"errors", len(errors),
		)
	}

	return commands, nil
}

// loadCommandFile loads a single command file and parses it into a Command struct.
func loadCommandFile(filePath, baseDir string, source CommandSource) (Command, error) {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return Command{}, err
	}

	// Parse frontmatter
	fm, cmdContent, err := ParseFrontmatter(string(content))
	if err != nil {
		return Command{}, err
	}

	// Derive command name and namespace from path
	name, namespace := deriveCommandName(filePath, baseDir)

	// Build source indicator
	sourceIndicator := buildSourceIndicator(source, namespace)

	// Validate and filter allowed tools
	validatedTools := validateAllowedTools(fm.AllowedTools, filePath)

	// Create Command struct
	cmd := Command{
		Name:         name,
		Namespace:    namespace,
		Description:  fm.Description,
		ArgumentHint: fm.ArgumentHint,
		AllowedTools: validatedTools,
		Content:      cmdContent,
		Path:         filePath,
		Source:       sourceIndicator,
	}

	return cmd, nil
}

// loadUserHomeCommands loads all commands from the user home directory (~/.crush/commands/**/*.md).
// It recursively walks subdirectories and parses each markdown file into a Command struct.
// Returns a slice of all commands found, with errors logged but not returned (partial success).
func loadUserHomeCommands() ([]Command, error) {
	commandsDir := filepath.Join(home.Dir(), ".crush", "commands")

	// Check if commands directory exists
	if _, err := os.Stat(commandsDir); os.IsNotExist(err) {
		// Directory doesn't exist - this is fine, just return empty slice
		return []Command{}, nil
	}

	var commands []Command
	var errors []error

	// Walk directory recursively
	err := filepath.WalkDir(commandsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Log but continue - don't stop on individual file errors
			slog.Warn("Error accessing path during command walk",
				"path", path,
				"error", err,
			)
			return nil
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only process .md files
		if !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}

		// Load and parse the command file
		cmd, err := loadCommandFile(path, commandsDir, SourceUserHome)
		if err != nil {
			// Log error but continue loading other commands
			slog.Warn("Failed to load command file",
				"path", path,
				"error", err,
			)
			errors = append(errors, err)
			return nil
		}

		commands = append(commands, cmd)
		return nil
	})

	if err != nil {
		return commands, err
	}

	// If we have some commands but also some errors, log a summary
	if len(errors) > 0 && len(commands) > 0 {
		slog.Warn("Some commands failed to load",
			"loaded", len(commands),
			"errors", len(errors),
		)
	}

	return commands, nil
}

// loadXDGCommands loads all commands from the XDG config directory.
// Checks $XDG_CONFIG_HOME first, then falls back to ~/.config/crush/commands.
// Returns a slice of all commands found, with errors logged but not returned (partial success).
func loadXDGCommands() ([]Command, error) {
	// Check XDG_CONFIG_HOME environment variable first
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		// Fall back to ~/.config if XDG_CONFIG_HOME not set
		xdgConfigHome = filepath.Join(home.Dir(), ".config")
	}

	commandsDir := filepath.Join(xdgConfigHome, "crush", "commands")

	// Check if commands directory exists
	if _, err := os.Stat(commandsDir); os.IsNotExist(err) {
		// Directory doesn't exist - this is fine, just return empty slice
		return []Command{}, nil
	}

	var commands []Command
	var errors []error

	// Walk directory recursively
	err := filepath.WalkDir(commandsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Log but continue - don't stop on individual file errors
			slog.Warn("Error accessing path during command walk",
				"path", path,
				"error", err,
			)
			return nil
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only process .md files
		if !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}

		// Load and parse the command file
		cmd, err := loadCommandFile(path, commandsDir, SourceXDG)
		if err != nil {
			// Log error but continue loading other commands
			slog.Warn("Failed to load command file",
				"path", path,
				"error", err,
			)
			errors = append(errors, err)
			return nil
		}

		commands = append(commands, cmd)
		return nil
	})

	if err != nil {
		return commands, err
	}

	// If we have some commands but also some errors, log a summary
	if len(errors) > 0 && len(commands) > 0 {
		slog.Warn("Some commands failed to load",
			"loaded", len(commands),
			"errors", len(errors),
		)
	}

	return commands, nil
}
