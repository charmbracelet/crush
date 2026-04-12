package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
)

// DirRestrictions holds configuration for directory access restrictions.
type DirRestrictions struct {
	WorkingDir        string
	AdditionalDirs    []string
	RestrictToProject bool
}

// isOutsideAllowedDirs checks whether the given absolute path is outside the
// working directory and all additional directories.
func (r DirRestrictions) isOutsideAllowedDirs(absPath string) bool {
	absWorkingDir, err := filepath.Abs(r.WorkingDir)
	if err != nil {
		return true
	}

	relPath, err := filepath.Rel(absWorkingDir, absPath)
	if err == nil && !strings.HasPrefix(relPath, "..") {
		return false
	}

	return !isInAdditionalDir(absPath, r.AdditionalDirs)
}

// DenyIfRestricted returns a text error response if the path is outside allowed
// directories and restrict_to_project is enabled. Returns nil if access is allowed.
func (r DirRestrictions) DenyIfRestricted(absPath, toolName string) *fantasy.ToolResponse {
	if !r.RestrictToProject {
		return nil
	}

	if r.isOutsideAllowedDirs(absPath) {
		resp := fantasy.NewTextErrorResponse(
			fmt.Sprintf("Access denied: %s is outside the project directory. "+
				"The restrict_to_project setting prevents %s from accessing paths outside the working directory and additional_dirs.",
				absPath, toolName))
		return &resp
	}

	return nil
}
