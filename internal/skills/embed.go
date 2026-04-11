package skills

import (
	"embed"
	"io/fs"
	"log/slog"
	"path/filepath"
)

// BuiltinPrefix is the path prefix for builtin skill files. It is used by
// the View tool to distinguish embedded files from disk files.
const BuiltinPrefix = "crush://skills/"

//go:embed builtin/*
var builtinFS embed.FS

// BuiltinFS returns the embedded filesystem containing builtin skills.
func BuiltinFS() embed.FS {
	return builtinFS
}

// DiscoverBuiltin finds all valid skills embedded in the binary.
func DiscoverBuiltin() []*Skill {
	var discovered []*Skill

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
			return nil
		}

		skill, err := ParseContent(content)
		if err != nil {
			slog.Warn("Failed to parse builtin skill file", "path", path, "error", err)
			return nil
		}

		// Set paths using the crush prefix. Strip the leading "builtin/"
		// so the path is relative to the embedded root
		// (e.g., "crush://skills/crush-config/SKILL.md").
		relPath, _ := filepath.Rel("builtin", path)
		relPath = filepath.ToSlash(relPath)
		skill.SkillFilePath = BuiltinPrefix + relPath
		skill.Path = BuiltinPrefix + filepath.Dir(relPath)
		skill.Builtin = true

		if err := skill.Validate(); err != nil {
			slog.Warn("Builtin skill validation failed", "path", path, "error", err)
			return nil
		}

		slog.Debug("Successfully loaded builtin skill", "name", skill.Name, "path", skill.SkillFilePath)
		discovered = append(discovered, skill)
		return nil
	})

	return discovered
}
