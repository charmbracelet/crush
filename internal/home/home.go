// Package home provides utilities for dealing with the user's home directory.
package home

import (
	"os"
	"path/filepath"
	"strings"
)

// Dir returns the user home directory.
func Dir() string {
	home, _ := os.UserHomeDir()
	return home
}

// Short replaces the actual home path from [Dir] with `~`.
func Short(p string) string {
	if !strings.HasPrefix(p, Dir()) || Dir() == "" {
		return p
	}
	return filepath.Join("~", strings.TrimPrefix(p, Dir()))
}

// Long replaces the `~` with actual home path from [Dir].
func Long(p string) string {
	if !strings.HasPrefix(p, "~") || Dir() == "" {
		return p
	}
	return strings.Replace(p, "~", Dir(), 1)
}
