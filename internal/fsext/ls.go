package fsext

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charlievieth/fastwalk"
	ignore "github.com/sabhiram/go-gitignore"
)

// CommonIgnorePatterns contains commonly ignored files and directories
var CommonIgnorePatterns = []string{
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
}

type DirectoryLister struct {
	gitignore    *ignore.GitIgnore
	commonIgnore *ignore.GitIgnore
	rootPath     string
}

func NewDirectoryLister(rootPath string) *DirectoryLister {
	dl := &DirectoryLister{
		rootPath: rootPath,
	}

	// Load gitignore if it exists
	gitignorePath := filepath.Join(rootPath, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		if gi, err := ignore.CompileIgnoreFile(gitignorePath); err == nil {
			dl.gitignore = gi
		}
	}

	// Create common ignore patterns
	dl.commonIgnore = ignore.CompileIgnoreLines(CommonIgnorePatterns...)

	return dl
}

func (dl *DirectoryLister) shouldIgnore(path string, ignorePatterns []string) bool {
	relPath, err := filepath.Rel(dl.rootPath, path)
	if err != nil {
		relPath = path
	}

	// Check common ignore patterns
	if dl.commonIgnore.MatchesPath(relPath) {
		return true
	}

	// Check gitignore patterns if available
	if dl.gitignore != nil && dl.gitignore.MatchesPath(relPath) {
		return true
	}

	base := filepath.Base(path)

	if base != "." && strings.HasPrefix(base, ".") {
		return true
	}

	for _, pattern := range ignorePatterns {
		matched, err := filepath.Match(pattern, base)
		if err == nil && matched {
			return true
		}
	}
	return false
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
	if err != nil {
		return nil, truncated, err
	}

	return results, truncated, nil
}
