// Package subagents implements parsing and validation of subagent definition files.
package subagents

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// MaxNameLength is the maximum number of characters allowed in a subagent name.
	MaxNameLength = 64
	// MaxDescriptionLength is the maximum number of characters allowed in a subagent description.
	MaxDescriptionLength = 1024
)

// namePattern matches valid subagent names: lowercase alphanumeric with single hyphens,
// no leading or trailing hyphens, no consecutive hyphens.
var namePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// reservedNames is the set of names that may not be used for subagents.
var reservedNames = map[string]bool{
	"agent": true,
	"task":  true,
	"coder": true,
	"bash":  true,
	"view":  true,
	"edit":  true,
	"grep":  true,
	"glob":  true,
	"write": true,
	"ls":    true,
	"mcp":   true,
}

// ToolList is a []string that YAML-unmarshals from either a comma-separated
// scalar string ("Read, Grep, Bash") or a YAML sequence (["Read","Grep"]).
// When the field is absent the value stays nil.
type ToolList []string

// UnmarshalYAML implements yaml.Unmarshaler for ToolList.
func (t *ToolList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		if value.Value == "" || value.Tag == "!!null" {
			return nil
		}
		parts := strings.Split(value.Value, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			*t = result
		}
		return nil
	case yaml.SequenceNode:
		var items []string
		if err := value.Decode(&items); err != nil {
			return err
		}
		if len(items) > 0 {
			*t = items
		}
		return nil
	default:
		return nil
	}
}

// Subagent is a parsed subagent definition file.
type Subagent struct {
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	Tools           ToolList `yaml:"tools"`
	DisallowedTools ToolList `yaml:"disallowed_tools"`
	Model           string   `yaml:"model"`
	Skills          []string `yaml:"skills"`
	MCPServers      []string `yaml:"mcp_servers"`
	Body            string   // set from markdown body after frontmatter
	FilePath        string   // set from the file path passed to Parse
}

// ParseContent parses a subagent definition from raw bytes.
func ParseContent(content []byte) (*Subagent, error) {
	frontmatter, body, err := splitFrontmatter(string(content))
	if err != nil {
		return nil, err
	}

	var agent Subagent
	if err := yaml.Unmarshal([]byte(frontmatter), &agent); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	agent.Body = strings.TrimSpace(body)

	return &agent, nil
}

// Parse reads a subagent definition file from disk and sets FilePath.
func Parse(path string) (*Subagent, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	agent, err := ParseContent(content)
	if err != nil {
		return nil, err
	}

	agent.FilePath = path

	return agent, nil
}

// Validate checks that the subagent meets all specification requirements.
// Multiple errors are joined with errors.Join.
func (s *Subagent) Validate() error {
	var errs []error

	if s.Name == "" {
		errs = append(errs, errors.New("name is required"))
	} else {
		if len(s.Name) > MaxNameLength {
			errs = append(errs, fmt.Errorf("name exceeds %d characters", MaxNameLength))
		}
		if !namePattern.MatchString(s.Name) {
			errs = append(errs, errors.New("name must be lowercase alphanumeric with single hyphens (no leading, trailing, or consecutive hyphens)"))
		}
		if reservedNames[s.Name] {
			errs = append(errs, fmt.Errorf("name %q is reserved", s.Name))
		}
	}

	if s.Description == "" {
		errs = append(errs, errors.New("description is required"))
	} else if len(s.Description) > MaxDescriptionLength {
		errs = append(errs, fmt.Errorf("description exceeds %d characters", MaxDescriptionLength))
	}

	if len(s.Tools) > 0 && len(s.DisallowedTools) > 0 {
		disallowedSet := make(map[string]bool, len(s.DisallowedTools))
		for _, tool := range s.DisallowedTools {
			disallowedSet[tool] = true
		}
		for _, tool := range s.Tools {
			if disallowedSet[tool] {
				errs = append(errs, fmt.Errorf("tool %q appears in both tools and disallowed_tools", tool))
				break
			}
		}
	}

	return errors.Join(errs...)
}

// Filter removes subagents whose names appear in the disabled list.
func Filter(all []*Subagent, disabled []string) []*Subagent {
	if len(disabled) == 0 {
		return all
	}

	disabledSet := make(map[string]bool, len(disabled))
	for _, name := range disabled {
		disabledSet[name] = true
	}

	result := make([]*Subagent, 0, len(all))
	for _, s := range all {
		if !disabledSet[s.Name] {
			result = append(result, s)
		}
	}
	return result
}

// Deduplicate removes duplicate subagents by name. When duplicates exist, the
// last occurrence wins.
func Deduplicate(all []*Subagent) []*Subagent {
	if len(all) == 0 {
		return nil
	}

	seen := make(map[string]int, len(all))
	for i, s := range all {
		seen[s.Name] = i
	}

	result := make([]*Subagent, 0, len(seen))
	for i, s := range all {
		if seen[s.Name] == i {
			result = append(result, s)
		}
	}
	return result
}

// splitFrontmatter extracts YAML frontmatter and body from markdown content.
func splitFrontmatter(content string) (frontmatter, body string, err error) {
	// Strip UTF-8 BOM for compatibility with editors that include it.
	content = strings.TrimPrefix(content, "\uFEFF")
	// Normalize line endings to \n for consistent parsing.
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	lines := strings.Split(content, "\n")
	start := slices.IndexFunc(lines, func(line string) bool {
		return strings.TrimSpace(line) != ""
	})
	if start == -1 || strings.TrimSpace(lines[start]) != "---" {
		return "", "", errors.New("no YAML frontmatter found")
	}

	endOffset := slices.IndexFunc(lines[start+1:], func(line string) bool {
		return strings.TrimSpace(line) == "---"
	})
	if endOffset == -1 {
		return "", "", errors.New("unclosed frontmatter")
	}
	end := start + 1 + endOffset

	frontmatter = strings.Join(lines[start+1:end], "\n")
	body = strings.Join(lines[end+1:], "\n")
	return frontmatter, body, nil
}
