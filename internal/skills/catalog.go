package skills

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
)

// DiscoverySource provides the configuration surface needed to resolve and
// discover skills for a workspace.
type DiscoverySource interface {
	Config() *config.Config
	WorkingDir() string
	Resolver() config.VariableResolver
}

// SourceType describes where a visible skill comes from.
type SourceType string

const (
	SourceSystem  SourceType = "system"
	SourceUser    SourceType = "user"
	SourceProject SourceType = "project"
)

// CatalogEntry describes an effective visible skill.
type CatalogEntry struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Label       string     `json:"label"`
	Source      SourceType `json:"source"`
}

// SkillReadResult holds metadata about a skill returned alongside its
// content.
type SkillReadResult struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Source      SourceType `json:"source"`
	Builtin     bool       `json:"builtin"`
}

// ErrSkillNotFound is returned when a skill ID is not part of the effective
// visible skill set.
var ErrSkillNotFound = errors.New("skill not found")

// ResolvePath expands home directory references and environment variables in a
// configured skill path. Relative paths are left relative to the process CWD.
func ResolvePath(src DiscoverySource, path string) string {
	resolved := home.Long(path)
	if src != nil && src.Resolver() != nil && strings.Contains(resolved, "$") {
		expanded, err := src.Resolver().ResolveValue(resolved)
		if err != nil {
			slog.Debug("Failed to resolve variables in skill path",
				"path", resolved, "error", err)
		} else {
			resolved = expanded
		}
	}
	return filepath.Clean(resolved)
}

// ExpandedPaths returns the configured skill roots after applying workspace-
// relative resolution.
func ExpandedPaths(src DiscoverySource) []string {
	if src == nil || src.Config() == nil || src.Config().Options == nil {
		return nil
	}

	paths := make([]string, 0, len(src.Config().Options.SkillsPaths))
	for _, pth := range src.Config().Options.SkillsPaths {
		paths = append(paths, ResolvePath(src, pth))
	}
	return paths
}

// All returns the full discovered skill set for a workspace after dedup but
// before disabled-skill filtering. This is the pre-filter list used by the
// coordinator to feed the skill tracker.
func All(src DiscoverySource) []*Skill {
	all, _, _ := effective(src)
	return all
}

// Effective returns the visible skill set for a workspace, matching prompt
// construction semantics: builtin skills, then user/project discovery, then
// user-over-builtin deduplication, then disabled skill filtering.
func Effective(src DiscoverySource) []*Skill {
	_, active, _ := effective(src)
	return active
}

// effective is the shared implementation that returns the pre-filter
// (all) list, the post-filter (active) list, and the expanded paths
// used during discovery.
func effective(src DiscoverySource) (all, active []*Skill, paths []string) {
	allSkills := DiscoverBuiltin()
	builtinNames := make(map[string]bool, len(allSkills))
	for _, skill := range allSkills {
		builtinNames[skill.Name] = true
	}

	paths = ExpandedPaths(src)
	userSkills := Discover(paths)
	sortDiscoveredSkills(paths, userSkills)
	for _, userSkill := range userSkills {
		if builtinNames[userSkill.Name] {
			slog.Warn("User skill overrides builtin skill", "name", userSkill.Name)
		}
		allSkills = append(allSkills, userSkill)
	}

	allSkills = Deduplicate(allSkills)

	if src == nil || src.Config() == nil || src.Config().Options == nil {
		return allSkills, allSkills, paths
	}
	return allSkills, Filter(allSkills, src.Config().Options.DisabledSkills), paths
}

// Catalog returns the effective visible skills formatted for frontend display.
func Catalog(src DiscoverySource) []CatalogEntry {
	_, resolved, skillPaths := effective(src)
	entries := make([]CatalogEntry, 0, len(resolved))
	workingDir := ""
	if src != nil {
		workingDir = src.WorkingDir()
	}

	for _, skill := range resolved {
		label, source := skillLabel(skillPaths, workingDir, skill)
		entries = append(entries, CatalogEntry{
			ID:          skill.SkillFilePath,
			Name:        skill.Name,
			Description: skill.Description,
			Label:       label,
			Source:      source,
		})
	}

	return entries
}

// FindEffective returns the visible skill with the given ID.
func FindEffective(src DiscoverySource, skillID string) (*Skill, error) {
	for _, skill := range Effective(src) {
		if skill.SkillFilePath == skillID {
			return skill, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrSkillNotFound, skillID)
}

// ReadContent reads the contents of a visible skill by ID and returns
// the raw bytes along with metadata about the skill.
func ReadContent(src DiscoverySource, skillID string) ([]byte, SkillReadResult, error) {
	skill, err := FindEffective(src, skillID)
	if err != nil {
		return nil, SkillReadResult{}, err
	}

	result := SkillReadResult{
		Name:        skill.Name,
		Description: skill.Description,
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

	// Fallback: use the parent directory name rather than the full path
	// so internal directory structure is not exposed to the UI.
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

func sortDiscoveredSkills(skillPaths []string, discovered []*Skill) {
	sort.SliceStable(discovered, func(i, j int) bool {
		rootI := matchingSkillRootIndex(skillPaths, discovered[i].SkillFilePath)
		rootJ := matchingSkillRootIndex(skillPaths, discovered[j].SkillFilePath)
		if rootI != rootJ {
			return rootI < rootJ
		}
		return filepath.Clean(discovered[i].SkillFilePath) < filepath.Clean(discovered[j].SkillFilePath)
	})
}

func matchingSkillRootIndex(skillPaths []string, skillFilePath string) int {
	cleanFile := filepath.Clean(skillFilePath)
	for i, root := range skillPaths {
		rel, err := filepath.Rel(filepath.Clean(root), cleanFile)
		if err == nil && !escapesParent(rel) {
			return i
		}
	}
	return len(skillPaths)
}
