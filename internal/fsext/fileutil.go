package fsext

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/charlievieth/fastwalk"
	"github.com/charmbracelet/crush/internal/log"

	ignore "github.com/sabhiram/go-gitignore"
)

var rgPath string

func init() {
	var err error
	rgPath, err = exec.LookPath("rg")
	if err != nil {
		if log.Initialized() {
			slog.Warn("Ripgrep (rg) not found in $PATH. Some grep features might be limited or slower.")
		}
	}
}

func GetRgCmd(globPattern string) *exec.Cmd {
	if rgPath == "" {
		return nil
	}
	rgArgs := []string{
		"--files",
		"-L",
		"--null",
	}
	if globPattern != "" {
		if !filepath.IsAbs(globPattern) && !strings.HasPrefix(globPattern, "/") {
			globPattern = "/" + globPattern
		}
		rgArgs = append(rgArgs, "--glob", globPattern)
	}
	return exec.Command(rgPath, rgArgs...)
}

func GetRgSearchCmd(pattern, path, include string) *exec.Cmd {
	if rgPath == "" {
		return nil
	}
	// Use -n to show line numbers and include the matched line
	args := []string{"-H", "-n", pattern}
	if include != "" {
		args = append(args, "--glob", include)
	}
	args = append(args, path)

	return exec.Command(rgPath, args...)
}

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
		"tmp":              true,
		"temp":             true,
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
	gitignore *ignore.GitIgnore
	rootPath  string
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

	return walker
}

func (w *FastGlobWalker) shouldSkip(path string) bool {
	if SkipHidden(path) {
		return true
	}

	if w.gitignore != nil {
		relPath, err := filepath.Rel(w.rootPath, path)
		if err == nil && w.gitignore.MatchesPath(relPath) {
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
			if walker.shouldSkip(path) {
				return filepath.SkipDir
			}
			return nil
		}

		if walker.shouldSkip(path) {
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
	// replace home directory with ~
	homeDir, err := os.UserHomeDir()
	if err == nil {
		path = strings.ReplaceAll(path, homeDir, "~")
	}
	return path
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
