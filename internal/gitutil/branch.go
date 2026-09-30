// Package gitutil provides utility functions for interacting with Git
// repositories.
package gitutil

import (
	"os"
	"path/filepath"
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

// findGitDir walks up from dir looking for a .git directory. It also resolves
// "gitdir:" pointers, which .git files contain in worktrees and submodules.
// Returns an empty string when dir is not inside a Git repository.
func findGitDir(dir string) string {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}

	for {
		gitPath := filepath.Join(dir, ".git")
		info, statErr := os.Stat(gitPath)
		if statErr == nil {
			if info.IsDir() {
				return gitPath
			}
			content, readErr := os.ReadFile(gitPath)
			if readErr != nil {
				return ""
			}
			gitDir, ok := strings.CutPrefix(strings.TrimSpace(string(content)), "gitdir:")
			if !ok {
				return ""
			}
			gitDir = strings.TrimSpace(gitDir)
			if !filepath.IsAbs(gitDir) {
				gitDir = filepath.Join(dir, gitDir)
			}
			return gitDir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
