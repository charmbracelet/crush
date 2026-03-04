// Package skills implements the Agent Skills open standard.
// See https://agentskills.io for the specification.
package skills

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/charlievieth/fastwalk"
	"gopkg.in/yaml.v3"
)

const (
	SkillFileName          = "SKILL.md"
	MaxNameLength          = 64
	MaxDescriptionLength   = 1024
	MaxCompatibilityLength = 500
)

var (
	namePattern    = regexp.MustCompile(`^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$`)
	promptReplacer = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
)

// Skill represents a parsed SKILL.md file.
type Skill struct {
	Name          string            `yaml:"name" json:"name"`
	Description   string            `yaml:"description" json:"description"`
	License       string            `yaml:"license,omitempty" json:"license,omitempty"`
	Compatibility string            `yaml:"compatibility,omitempty" json:"compatibility,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Instructions  string            `yaml:"-" json:"instructions"`
	Path          string            `yaml:"-" json:"path"`
	SkillFilePath string            `yaml:"-" json:"skill_file_path"`
}

// Validate checks if the skill meets spec requirements.
func (s *Skill) Validate() error {
	var errs []error

	if s.Name == "" {
		errs = append(errs, errors.New("name is required"))
	} else {
		if len(s.Name) > MaxNameLength {
			errs = append(errs, fmt.Errorf("name exceeds %d characters", MaxNameLength))
		}
		if !namePattern.MatchString(s.Name) {
			errs = append(errs, errors.New("name must be alphanumeric with hyphens, no leading/trailing/consecutive hyphens"))
		}
		if s.Path != "" && !strings.EqualFold(filepath.Base(s.Path), s.Name) {
			errs = append(errs, fmt.Errorf("name %q must match directory %q", s.Name, filepath.Base(s.Path)))
		}
	}

	if s.Description == "" {
		errs = append(errs, errors.New("description is required"))
	} else if len(s.Description) > MaxDescriptionLength {
		errs = append(errs, fmt.Errorf("description exceeds %d characters", MaxDescriptionLength))
	}

	if len(s.Compatibility) > MaxCompatibilityLength {
		errs = append(errs, fmt.Errorf("compatibility exceeds %d characters", MaxCompatibilityLength))
	}

	return errors.Join(errs...)
}

// Parse parses a SKILL.md file.
func Parse(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	frontmatter, body, err := splitFrontmatter(string(content))
	if err != nil {
		return nil, err
	}

	var skill Skill
	if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	skill.Instructions = strings.TrimSpace(body)
	skill.Path = filepath.Dir(path)
	skill.SkillFilePath = path

	return &skill, nil
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

// Discover finds all valid skills in the given paths.
func Discover(paths []string) []*Skill {
	var skills []*Skill
	var mu sync.Mutex
	seen := make(map[string]bool)

	for _, base := range paths {
		// We use fastwalk with Follow: true instead of filepath.WalkDir because
		// WalkDir doesn't follow symlinked directories at any depth—only entry
		// points. This ensures skills in symlinked subdirectories are discovered.
		// fastwalk is concurrent, so we protect shared state (seen, skills) with mu.
		conf := fastwalk.Config{
			Follow:  true,
			ToSlash: fastwalk.DefaultToSlash(),
		}
		err := fastwalk.Walk(&conf, base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				slog.Warn("Failed to walk skills path entry", "base", base, "path", path, "error", err)
				return nil
			}
			if d.IsDir() || d.Name() != SkillFileName {
				return nil
			}
			mu.Lock()
			if seen[path] {
				mu.Unlock()
				return nil
			}
			seen[path] = true
			mu.Unlock()
			skill, err := Parse(path)
			if err != nil {
				slog.Warn("Failed to parse skill file", "path", path, "error", err)
				return nil
			}
			if err := skill.Validate(); err != nil {
				slog.Warn("Skill validation failed", "path", path, "error", err)
				return nil
			}
			slog.Debug("Successfully loaded skill", "name", skill.Name, "path", path)
			mu.Lock()
			skills = append(skills, skill)
			mu.Unlock()
			return nil
		})
		if err != nil {
			slog.Warn("Failed to walk skills path", "path", base, "error", err)
		}
	}

	return skills
}

// ToPromptXML generates XML for injection into the system prompt.
func ToPromptXML(skills []*Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<available_skills>\n")
	for _, s := range skills {
		sb.WriteString("  <skill>\n")
		sb.WriteString("    <name>" + escape(s.Name) + "</name>\n")
		sb.WriteString("    <description>" + escape(s.Description) + "</description>\n")
		sb.WriteString("    <location>" + escape(s.SkillFilePath) + "</location>\n")
		sb.WriteString("  </skill>\n")
	}
	sb.WriteString("</available_skills>")
	return sb.String()
}

func escape(s string) string {
	return promptReplacer.Replace(s)
}
