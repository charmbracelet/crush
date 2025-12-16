// Package gitutil provides utility functions for interacting with Git repositories.
package gitutil

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	gitDir    = ".git"
	headFile  = "HEAD"
	refPrefix = "ref: refs/heads/"
)

// GetCurrentBranch returns the current Git branch name for the given directory.
// It walks up the directory tree to find the .git directory.
// Returns an empty string if:
// - The directory is not in a Git repository
// - The repository is in a detached HEAD state
// - Any error occurs reading the Git files
func GetCurrentBranch(dir string) string {
	gitPath := findGitDir(dir)
	if gitPath == "" {
		return ""
	}

	headPath := filepath.Join(gitPath, headFile)
	content, err := os.ReadFile(headPath)
	if err != nil {
		return ""
	}

	return parseHeadRef(strings.TrimSpace(string(content)))
}

// findGitDir walks up the directory tree to find the .git directory.
// Returns the path to the .git directory, or empty string if not found.
func findGitDir(dir string) string {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}

	for {
		gitPath := filepath.Join(absDir, gitDir)
		info, err := os.Stat(gitPath)
		if err == nil && info.IsDir() {
			return gitPath
		}

		parent := filepath.Dir(absDir)
		if parent == absDir {
			// Reached root without finding .git
			return ""
		}
		absDir = parent
	}
}

// parseHeadRef extracts the branch name from the HEAD file content.
// The HEAD file typically contains either:
// - "ref: refs/heads/<branch-name>" for a normal branch checkout
// - A commit SHA for a detached HEAD state
func parseHeadRef(content string) string {
	if strings.HasPrefix(content, refPrefix) {
		return strings.TrimPrefix(content, refPrefix)
	}
	// Detached HEAD or unexpected format
	return ""
}
