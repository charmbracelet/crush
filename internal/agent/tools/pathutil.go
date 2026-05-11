package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// clampToWorkingDir ensures the search path is within the working directory.
// This prevents tools from accidentally searching the entire filesystem.
func clampToWorkingDir(workingDir, searchPath string) (string, error) {
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return "", fmt.Errorf("error resolving working directory: %w", err)
	}

	absSearchPath, err := filepath.Abs(searchPath)
	if err != nil {
		return "", fmt.Errorf("error resolving search path: %w", err)
	}

	rel, err := filepath.Rel(absWorkingDir, absSearchPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf(
			"search path %q is outside the working directory %q, please use a path within the project",
			searchPath, workingDir,
		)
	}

	return absSearchPath, nil
}
