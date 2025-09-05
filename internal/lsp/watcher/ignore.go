package watcher

import (
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

// shouldIgnoreDirectory checks if a directory should be ignored based on hierarchical
// .gitignore/.crushignore files. It checks for ignore files in each directory from
// the target directory up to the workspace root.
func shouldIgnoreDirectory(workspaceRoot, dirPath string) bool {
	if workspaceRoot == "" {
		return false
	}

	// Always ignore .git directories
	if filepath.Base(dirPath) == ".git" {
		return true
	}

	if strings.HasPrefix(dirPath, workspaceRoot) {
		return isIgnoredInWorkspace(dirPath, workspaceRoot)
	}

	return false
}

// isIgnoredInWorkspace checks if a path is ignored by checking .gitignore/.crushignore
// files in each directory from workspace root to the target path's parent.
func isIgnoredInWorkspace(targetPath, workspaceRoot string) bool {
	// Get relative path from workspace root
	relPath, err := filepath.Rel(workspaceRoot, targetPath)
	if err != nil {
		return false
	}

	// Don't ignore the workspace root itself
	if relPath == "." {
		return false
	}

	// Check ignore files in each directory from workspace root to target
	pathParts := strings.Split(relPath, string(filepath.Separator))

	for i := range pathParts {
		// Build the directory path to check for ignore files
		checkDir := workspaceRoot
		if i > 0 {
			checkDir = filepath.Join(workspaceRoot, filepath.Join(pathParts[:i]...))
		}

		// Build the relative path to test against ignore patterns
		testPath := strings.Join(pathParts[:i+1], "/")

		// Check .gitignore in this directory
		if checkIgnoreFile(filepath.Join(checkDir, ".gitignore"), testPath) {
			return true
		}

		// Check .crushignore in this directory
		if checkIgnoreFile(filepath.Join(checkDir, ".crushignore"), testPath) {
			return true
		}
	}

	return false
}

// checkIgnoreFile checks if a path matches patterns in an ignore file
func checkIgnoreFile(ignoreFilePath, relPath string) bool {
	content, err := os.ReadFile(ignoreFilePath)
	if err != nil {
		return false // File doesn't exist or can't be read
	}

	lines := strings.Split(string(content), "\n")
	ignoreParser := ignore.CompileIgnoreLines(lines...)

	// Check both with and without trailing slash for directories
	if ignoreParser.MatchesPath(relPath) {
		return true
	}

	// For directories, also check with trailing slash
	if ignoreParser.MatchesPath(relPath + "/") {
		return true
	}

	return false
}
