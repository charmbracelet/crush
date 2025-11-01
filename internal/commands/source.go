package commands

import (
	"strings"
)

// CommandSource represents the source location of a command.
type CommandSource string

const (
	// SourceProject indicates command is from project directory (.crush/commands)
	SourceProject CommandSource = "project"
	// SourceUserHome indicates command is from user home directory (~/.crush/commands)
	SourceUserHome CommandSource = "user"
	// SourceXDG indicates command is from XDG config directory (~/.config/crush/commands)
	SourceXDG CommandSource = "user"
)

// buildSourceIndicator generates a source indicator string for a command based on its location and namespace.
// Examples:
//   - Project root command: `project`
//   - Project namespaced command: `project:frontend`
//   - User root command: `user`
//   - User namespaced command: `user:frontend`
//
// Both user home and XDG config commands use `user` as the source prefix.
func buildSourceIndicator(source CommandSource, namespace string) string {
	if namespace == "" {
		return string(source)
	}
	return string(source) + ":" + namespace
}

// detectCommandSource determines the source type from a file path by comparing
// against known base directories.
func detectCommandSource(path string, projectBaseDir, userHomeBaseDir, xdgBaseDir string) CommandSource {
	path = strings.ToLower(path)
	projectBaseDir = strings.ToLower(projectBaseDir)
	userHomeBaseDir = strings.ToLower(userHomeBaseDir)
	xdgBaseDir = strings.ToLower(xdgBaseDir)

	if strings.HasPrefix(path, projectBaseDir) {
		return SourceProject
	}
	if strings.HasPrefix(path, xdgBaseDir) {
		return SourceXDG
	}
	if strings.HasPrefix(path, userHomeBaseDir) {
		return SourceUserHome
	}

	// Default to project if unclear
	return SourceProject
}

