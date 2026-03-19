// Package gitutil provides utility functions for interacting with Git repositories.
package gitutil

import (
	"context"
	"os/exec"
	"strings"
)

// GetCurrentBranch returns the current Git branch name for the given directory.
// It uses the git CLI command to get the branch name.
// Returns an empty string if:
// - The directory is not in a Git repository
// - The repository is in a detached HEAD state
// - Git is not installed or any error occurs
func GetCurrentBranch(ctx context.Context, dir string) string {
	if !isInsideWorkTree(ctx, dir) {
		return ""
	}

	cmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// isInsideWorkTree checks if the given directory is inside a git work tree.
func isInsideWorkTree(ctx context.Context, dir string) bool {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) == "true"
}
