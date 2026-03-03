package commands

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
)

var namedArgPattern = regexp.MustCompile(`\$([A-Z][A-Z0-9_]*)`)

const (
	userCommandPrefix    = "user:"
	projectCommandPrefix = "project:"
	claudeCommandPrefix  = "claude:"
)

// Argument represents a command argument with its metadata.
type Argument struct {
	ID          string
	Title       string
	Description string
	Required    bool
}

// MCPPrompt represents a custom command loaded from an MCP server.
type MCPPrompt struct {
	ID          string
	Title       string
	Description string
	PromptID    string
	ClientID    string
	Arguments   []Argument
}

// CustomCommand represents a user-defined custom command loaded from markdown files.
type CustomCommand struct {
	ID        string
	Name      string
	Content   string
	Arguments []Argument
}

type commandSource struct {
	path        string
	prefix      string
	createDir   bool // Create directory if it doesn't exist.
	frontmatter bool // Strip YAML frontmatter from command content.
}

// LoadCustomCommands loads custom commands from multiple sources including
// XDG config directory, home directory, and project directory.
func LoadCustomCommands(cfg *config.Config) ([]CustomCommand, error) {
	return loadAll(buildCommandSources(cfg))
}

// LoadMCPPrompts loads custom commands from available MCP servers.
func LoadMCPPrompts() ([]MCPPrompt, error) {
	var commands []MCPPrompt
	for mcpName, prompts := range mcp.Prompts() {
		for _, prompt := range prompts {
			key := mcpName + ":" + prompt.Name
			var args []Argument
			for _, arg := range prompt.Arguments {
				title := arg.Title
				if title == "" {
					title = arg.Name
				}
				args = append(args, Argument{
					ID:          arg.Name,
					Title:       title,
					Description: arg.Description,
					Required:    arg.Required,
				})
			}
			commands = append(commands, MCPPrompt{
				ID:          key,
				Title:       prompt.Title,
				Description: prompt.Description,
				PromptID:    prompt.Name,
				ClientID:    mcpName,
				Arguments:   args,
			})
		}
	}
	return commands, nil
}

func buildCommandSources(cfg *config.Config) []commandSource {
	var sources []commandSource

	homeDir := home.Dir()
	projectDir := filepath.Dir(cfg.Options.DataDirectory)

	// XDG config directory
	if dir := getXDGCommandsDir(); dir != "" {
		sources = append(sources, commandSource{
			path:      dir,
			prefix:    userCommandPrefix,
			createDir: true,
		})
	}

	// Home directory
	if homeDir != "" {
		sources = append(sources, commandSource{
			path:      filepath.Join(homeDir, ".crush", "commands"),
			prefix:    userCommandPrefix,
			createDir: true,
		})
	}

	// Project directory
	sources = append(sources, commandSource{
		path:      filepath.Join(cfg.Options.DataDirectory, "commands"),
		prefix:    projectCommandPrefix,
		createDir: true,
	})

	// Claude Code global commands (~/.claude/commands)
	if homeDir != "" {
		sources = append(sources, commandSource{
			path:        filepath.Join(homeDir, ".claude", "commands"),
			prefix:      claudeCommandPrefix,
			frontmatter: true,
		})
	}

	// Claude Code project commands (<project>/.claude/commands)
	sources = append(sources, commandSource{
		path:        filepath.Join(projectDir, ".claude", "commands"),
		prefix:      claudeCommandPrefix,
		frontmatter: true,
	})

	return sources
}

func loadAll(sources []commandSource) ([]CustomCommand, error) {
	var commands []CustomCommand

	for _, source := range sources {
		if cmds, err := loadFromSource(source); err == nil {
			commands = append(commands, cmds...)
		}
	}

	return commands, nil
}

func loadFromSource(source commandSource) ([]CustomCommand, error) {
	if source.createDir {
		if err := ensureDir(source.path); err != nil {
			return nil, err
		}
	} else if _, err := os.Stat(source.path); err != nil {
		return nil, nil
	}

	var commands []CustomCommand

	err := filepath.WalkDir(source.path, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isMarkdownFile(d.Name()) {
			return err
		}

		cmd, err := loadCommand(path, source)
		if err != nil {
			return nil // Skip invalid files
		}

		commands = append(commands, cmd)
		return nil
	})

	return commands, err
}

func loadCommand(path string, source commandSource) (CustomCommand, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return CustomCommand{}, err
	}

	content := string(raw)
	if source.frontmatter {
		content = stripFrontmatter(content)
	}

	id := buildCommandID(path, source.path, source.prefix)

	return CustomCommand{
		ID:        id,
		Name:      id,
		Content:   content,
		Arguments: extractArgNames(content),
	}, nil
}

func extractArgNames(content string) []Argument {
	matches := namedArgPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var args []Argument

	for _, match := range matches {
		arg := match[1]
		if !seen[arg] {
			seen[arg] = true
			// for normal custom commands, all args are required
			args = append(args, Argument{ID: arg, Title: arg, Required: true})
		}
	}

	return args
}

func buildCommandID(path, baseDir, prefix string) string {
	relPath, _ := filepath.Rel(baseDir, path)
	parts := strings.Split(relPath, string(filepath.Separator))

	// Remove .md extension from last part
	if len(parts) > 0 {
		lastIdx := len(parts) - 1
		parts[lastIdx] = strings.TrimSuffix(parts[lastIdx], filepath.Ext(parts[lastIdx]))
	}

	return prefix + strings.Join(parts, ":")
}

func getXDGCommandsDir() string {
	xdgHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgHome == "" {
		if home := home.Dir(); home != "" {
			xdgHome = filepath.Join(home, ".config")
		}
	}
	if xdgHome != "" {
		return filepath.Join(xdgHome, "crush", "commands")
	}
	return ""
}

func stripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return content
	}
	return strings.TrimSpace(rest[end+4:])
}

func ensureDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0o755)
	}
	return nil
}

func isMarkdownFile(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".md")
}

func GetMCPPrompt(cfg *config.Config, clientID, promptID string, args map[string]string) (string, error) {
	// TODO: we should pass the context down
	result, err := mcp.GetPromptMessages(context.Background(), cfg, clientID, promptID, args)
	if err != nil {
		return "", err
	}
	return strings.Join(result, " "), nil
}
