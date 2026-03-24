package dialog

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/skills"
)

type SkillSourceType uint

const (
	SystemSkills SkillSourceType = iota
	UserSkills
)

func (s SkillSourceType) String() string { return []string{"System", "User"}[s] }

type SkillEntry struct {
	ID          string
	Name        string
	Description string
	Path        string
	Source      SkillSourceType
	DisplayPath string
	Label       string
}

// LoadSystemSkillEntries returns entries for all skills embedded in the
// Crush binary.
func LoadSystemSkillEntries() []SkillEntry {
	system := skills.DiscoverBuiltin()
	entries := make([]SkillEntry, 0, len(system))
	for _, skill := range system {
		entries = append(entries, SkillEntry{
			ID:          skill.SkillFilePath,
			Name:        skill.Name,
			Description: skill.Description,
			Path:        skill.SkillFilePath,
			Source:      SystemSkills,
			DisplayPath: "system:" + skill.Name,
			Label:       "system:" + skill.Name,
		})
	}
	return entries
}

func LoadUserSkillEntries(store *config.ConfigStore) []SkillEntry {
	paths := expandedSkillPaths(store)
	if len(paths) == 0 {
		return nil
	}

	workingDir := ""
	if store != nil {
		workingDir = store.WorkingDir()
	}

	discovered := skills.Discover(paths)
	entries := make([]SkillEntry, 0, len(discovered))
	for _, skill := range discovered {
		displayPath := displaySkillPath(paths, skill.SkillFilePath, workingDir)
		entries = append(entries, SkillEntry{
			ID:          skill.SkillFilePath,
			Name:        skill.Name,
			Description: skill.Description,
			Path:        skill.SkillFilePath,
			Source:      UserSkills,
			DisplayPath: displayPath,
			Label:       displayPath,
		})
	}
	return entries
}

func expandedSkillPaths(store *config.ConfigStore) []string {
	if store == nil || store.Config() == nil || store.Config().Options == nil {
		return nil
	}

	paths := make([]string, 0, len(store.Config().Options.SkillsPaths))
	for _, pth := range store.Config().Options.SkillsPaths {
		expanded := home.Long(pth)
		if strings.HasPrefix(expanded, "$") {
			if resolved, err := store.Resolver().ResolveValue(expanded); err == nil {
				expanded = resolved
			}
		}
		paths = append(paths, expanded)
	}
	return paths
}

func displaySkillPath(skillPaths []string, skillFilePath, workingDir string) string {
	cleanFile := filepath.Clean(skillFilePath)
	for _, base := range skillPaths {
		cleanBase := filepath.Clean(base)
		rel, err := filepath.Rel(cleanBase, cleanFile)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		prefix := "user:"
		if isProjectSkillPath(cleanBase, workingDir) {
			prefix = "project:"
		}
		return prefix + filepath.Base(filepath.Dir(cleanFile))
	}
	return filepath.ToSlash(cleanFile)
}

// isProjectSkillPath reports whether the skills base directory lives inside
// the current working directory. Skills loaded from global/absolute paths
// (e.g. ~/.config/agents/skills) are user skills; those from relative or
// working-directory-rooted paths are project skills.
func isProjectSkillPath(basePath, workingDir string) bool {
	if workingDir == "" {
		return false
	}
	cleanBase := filepath.Clean(basePath)
	cleanWD := filepath.Clean(workingDir)
	rel, err := filepath.Rel(cleanWD, cleanBase)
	if err != nil {
		return false
	}
	// If the relative path doesn't escape the working directory, it's a
	// project skill.
	return !strings.HasPrefix(rel, "..")
}
