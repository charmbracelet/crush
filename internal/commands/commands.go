package commands

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/skills"
)

var namedArgPattern = regexp.MustCompile(`\$([A-Z][A-Z0-9_]*)`)

const (
	userCommandPrefix    = "user:"
	projectCommandPrefix = "project:"
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
	// Skill is set when this command represents a user-invocable skill
	Skill *skills.Skill
}

type commandSource struct {
	path   string
	prefix string
	// claude marks Claude Code command dirs: frontmatter is stripped, $ARGUMENTS
	// is optional, and commands never shadow earlier sources.
	claude bool
}

// warnedShadows remembers which shadowed Claude commands were already logged.
var warnedShadows sync.Map

// LoadCustomCommands loads custom commands from multiple sources including
// XDG config directory, home directory, and project directory. workingDir is
// the project root used for <project>/.claude/commands.
func LoadCustomCommands(cfg *config.Config, workingDir string) ([]CustomCommand, error) {
	return loadAll(buildCommandSources(cfg, workingDir))
}

// FromSkillCatalog converts user-invocable catalog entries into custom
// command entries for the command palette.
func FromSkillCatalog(entries []skills.CatalogEntry) []CustomCommand {
	commands := make([]CustomCommand, 0, len(entries))
	for _, entry := range entries {
		if !entry.UserInvocable {
			continue
		}
		name := entry.Label
		if name == "" {
			name = userCommandPrefix + entry.Name
		}
		commands = append(commands, CustomCommand{
			ID:   name,
			Name: name,
			Skill: &skills.Skill{
				Name:          entry.Name,
				Description:   entry.Description,
				SkillFilePath: entry.ID,
			},
		})
	}
	return commands
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

// claudeSources lists the Claude Code command dirs. Subdirs become prefixes,
// same as crush's own dirs.
func claudeSources(userHome, projectDir string) []commandSource {
	return []commandSource{
		{path: filepath.Join(userHome, ".claude", "commands"), prefix: userCommandPrefix, claude: true},
		{path: filepath.Join(projectDir, ".claude", "commands"), prefix: projectCommandPrefix, claude: true},
	}
}

func buildCommandSources(cfg *config.Config, workingDir string) []commandSource {
	sources := []commandSource{
		{
			path:   filepath.Join(home.Config(), "crush", "commands"),
			prefix: userCommandPrefix,
		},
		{
			path:   filepath.Join(home.Dir(), ".crush", "commands"),
			prefix: userCommandPrefix,
		},
		{
			path:   filepath.Join(cfg.Options.DataDirectory, "commands"),
			prefix: projectCommandPrefix,
		},
	}
	return append(sources, claudeSources(home.Dir(), workingDir)...)
}

func loadAll(sources []commandSource) ([]CustomCommand, error) {
	var commands []CustomCommand
	seen := make(map[string]bool)

	for _, source := range sources {
		cmds, err := loadFromSource(source)
		if err != nil {
			continue
		}
		for _, cmd := range cmds {
			if source.claude && seen[cmd.ID] {
				if _, warned := warnedShadows.LoadOrStore(source.path+"|"+cmd.ID, true); !warned {
					slog.Warn("Claude command ignored, a command with the same name exists", "id", cmd.ID, "dir", source.path)
				}
				continue
			}
			seen[cmd.ID] = true
			commands = append(commands, cmd)
		}
	}

	return commands, nil
}

func loadFromSource(source commandSource) ([]CustomCommand, error) {
	if _, err := os.Stat(source.path); os.IsNotExist(err) {
		return nil, nil
	}

	var commands []CustomCommand

	err := filepath.WalkDir(source.path, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isMarkdownFile(d.Name()) {
			return err
		}

		cmd, err := loadCommand(path, source.path, source.prefix, source.claude)
		if err != nil {
			return nil // Skip invalid files
		}

		commands = append(commands, cmd)
		return nil
	})

	return commands, err
}

func loadCommand(path, baseDir, prefix string, claude bool) (CustomCommand, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return CustomCommand{}, err
	}
	content := string(raw)
	if claude {
		content = stripFrontmatter(content)
	}

	id := buildCommandID(path, baseDir, prefix)
	args := extractArgNames(content)
	if claude {
		// Claude Code runs a command with an empty $ARGUMENTS.
		for i := range args {
			if args[i].ID == "ARGUMENTS" {
				args[i].Required = false
			}
		}
	}

	return CustomCommand{ID: id, Name: id, Content: content, Arguments: args}, nil
}

// stripFrontmatter drops a leading YAML block. The opening line must be exactly
// "---"; the closing line is "---" plus optional spaces/tabs, followed by a
// newline or EOF. A body that itself starts with a markdown rule "---" is
// therefore read as frontmatter when a later "---" line exists; with no later
// one the content is returned whole. Its keys (description, argument-hint,
// allowed-tools, model) are ignored: the command model has no field for them.
func stripFrontmatter(content string) string {
	content = strings.TrimPrefix(content, "\uFEFF")
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if lines[0] != "---" {
		return content
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \t") == "---" {
			return strings.TrimPrefix(strings.Join(lines[i+1:], "\n"), "\n")
		}
	}
	return content
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

func isMarkdownFile(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".md")
}

func GetMCPPrompt(cfg *config.ConfigStore, clientID, promptID string, args map[string]string) (string, error) {
	// Create a context with timeout since tea.Cmd doesn't support context passing.
	// The MCP client has its own timeout, but this provides an additional safeguard.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := mcp.GetPromptMessages(ctx, cfg, clientID, promptID, args)
	if err != nil {
		return "", err
	}
	return strings.Join(result, " "), nil
}
