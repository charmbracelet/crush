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
	ignore "github.com/sabhiram/go-gitignore"
)

// commonIgnorePatterns contains commonly ignored files and directories
var commonIgnorePatterns = sync.OnceValue(func() ignore.IgnoreParser {
	return ignore.CompileIgnoreLines(
		// Version control
		".git",
		".svn",
		".hg",
		".bzr",

		// IDE and editor files
		".vscode",
		".idea",
		"*.swp",
		"*.swo",
		"*~",
		".DS_Store",
		"Thumbs.db",

		// Build artifacts and dependencies
		"node_modules",
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

		// Logs and temporary files
		"*.log",
		"*.tmp",
		"*.temp",
		".cache",
		".tmp",

		// Language-specific
		"__pycache__",
		"*.pyc",
		"*.pyo",
		".pytest_cache",
		"vendor",
		"Cargo.lock",
		"package-lock.json",
		"yarn.lock",
		"pnpm-lock.yaml",

		// OS generated files
		".Trash",
		".Spotlight-V100",
		".fseventsd",

		// Crush
		".crush",

		// macOS stuff
		"OrbStack",
		".local",
		".share",
	)
})

var homeIgnore = sync.OnceValue(func() ignore.IgnoreParser {
	home := home.Dir()
	var lines []string
	for _, name := range []string{
		filepath.Join(home, ".gitignore"),
		filepath.Join(home, ".config", "git", "ignore"),
		filepath.Join(home, ".config", "crush", "ignore"),
	} {
		if bts, err := os.ReadFile(name); err == nil {
			lines = append(lines, strings.Split(string(bts), "\n")...)
		}
	}
	return ignore.CompileIgnoreLines(lines...)
})

type directoryLister struct {
	ignores  *csync.Map[string, ignore.IgnoreParser]
	rootPath string
}

func NewDirectoryLister(rootPath string) *directoryLister {
	dl := &directoryLister{
		rootPath: rootPath,
		ignores:  csync.NewMap[string, ignore.IgnoreParser](),
	}
	dl.getIgnore(rootPath)
	return dl
}

// git checks, in order:
// - ./.gitignore, ../.gitignore, etc, until repo root
// ~/.config/git/ignore
// ~/.gitignore
//
// This will do the following:
// - the given ignorePatterns
// - [commonIgnorePatterns]
// - ./.gitignore, ../.gitignore, etc, until dl.rootPath
// - ./.crushignore, ../.crushignore, etc, until dl.rootPath
// ~/.config/git/ignore
// ~/.gitignore
// ~/.config/crush/ignore
func (dl *directoryLister) shouldIgnore(path string, ignorePatterns []string) bool {
	if len(ignorePatterns) > 0 {
		base := filepath.Base(path)
		for _, pattern := range ignorePatterns {
			if matched, err := filepath.Match(pattern, base); err == nil && matched {
				return true
			}
		}
	}

	// Don't apply gitignore rules to the root directory itself
	// In gitignore semantics, patterns don't apply to the repo root
	if path == dl.rootPath {
		return false
	}

	relPath, err := filepath.Rel(dl.rootPath, path)
	if err != nil {
		relPath = path
	}

	if commonIgnorePatterns().MatchesPath(relPath) {
		slog.Debug("Ignoring common pattern", "path", relPath)
		return true
	}

	parentDir := filepath.Dir(path)
	ignoreParser := dl.getIgnore(parentDir)
	if ignoreParser.MatchesPath(relPath) {
		slog.Debug("Ignoring dir pattern", "path", relPath, "dir", parentDir)
		return true
	}

	// For directories, also check with trailing slash (gitignore convention)
	if ignoreParser.MatchesPath(relPath + "/") {
		slog.Debug("Ignoring dir pattern with slash", "path", relPath+"/", "dir", parentDir)
		return true
	}

	if dl.checkParentIgnores(relPath) {
		return true
	}

	if homeIgnore().MatchesPath(relPath) {
		slog.Debug("Ignoring home dir pattern", "path", relPath)
		return true
	}

	return false
}

func (dl *directoryLister) checkParentIgnores(path string) bool {
	parent := filepath.Dir(filepath.Dir(path))
	for parent != "." && path != "." {
		if dl.getIgnore(parent).MatchesPath(path) {
			slog.Debug("Ignoring parent dir pattern", "path", path, "dir", parent)
			return true
		}
		if parent == dl.rootPath {
			break
		}
		parent = filepath.Dir(parent)
	}
	return false
}

func (dl *directoryLister) getIgnore(path string) ignore.IgnoreParser {
	return dl.ignores.GetOrSet(path, func() ignore.IgnoreParser {
		var lines []string
		for _, ign := range []string{".crushignore", ".gitignore"} {
			name := filepath.Join(path, ign)
			if content, err := os.ReadFile(name); err == nil {
				lines = append(lines, strings.Split(string(content), "\n")...)
			}
		}
		if len(lines) == 0 {
			// Return a no-op parser to avoid nil checks
			return ignore.CompileIgnoreLines()
		}
		return ignore.CompileIgnoreLines(lines...)
	})
}

// ListDirectory lists files and directories in the specified path,
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

		if dl.shouldIgnore(path, ignorePatterns) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if path != initialPath {
			if d.IsDir() {
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
