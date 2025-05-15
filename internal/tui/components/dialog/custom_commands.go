package dialog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// Command prefix constants
const (
	UserCommandPrefix    = "user:"
	ProjectCommandPrefix = "project:"
)

// namedArgPattern is a regex pattern to find named arguments in the format $NAME
var namedArgPattern = regexp.MustCompile(`\$([A-Z][A-Z0-9_]*)`)

// LoadCustomCommands loads custom commands from both XDG_CONFIG_HOME and project data directory
func LoadCustomCommands() ([]Command, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, fmt.Errorf("config not loaded")
	}

	var commands []Command

	// Load user commands from XDG_CONFIG_HOME/opencode/commands
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		// Default to ~/.config if XDG_CONFIG_HOME is not set
		home, err := os.UserHomeDir()
		if err == nil {
			xdgConfigHome = filepath.Join(home, ".config")
		}
	}

	if xdgConfigHome != "" {
		userCommandsDir := filepath.Join(xdgConfigHome, "opencode", "commands")
		userCommands, err := loadCommandsFromDir(userCommandsDir, UserCommandPrefix)
		if err != nil {
			// Log error but continue - we'll still try to load other commands
			fmt.Printf("Warning: failed to load user commands from XDG_CONFIG_HOME: %v\n", err)
		} else {
			commands = append(commands, userCommands...)
		}
	}

	// Load commands from $HOME/.opencode/commands
	home, err := os.UserHomeDir()
	if err == nil {
		homeCommandsDir := filepath.Join(home, ".opencode", "commands")
		homeCommands, err := loadCommandsFromDir(homeCommandsDir, UserCommandPrefix)
		if err != nil {
			// Log error but continue - we'll still try to load other commands
			fmt.Printf("Warning: failed to load home commands: %v\n", err)
		} else {
			commands = append(commands, homeCommands...)
		}
	}

	// Load project commands from data directory
	projectCommandsDir := filepath.Join(cfg.Data.Directory, "commands")
	projectCommands, err := loadCommandsFromDir(projectCommandsDir, ProjectCommandPrefix)
	if err != nil {
		// Log error but return what we have so far
		fmt.Printf("Warning: failed to load project commands: %v\n", err)
	} else {
		commands = append(commands, projectCommands...)
	}

	return commands, nil
}

// loadCommandsFromDir loads commands from a specific directory with the given prefix
func loadCommandsFromDir(commandsDir string, prefix string) ([]Command, error) {
	// Check if the commands directory exists
	if _, err := os.Stat(commandsDir); os.IsNotExist(err) {
		// Create the commands directory if it doesn't exist
		if err := os.MkdirAll(commandsDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create commands directory %s: %w", commandsDir, err)
		}
		// Return empty list since we just created the directory
		return []Command{}, nil
	}

	var commands []Command

	// Walk through the commands directory and load all .md files
	err := filepath.Walk(commandsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process markdown files
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		// Read the file content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read command file %s: %w", path, err)
		}

		// Get the command ID from the file name without the .md extension
		commandID := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))

		// Get relative path from commands directory
		relPath, err := filepath.Rel(commandsDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}

		// Create the command ID from the relative path
		// Replace directory separators with colons
		commandIDPath := strings.ReplaceAll(filepath.Dir(relPath), string(filepath.Separator), ":")
		if commandIDPath != "." {
			commandID = commandIDPath + ":" + commandID
		}

		// Create a command
		command := Command{
			ID:          prefix + commandID,
			Title:       prefix + commandID,
			Description: fmt.Sprintf("Custom command from %s", relPath),
			Handler: func(cmd Command) tea.Cmd {
				commandContent := string(content)

				// Check for named arguments
				matches := namedArgPattern.FindAllStringSubmatch(commandContent, -1)
				if len(matches) > 0 {
					// Extract unique argument names
					argNames := make([]string, 0)
					argMap := make(map[string]bool)

					for _, match := range matches {
						argName := match[1] // Group 1 is the name without $
						if !argMap[argName] {
							argMap[argName] = true
							argNames = append(argNames, argName)
						}
					}

					// Show multi-arguments dialog for all named arguments
					return util.CmdHandler(ShowMultiArgumentsDialogMsg{
						CommandID: cmd.ID,
						Content:   commandContent,
						ArgNames:  argNames,
					})
				}

				// No arguments needed, run command directly
				return util.CmdHandler(CommandRunCustomMsg{
					Content: commandContent,
					Args:    nil, // No arguments
				})
			},
		}

		commands = append(commands, command)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load custom commands from %s: %w", commandsDir, err)
	}

	return commands, nil
}

// CommandRunCustomMsg is sent when a custom command is executed
type CommandRunCustomMsg struct {
	Content string
	Args    map[string]string // Map of argument names to values
}
