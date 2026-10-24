// Package gitutil provides utility functions for interacting with Git
// repositories.
package gitutil

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// CurrentBranch returns the current Git branch name for the given directory.
// Returns an empty string if the directory is not in a Git repository, the
// repository is in a detached HEAD state, or any error occurs.
//
// This is a directory walk and one small file read, so it is not cached.
// Callers that would otherwise ask once per rendered frame poll it instead
// and render from their own state.
func CurrentBranch(dir string) string {
	return readBranch(dir)
}

// readBranch reads the current branch name straight from the repository's
// HEAD file. Going through the filesystem is both cheaper and more tolerant
// than opening the repository with a Git library, which can reject repos with
// unusual configuration.
func readBranch(dir string) string {
	gitDir := findGitDir(dir)
	if gitDir == "" {
		return ""
	}

	head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}

	// HEAD is either a symbolic ref ("ref: refs/heads/<branch>") or, when
	// detached, a raw commit hash. Only the former names a branch.
	name, ok := strings.CutPrefix(strings.TrimSpace(string(head)), "ref: refs/heads/")
	if !ok {
		return ""
	}
	return name
}

// RepoRoot walks up from dir looking for a .git entry and returns the
// directory that contains it, or the empty string when there is none.
//
// The directory is not trusted: in server mode it is a workspace path a
// client sent over the API. A path that climbs with ".." is refused rather
// than resolved, since walking from it would inspect directories other than
// the one the caller named.
func RepoRoot(dir string) string {
	if dir == "" || hasDotDot(dir) {
		return ""
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	cur := filepath.Clean(abs)
	if !strings.HasPrefix(cur, filepath.VolumeName(cur)+string(filepath.Separator)) {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}

// hasDotDot reports whether p contains a ".." path element. Names that
// merely contain two dots, such as "a..b", are allowed. Both separators are
// checked so a Windows-style path is refused on every platform.
func hasDotDot(p string) bool {
	return slices.Contains(strings.FieldsFunc(p, func(r rune) bool {
		return r == '/' || r == '\\'
	}), "..")
}

// findGitDir returns the git directory for the repository containing dir. It
// resolves "gitdir:" pointers, which .git files contain in worktrees and
// submodules. Returns an empty string when dir is not inside a repository.
func findGitDir(dir string) string {
	root := RepoRoot(dir)
	if root == "" {
		return ""
	}

	gitPath := filepath.Join(root, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return gitPath
	}
	content, err := os.ReadFile(gitPath)
	if err != nil {
		return ""
	}
	gitDir, ok := strings.CutPrefix(strings.TrimSpace(string(content)), "gitdir:")
	if !ok {
		return ""
	}
	gitDir = strings.TrimSpace(gitDir)
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}
	return gitDir
}
