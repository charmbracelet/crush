package fsext

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/charlievieth/fastwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// fastIgnoreDirs is a set of directory names that are always ignored.
// This provides O(1) lookup for common cases to avoid expensive pattern matching.
var fastIgnoreDirs = map[string]bool{
	".git":            true,
	".svn":            true,
	".hg":             true,
	".bzr":            true,
	".vscode":         true,
	".idea":           true,
	"node_modules":    true,
	"__pycache__":     true,
	".pytest_cache":   true,
	".cache":          true,
	".tmp":            true,
	".Trash":          true,
	".Spotlight-V100": true,
	".fseventsd":      true,
	".crush":          true,
	"OrbStack":        true,
	".local":          true,
	".share":          true,
}

// commonIgnorePatterns contains commonly ignored files and directories.
// Note: Exact directory names that are in fastIgnoreDirs are handled there for O(1) lookup.
// This list contains wildcard patterns and file-specific patterns.
var commonIgnorePatterns = sync.OnceValue(func() []gitignore.Pattern {
	patterns := []string{
		// IDE and editor files (wildcards)
		"*.swp",
		"*.swo",
		"*~",
		".DS_Store",
		"Thumbs.db",

		// Build artifacts (non-fastIgnoreDirs)
		"target",
		"build",
		"dist",
		"out",
		"bin",
		"obj",
		"*.o",
		"*.so",
		"*.dylib",
		"*.dll",
		"*.exe",

		// Logs and temporary files (wildcards)
		"*.log",
		"*.tmp",
		"*.temp",

		// Language-specific (wildcards and non-fastIgnoreDirs)
		"*.pyc",
		"*.pyo",
		"vendor",
		"Cargo.lock",
		"package-lock.json",
		"yarn.lock",
		"pnpm-lock.yaml",
	}
	return parsePatterns(patterns, nil)
})

var homeIgnorePatterns = sync.OnceValue(func() []gitignore.Pattern {
	homeDir := home.Dir()
	var lines []string
	for _, name := range []string{
		filepath.Join(homeDir, ".gitignore"),
		filepath.Join(homeDir, ".config", "git", "ignore"),
		filepath.Join(homeDir, ".config", "crush", "ignore"),
	} {
		if bts, err := os.ReadFile(name); err == nil {
			lines = append(lines, strings.Split(string(bts), "\n")...)
		}
	}
	return parsePatterns(lines, nil)
})

// parsePatterns parses gitignore pattern strings into Pattern objects.
// domain is the path components where the patterns are defined (nil for global).
func parsePatterns(lines []string, domain []string) []gitignore.Pattern {
	var patterns []gitignore.Pattern
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(line, domain))
	}
	return patterns
}

type directoryLister struct {
	// dirPatterns caches parsed patterns from .gitignore/.crushignore for each directory.
	// This avoids re-reading files when building combined matchers.
	dirPatterns *csync.Map[string, []gitignore.Pattern]
	// combinedMatchers caches a combined matcher for each directory that includes
	// all ancestor patterns. This allows O(1) matching per file.
	combinedMatchers *csync.Map[string, gitignore.Matcher]
	rootPath         string
}

func NewDirectoryLister(rootPath string) *directoryLister {
	return &directoryLister{
		rootPath:         rootPath,
		dirPatterns:      csync.NewMap[string, []gitignore.Pattern](),
		combinedMatchers: csync.NewMap[string, gitignore.Matcher](),
	}
}

// pathToComponents splits a path into its components for gitignore matching.
func pathToComponents(path string) []string {
	path = filepath.ToSlash(path)
	if path == "" || path == "." {
		return nil
	}
	return strings.Split(path, "/")
}

// getDirPatterns returns the parsed patterns for a specific directory's
// .gitignore and .crushignore files. Results are cached.
func (dl *directoryLister) getDirPatterns(dir string) []gitignore.Pattern {
	return dl.dirPatterns.GetOrSet(dir, func() []gitignore.Pattern {
		var allPatterns []gitignore.Pattern

		relPath, _ := filepath.Rel(dl.rootPath, dir)
		var domain []string
		if relPath != "" && relPath != "." {
			domain = pathToComponents(relPath)
		}

		for _, ignoreFile := range []string{".gitignore", ".crushignore"} {
			ignPath := filepath.Join(dir, ignoreFile)
			if content, err := os.ReadFile(ignPath); err == nil {
				lines := strings.Split(string(content), "\n")
				allPatterns = append(allPatterns, parsePatterns(lines, domain)...)
			}
		}
		return allPatterns
	})
}

