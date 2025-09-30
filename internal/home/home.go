// Package home provides utilities for dealing with the user's home directory.
package home

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Dir returns the users home directory, or if it fails, tries to create a new
// temporary directory and use that instead.
var Dir = sync.OnceValue(func() string {
	home, err := os.UserHomeDir()
	if err == nil {
		slog.Debug("user home directory", "home", home)
		return home
	}
	tmp, err := os.MkdirTemp("crush", "")
	if err != nil {
		slog.Error("could not find the user home directory")
		return ""
	}
	slog.Warn("could not find the user home directory, using a temporary one", "home", tmp)
	return tmp
})

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
