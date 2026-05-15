package skills

import (
	"path/filepath"
	"sort"
	"strings"
)

// Source represents the origin category of a skill.
type Source string

const (
	SourceBuiltin Source = "Builtin"
	SourceProject Source = "Project"
	SourceUser    Source = "User"
)

// SourceGroup holds a source label and the skills that belong to it.
type SourceGroup struct {
	Source Source
	Skills []*Skill
}

// GroupBySource partitions skills by origin. projectDirs lists the
// project-level skill directories so that non-builtin skills found
// under one of those directories are classified as Project rather than
// User. The returned slice is ordered Builtin, Project, User, and
// skills within each group are sorted by name. Empty groups are omitted.
func GroupBySource(all []*Skill, projectDirs []string) []SourceGroup {
	groups := map[Source][]*Skill{}

	for _, s := range all {
		src := ClassifySource(s, projectDirs)
		groups[src] = append(groups[src], s)
	}

	for src := range groups {
		sort.Slice(groups[src], func(i, j int) bool {
			return strings.ToLower(groups[src][i].Name) < strings.ToLower(groups[src][j].Name)
		})
	}

	var result []SourceGroup
	for _, src := range []Source{SourceBuiltin, SourceProject, SourceUser} {
		if len(groups[src]) > 0 {
			result = append(result, SourceGroup{Source: src, Skills: groups[src]})
		}
	}
	return result
}

// ClassifySource determines the source category for a single skill.
func ClassifySource(s *Skill, projectDirs []string) Source {
	if s.Builtin {
		return SourceBuiltin
	}
	for _, dir := range projectDirs {
		cleaned := filepath.Clean(dir)
		if cleaned != "" && strings.HasPrefix(filepath.Clean(s.Path), cleaned) {
			return SourceProject
		}
	}
	return SourceUser
}
