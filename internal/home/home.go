// Package home provides utilities for dealing with the user's home directory.
package home

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var homedir, homedirErr = os.UserHomeDir()

func init() {
	if homedirErr != nil {
		slog.Error("failed to get user home directory", "error", homedirErr)
	}
}

// Dir returns the user home directory.
func Dir() string {
	return homedir
}

// Short replaces the actual home path from [Dir] with `~`.
func Short(p string) string {
	if homedir == "" || !strings.HasPrefix(p, homedir) {
		return p
	}
	return filepath.Join("~", strings.TrimPrefix(p, homedir))
}

// Long replaces the `~` with actual home path from [Dir].
func Long(p string) string {
	if homedir == "" || !strings.HasPrefix(p, "~") {
		return p
	}
	return strings.Replace(p, "~", homedir, 1)
}