// getCombinedMatcher returns a matcher that combines all gitignore patterns
// from the root to the given directory, plus common patterns and home patterns.
// Results are cached per directory, and we reuse parent directory matchers.
func (dl *directoryLister) getCombinedMatcher(dir string) gitignore.Matcher {
	return dl.combinedMatchers.GetOrSet(dir, func() gitignore.Matcher {
		var allPatterns []gitignore.Pattern

		// Add common patterns first (lowest priority).
		allPatterns = append(allPatterns, commonIgnorePatterns()...)

		// Add home ignore patterns.
		allPatterns = append(allPatterns, homeIgnorePatterns()...)

		// Collect patterns from root to this directory.
		relDir, _ := filepath.Rel(dl.rootPath, dir)
		var pathParts []string
		if relDir != "" && relDir != "." {
			pathParts = pathToComponents(relDir)
		}

		// Add patterns from each directory from root to current.
		currentPath := dl.rootPath
		allPatterns = append(allPatterns, dl.getDirPatterns(currentPath)...)

		for _, part := range pathParts {
			currentPath = filepath.Join(currentPath, part)
			allPatterns = append(allPatterns, dl.getDirPatterns(currentPath)...)
		}

		return gitignore.NewMatcher(allPatterns)
	})
}

// shouldIgnore checks if a path should be ignored based on gitignore rules.
// This uses a combined matcher that includes all ancestor patterns for O(1) matching.
func (dl *directoryLister) shouldIgnore(path string, ignorePatterns []string, isDir bool) bool {
	base := filepath.Base(path)

	// Fast path: O(1) lookup for commonly ignored directories.
	if isDir && fastIgnoreDirs[base] {
		return true
	}

	// Check explicit ignore patterns.
	if len(ignorePatterns) > 0 {
		for _, pattern := range ignorePatterns {
			if matched, err := filepath.Match(pattern, base); err == nil && matched {
				return true
			}
		}
	}

	// Don't apply gitignore rules to the root directory itself.
	if path == dl.rootPath {
		return false
	}

	relPath, err := filepath.Rel(dl.rootPath, path)
	if err != nil {
		relPath = path
	}

	pathComponents := pathToComponents(relPath)
	if len(pathComponents) == 0 {
		return false
	}

	// Get the combined matcher for the parent directory.
	parentDir := filepath.Dir(path)
	matcher := dl.getCombinedMatcher(parentDir)

	if matcher.Match(pathComponents, isDir) {
		slog.Debug("Ignoring path", "path", relPath)
		return true
	}

	return false
}

// ListDirectory lists files and directories in the specified path.
func ListDirectory(initialPath string, ignorePatterns []string, depth, limit int) ([]string, bool, error) {
	found := csync.NewSlice[string]()
	dl := NewDirectoryLister(initialPath)

	slog.Debug("Listing directory", "path", initialPath, "depth", depth, "limit", limit, "ignorePatterns", ignorePatterns)

	conf := fastwalk.Config{
		Follow:   true,
		ToSlash:  fastwalk.DefaultToSlash(),
		Sort:     fastwalk.SortDirsFirst,
		MaxDepth: depth,
	}

	err := fastwalk.Walk(&conf, initialPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files we don't have permission to access
		}

		isDir := d.IsDir()
		if dl.shouldIgnore(path, ignorePatterns, isDir) {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}

		if path != initialPath {
			if isDir {
				path = path + string(filepath.Separator)
			}
			found.Append(path)
		}

		if limit > 0 && found.Len() >= limit {
			return filepath.SkipAll
		}

		return nil
	})
	if err != nil && !errors.Is(err, filepath.SkipAll) {
		return nil, false, err
	}

	matches, truncated := truncate(slices.Collect(found.Seq()), limit)
	return matches, truncated || errors.Is(err, filepath.SkipAll), nil
}
