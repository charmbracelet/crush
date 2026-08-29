package skills

import (
	"embed"
	"io/fs"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
)

// BuiltinPrefix is the path prefix for builtin skill files. It is used by
// the View tool to distinguish embedded files from disk files.
const BuiltinPrefix = "ultra://skills/"

//go:embed builtin/*
var builtinFS embed.FS

// BuiltinFS returns the embedded filesystem containing builtin skills.
func BuiltinFS() embed.FS {
	return builtinFS
}

// DiscoverBuiltin finds all valid skills embedded in the binary.
func DiscoverBuiltin() []*Skill {
	skills, _ := DiscoverBuiltinWithStates()
	return skills
}

// DiscoverBuiltinWithStates is like DiscoverBuiltin but additionally returns
// a per-file state slice describing parse/validation outcomes. Useful for
// diagnostics.
func DiscoverBuiltinWithStates() ([]*Skill, []*SkillState) {
	var discovered []*Skill
	var states []*SkillState

	fs.WalkDir(builtinFS, "builtin", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || d.Name() != SkillFileName {
			return nil
		}

		content, err := builtinFS.ReadFile(path)
		if err != nil {
			slog.Warn("Failed to read builtin skill file", "path", path, "error", err)
			states = append(states, &SkillState{Path: path, State: StateError, Err: err})
			return nil
		}

		skill, err := ParseContent(content)
		if err != nil {
			slog.Warn("Failed to parse builtin skill file", "path", path, "error", err)
			states = append(states, &SkillState{Path: path, State: StateError, Err: err})
			return nil
		}

		// Set paths using the ultra prefix. Strip the leading "builtin/"
		// so the path is relative to the embedded root
		// (e.g., "ultra://skills/ultra-config/SKILL.md").
		relPath, _ := filepath.Rel("builtin", path)
		relPath = filepath.ToSlash(relPath)
		skill.SkillFilePath = BuiltinPrefix + relPath
		skill.Path = BuiltinPrefix + filepath.Dir(relPath)
		skill.Builtin = true

		if err := skill.Validate(); err != nil {
			slog.Warn("Builtin skill validation failed", "path", path, "error", err)
			states = append(states, &SkillState{Name: skill.Name, Path: path, State: StateError, Err: err})
			return nil
		}

		slog.Debug("Successfully loaded builtin skill", "name", skill.Name, "path", skill.SkillFilePath)
		discovered = append(discovered, skill)
		states = append(states, &SkillState{Name: skill.Name, Path: skill.SkillFilePath, State: StateNormal})
		return nil
	})

	// Keep Ultra-specific guidance ahead of generic builtins. This preserves
	// stable prompt ordering when branded skill directories are renamed.
	slices.SortStableFunc(discovered, func(a, b *Skill) int {
		aUltra := strings.HasPrefix(a.Name, "ultra-")
		bUltra := strings.HasPrefix(b.Name, "ultra-")
		if aUltra != bUltra {
			if aUltra {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return discovered, states
}
