package fsext

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/charlievieth/fastwalk"
	"github.com/charmbracelet/crush/internal/csync"
	ignore "github.com/sabhiram/go-gitignore"
)

// commonIgnorePatterns contains commonly ignored files and directories
var commonIgnorePatterns = ignore.CompileIgnoreLines(
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
)

type directoryLister struct {
	ignores  *csync.Map[string, ignore.IgnoreParser]
	rootPath string
}

func NewDirectoryLister(rootPath string) *directoryLister {
	return &directoryLister{
		rootPath: rootPath,
		ignores:  csync.NewMap[string, ignore.IgnoreParser](),
	}
}

func (dl *directoryLister) shouldIgnore(path string, ignorePatterns []string) bool {
	relPath, err := filepath.Rel(dl.rootPath, path)
	if err != nil {
		relPath = path
	}

	base := filepath.Base(path)
	for _, pattern := range ignorePatterns {
		matched, err := filepath.Match(pattern, base)
		if err == nil && matched {
			slog.Info("ignoring path", "path", path)
			return true
		}
	}

	if commonIgnorePatterns.MatchesPath(relPath) || dl.getIgnore(path).MatchesPath(relPath) {
		slog.Info("ignoring path", "path", path)
		return true
	}

	parent := filepath.Dir(path)
	for {
		if dl.getIgnore(parent).MatchesPath(path) {
			slog.Info("ignoring path", "path", path, "parent", parent)
			return true
		}
		if parent == "/" || parent == "." { // TODO: windows
			return false
		}
		parent = filepath.Dir(parent)
	}
}

func (dl *directoryLister) getIgnore(path string) ignore.IgnoreParser {
	return dl.ignores.GetOrSet(path, func() ignore.IgnoreParser {
		var b bytes.Buffer
		for _, ign := range []string{".crushignore", ".gitignore"} {
			p := filepath.Join(path, ign)
			if _, err := os.Stat(p); err == nil {
				slog.Info("loading ignore file", "path", p)
				f, err := os.Open(p)
				if err != nil {
					_ = f.Close()
					slog.Error("Failed to open ignore file", "path", p, "error", err)
					continue
				}
				if _, err := io.Copy(&b, f); err != nil {
					slog.Error("Failed to read ignore file", "path", p, "error", err)
				}
				_ = f.Close()
			}
		}
		return ignore.CompileIgnoreLines(strings.Split(b.String(), "\n")...)
	})
}

// ListDirectory lists files and directories in the specified path,
func ListDirectory(initialPath string, ignorePatterns []string, limit int) ([]string, bool, error) {
	var results []string
	truncated := false
	dl := NewDirectoryLister(initialPath)

	conf := fastwalk.Config{
		Follow: true,
		// Use forward slashes when running a Windows binary under WSL or MSYS
		ToSlash: fastwalk.DefaultToSlash(),
		Sort:    fastwalk.SortDirsFirst,
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
			results = append(results, path)
		}

		if limit > 0 && len(results) >= limit {
			truncated = true
			return filepath.SkipAll
		}

		return nil
	})
	if err != nil && len(results) == 0 {
		return nil, truncated, err
	}

	return results, truncated, nil
}
