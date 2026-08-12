package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SourceType describes where a visible skill comes from.
type SourceType string

const (
	SourceSystem  SourceType = "system"
	SourceUser    SourceType = "user"
	SourceProject SourceType = "project"
)

// CatalogEntry describes an effective visible skill for frontend display.
type CatalogEntry struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	Label         string     `json:"label"`
	Source        SourceType `json:"source"`
	UserInvocable bool       `json:"user_invocable"`
}

// SkillReadResult holds metadata about a skill returned alongside its
// content.
type SkillReadResult struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Source      SourceType `json:"source"`
	Builtin     bool       `json:"builtin"`
}

// ErrSkillNotFound is returned when a skill ID is not part of the
// effective visible skill set.
var ErrSkillNotFound = errors.New("skill not found")

// Catalog builds a slice of CatalogEntry values from pre-discovered
// skills. The skillPaths and workingDir parameters are used only for
// labelling (system / user / project); pass nil/empty when labels are
// not needed.
func Catalog(active []*Skill, skillPaths []string, workingDir string) []CatalogEntry {
	entries := make([]CatalogEntry, 0, len(active))
	for _, skill := range active {
		label, source := skillLabel(skillPaths, workingDir, skill)
		entries = append(entries, CatalogEntry{
			ID:            skill.SkillFilePath,
			Name:          skill.Name,
			Description:   skill.Description,
			Label:         label,
			Source:        source,
			UserInvocable: skill.UserInvocable,
		})
	}
	return entries
}

// FindEffective returns the named skill from the given active skill
// set.
func FindEffective(active []*Skill, skillID string) (*Skill, error) {
	for _, skill := range active {
		if skill.SkillFilePath == skillID {
			return skill, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrSkillNotFound, skillID)
}

// ReadContent reads the contents of a visible skill by ID and returns
// the raw bytes along with metadata about the skill.
func ReadContent(active []*Skill, skillPaths []string, workingDir string, skillID string) ([]byte, SkillReadResult, error) {
	skill, err := FindEffective(active, skillID)
	if err != nil {
		return nil, SkillReadResult{}, err
	}

	_, source := skillLabel(skillPaths, workingDir, skill)
	result := SkillReadResult{
		Name:        skill.Name,
		Description: skill.Description,
		Source:      source,
		Builtin:     skill.Builtin,
	}

	if skill.Builtin {
		embeddedPath := "builtin/" + strings.TrimPrefix(skill.SkillFilePath, BuiltinPrefix)
		content, err := BuiltinFS().ReadFile(embeddedPath)
		if err != nil {
			return nil, SkillReadResult{}, fmt.Errorf("read builtin skill %q: %w", skillID, err)
		}
		return content, result, nil
	}

	content, err := os.ReadFile(skill.SkillFilePath)
	if err != nil {
		return nil, SkillReadResult{}, fmt.Errorf("read skill %q: %w", skillID, err)
	}
	return content, result, nil
}

func skillLabel(skillPaths []string, workingDir string, skill *Skill) (string, SourceType) {
	if skill.Builtin {
		return string(SourceSystem) + ":" + skill.Name, SourceSystem
	}

	cleanFile := filepath.Clean(skill.SkillFilePath)
	for _, base := range skillPaths {
		cleanBase := filepath.Clean(base)
		rel, err := filepath.Rel(cleanBase, cleanFile)
		if err != nil || escapesParent(rel) {
			continue
		}

		source := SourceUser
		prefix := string(SourceUser) + ":"
		if isProjectSkillPath(cleanBase, workingDir) {
			source = SourceProject
			prefix = string(SourceProject) + ":"
		}
		return prefix + filepath.Base(filepath.Dir(cleanFile)), source
	}

	return string(SourceUser) + ":" + filepath.Base(filepath.Dir(cleanFile)), SourceUser
}

func escapesParent(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isProjectSkillPath(basePath, workingDir string) bool {
	if workingDir == "" {
		return false
	}
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return false
	}
	absWD, err := filepath.Abs(workingDir)
	if err != nil {
		return false
	}
	cleanBase := filepath.Clean(absBase)
	cleanWD := filepath.Clean(absWD)
	rel, err := filepath.Rel(cleanWD, cleanBase)
	if err != nil {
		return false
	}
	return !escapesParent(rel)
}
