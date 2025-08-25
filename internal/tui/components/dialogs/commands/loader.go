package commands

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	UserCommandPrefix    = "user:"
	ProjectCommandPrefix = "project:"
)

var namedArgPattern = regexp.MustCompile(`\$([A-Z][A-Z0-9_]*)`)

type commandLoader struct {
	sources []commandSource
}

type commandSource struct {
	path   string
	prefix string
}

func LoadCustomCommands() ([]Command, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, fmt.Errorf("config not loaded")
	}

	loader := &commandLoader{
		sources: buildCommandSources(cfg),
	}

	return loader.loadAll()
}

func buildCommandSources(cfg *config.Config) []commandSource {
	var sources []commandSource

	// XDG config directory
	if dir := getXDGCommandsDir(); dir != "" {
		sources = append(sources, commandSource{
			path:   dir,
			prefix: UserCommandPrefix,
		})
	}

	// Home directory
	if home := home.Dir(); home != "" {
		sources = append(sources, commandSource{
			path:   filepath.Join(home, ".crush", "commands"),
			prefix: UserCommandPrefix,
		})
	}

	// Project directory
	sources = append(sources, commandSource{
		path:   filepath.Join(cfg.Options.DataDirectory, "commands"),
		prefix: ProjectCommandPrefix,
	})

	return sources
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

func (l *commandLoader) loadAll() ([]Command, error) {
	var commands []Command

	for _, source := range l.sources {
		if cmds, err := l.loadFromSource(source); err == nil {
			commands = append(commands, cmds...)
		}
	}

	return commands, nil
}

func (l *commandLoader) loadFromSource(source commandSource) ([]Command, error) {
	if err := ensureDir(source.path); err != nil {
		return nil, err
	}

	var commands []Command

	err := filepath.WalkDir(source.path, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isMarkdownFile(d.Name()) {
			return err
		}

		cmd, err := l.loadCommand(path, source.path, source.prefix)
		if err != nil {
			return nil // Skip invalid files
		}

		commands = append(commands, cmd)
		return nil
	})

	return commands, err
}

func (l *commandLoader) loadCommand(path, baseDir, prefix string) (Command, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Command{}, err
	}

	id := buildCommandID(path, baseDir, prefix)

	return Command{
		ID:          id,
		Title:       id,
		Description: fmt.Sprintf("Custom command from %s", filepath.Base(path)),
		Handler:     createCommandHandler(id, string(content)),
	}, nil
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

func createCommandHandler(id string, content string) func(Command) tea.Cmd {
	return func(cmd Command) tea.Cmd {
		args := extractArgNames(content)

		if len(args) > 0 {
			return util.CmdHandler(ShowArgumentsDialogMsg{
				CommandID: id,
				Content:   content,
				ArgNames:  args,
			})
		}

		return util.CmdHandler(CommandRunCustomMsg{
			Content: content,
		})
	}
}

func extractArgNames(content string) []string {
	matches := namedArgPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var args []string

	for _, match := range matches {
		arg := match[1]
		if !seen[arg] {
			seen[arg] = true
			args = append(args, arg)
		}
	}

	return args
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

type CommandRunCustomMsg struct {
	Content string
}
