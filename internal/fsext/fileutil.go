package fsext

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/charlievieth/fastwalk"
	"github.com/charmbracelet/crush/internal/home"

	ignore "github.com/sabhiram/go-gitignore"
)

type FileInfo struct {
	Path    string
	ModTime time.Time
}

func SkipHidden(path string) bool {
	// Check for hidden files (starting with a dot)
	base := filepath.Base(path)
	if base != "." && strings.HasPrefix(base, ".") {
		return true
	}

	commonIgnoredDirs := map[string]bool{
		".crush":           true,
		"node_modules":     true,
		"vendor":           true,
		"dist":             true,
		"build":            true,
		"target":           true,
		".git":             true,
		".idea":            true,
		".vscode":          true,
		"__pycache__":      true,
		"bin":              true,
		"obj":              true,
		"out":              true,
		"coverage":         true,
		"logs":             true,
		"generated":        true,
		"bower_components": true,
		"jspm_packages":    true,
	}

	parts := strings.SplitSeq(path, string(os.PathSeparator))
	for part := range parts {
		if commonIgnoredDirs[part] {
			return true
		}
	}
	return false
}

// FastGlobWalker provides gitignore-aware file walking with fastwalk
type FastGlobWalker struct {
	gitignore   *ignore.GitIgnore
	crushignore *ignore.GitIgnore
	rootPath    string
}

func NewFastGlobWalker(searchPath string) *FastGlobWalker {
	walker := &FastGlobWalker{
		rootPath: searchPath,
	}

	// Load gitignore if it exists
	gitignorePath := filepath.Join(searchPath, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		if gi, err := ignore.CompileIgnoreFile(gitignorePath); err == nil {
			walker.gitignore = gi
		}
	}

	// Load crushignore if it exists
	crushignorePath := filepath.Join(searchPath, ".crushignore")
	if _, err := os.Stat(crushignorePath); err == nil {
		if ci, err := ignore.CompileIgnoreFile(crushignorePath); err == nil {
			walker.crushignore = ci
		}
	}

	return walker
}

// ShouldSkip checks if a path should be skipped based on gitignore, crushignore, and hidden file rules
func (w *FastGlobWalker) ShouldSkip(path string) bool {
	if SkipHidden(path) {
		return true
	}

	relPath, err := filepath.Rel(w.rootPath, path)
	if err != nil {
		return false
	}

	if w.gitignore != nil {
		if w.gitignore.MatchesPath(relPath) {
			return true
		}
	}

	if w.crushignore != nil {
		if w.crushignore.MatchesPath(relPath) {
			return true
		}
	}

	return false
}

func GlobWithDoubleStar(pattern, searchPath string, limit int) ([]string, bool, error) {
	walker := NewFastGlobWalker(searchPath)
	var matches []FileInfo
	conf := fastwalk.Config{
		Follow: true,
		// Use forward slashes when running a Windows binary under WSL or MSYS
		ToSlash: fastwalk.DefaultToSlash(),
		Sort:    fastwalk.SortFilesFirst,
	}
	err := fastwalk.Walk(&conf, searchPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		if d.IsDir() {
			if walker.ShouldSkip(path) {
				return filepath.SkipDir
			}
			return nil
		}

		if walker.ShouldSkip(path) {
			return nil
		}

		// Check if path matches the pattern
		relPath, err := filepath.Rel(searchPath, path)
		if err != nil {
			relPath = path
		}

		matched, err := doublestar.Match(pattern, relPath)
		if err != nil || !matched {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		matches = append(matches, FileInfo{Path: path, ModTime: info.ModTime()})
		if limit > 0 && len(matches) >= limit*2 {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("fastwalk error: %w", err)
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].ModTime.After(matches[j].ModTime)
	})

	truncated := false
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
		truncated = true
	}

	results := make([]string, len(matches))
	for i, m := range matches {
		results[i] = m.Path
	}
	return results, truncated, nil
}

func PrettyPath(path string) string {
	return home.Short(path)
}

func DirTrim(pwd string, lim int) string {
	var (
		out string
		sep = string(filepath.Separator)
	)
	dirs := strings.Split(pwd, sep)
	if lim > len(dirs)-1 || lim <= 0 {
		return pwd
	}
	for i := len(dirs) - 1; i > 0; i-- {
		out = sep + out
		if i == len(dirs)-1 {
			out = dirs[i]
		} else if i >= len(dirs)-lim {
			out = string(dirs[i][0]) + out
		} else {
			out = "..." + out
			break
		}
	}
	out = filepath.Join("~", out)
	return out
}

// PathOrPrefix returns the prefix if the path starts with it, or falls back to
// the path otherwise.
func PathOrPrefix(path, prefix string) string {
	if HasPrefix(path, prefix) {
		return prefix
	}
	return path
}

// HasPrefix checks if the given path starts with the specified prefix.
// Uses filepath.Rel to determine if path is within prefix.
func HasPrefix(path, prefix string) bool {
	rel, err := filepath.Rel(prefix, path)
	if err != nil {
		return false
	}
	// If path is within prefix, Rel will not return a path starting with ".."
	return !strings.HasPrefix(rel, "..")
}

// ToUnixLineEndings converts Windows line endings (CRLF) to Unix line endings (LF).
func ToUnixLineEndings(content string) (string, bool) {
	if strings.Contains(content, "\r\n") {
		return strings.ReplaceAll(content, "\r\n", "\n"), true
	}
	return content, false
}

// ToWindowsLineEndings converts Unix line endings (LF) to Windows line endings (CRLF).
func ToWindowsLineEndings(content string) (string, bool) {
	if !strings.Contains(content, "\r\n") {
		return strings.ReplaceAll(content, "\n", "\r\n"), true
	}
	return content, false
}
