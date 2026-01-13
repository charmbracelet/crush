package subagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Loader handles loading subagents from various sources.
type Loader struct {
	userDir    string
	projectDir string
	cliAgents  string // JSON string from --agents flag
}

// NewLoader creates a new subagent loader.
func NewLoader(userDir, projectDir, cliAgents string) *Loader {
	return &Loader{
		userDir:    userDir,
		projectDir: projectDir,
		cliAgents:  cliAgents,
	}
}

// Load loads all subagents from all sources, with priority:
// CLI > Project > User.
func (l *Loader) Load() (map[string]*Subagent, error) {
	subagents := make(map[string]*Subagent)

	// 1. User subagents (lowest priority)
	if l.userDir != "" {
		userSubagents, err := l.loadFromDir(l.userDir, SubagentSourceUser)
		if err == nil {
			for name, sub := range userSubagents {
				subagents[name] = sub
			}
		}
	}

	// 2. Project subagents
	if l.projectDir != "" {
		projectSubagents, err := l.loadFromDir(l.projectDir, SubagentSourceProject)
		if err == nil {
			for name, sub := range projectSubagents {
				subagents[name] = sub
			}
		}
	}

	// 3. CLI subagents (highest priority)
	if l.cliAgents != "" {
		cliSubagents, err := l.loadFromCLI(l.cliAgents)
		if err == nil {
			for name, sub := range cliSubagents {
				subagents[name] = sub
			}
		}
	}

	return subagents, nil
}

func (l *Loader) loadFromDir(dir string, source SubagentSource) (map[string]*Subagent, error) {
	subagents := make(map[string]*Subagent)
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return subagents, nil
		}
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".md" {
			continue
		}

		path := filepath.Join(dir, file.Name())
		sub, err := ParseSubagentFile(path)
		if err != nil {
			// Skip invalid subagents
			continue
		}
		sub.Source = source
		sub.FilePath = path
		subagents[sub.Name] = sub
	}

	return subagents, nil
}

func (l *Loader) loadFromCLI(cliJSON string) (map[string]*Subagent, error) {
	var raw map[string]struct {
		Description     string         `json:"description"`
		Prompt          string         `json:"prompt"`
		Tools           []string       `json:"tools,omitempty"`
		DisallowedTools []string       `json:"disallowedTools,omitempty"`
		Model           string         `json:"model,omitempty"`
		PermissionMode  string         `json:"permissionMode,omitempty"`
		Color           string         `json:"color,omitempty"`
		Skills          []string       `json:"skills,omitempty"`
		Hooks           *SubagentHooks `json:"hooks,omitempty"`
	}

	if err := json.Unmarshal([]byte(cliJSON), &raw); err != nil {
		return nil, err
	}

	subagents := make(map[string]*Subagent)
	for name, data := range raw {
		subagents[name] = &Subagent{
			Name:            name,
			Description:     data.Description,
			SystemPrompt:    data.Prompt,
			Tools:           data.Tools,
			DisallowedTools: data.DisallowedTools,
			Model:           data.Model,
			PermissionMode:  data.PermissionMode,
			Color:           data.Color,
			Skills:          data.Skills,
			Hooks:           data.Hooks,
			Source:          SubagentSourceCLI,
		}
	}

	return subagents, nil
}

// ParseSubagentFile parses a subagent file (Markdown with YAML frontmatter).
func ParseSubagentFile(path string) (*Subagent, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	s := string(content)
	if !strings.HasPrefix(s, "---\n") {
		return nil, fmt.Errorf("missing YAML frontmatter")
	}

	parts := strings.SplitN(s, "---\n", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid subagent format")
	}

	var sub Subagent
	if err := yaml.Unmarshal([]byte(parts[1]), &sub); err != nil {
		return nil, err
	}

	sub.SystemPrompt = strings.TrimSpace(parts[2])
	if sub.Name == "" {
		// Default name to filename without extension
		sub.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	return &sub, nil
}
