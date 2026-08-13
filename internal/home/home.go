// Package home provides utilities for dealing with the user's home directory.
package home

import (
	"cmp"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var homedir, homedirErr = os.UserHomeDir()

func init() {
	if homedirErr != nil {
		slog.Error("Failed to get user home directory", "error", homedirErr)
	}
}

// Dir returns the user home directory.
func Dir() string {
	return homedir
}

// Config returns the user config directory.
func Config() string {
	return cmp.Or(
		os.Getenv("XDG_CONFIG_HOME"),
		filepath.Join(Dir(), ".config"),
	)
}

// Short replaces the actual home path from [Dir] with `~`.
func Short(p string) string {
	if homedir == "" || !strings.HasPrefix(p, homedir) {
		return p
	}
	if len(p) == len(homedir) {
		return "~"
	}
	if p[len(homedir)] != filepath.Separator {
		return p
	}
	return filepath.Join("~", strings.TrimPrefix(p, homedir))
}

// Long replaces the `~` with actual home path from [Dir].
func Long(p string) string {
	if homedir == "" || !strings.HasPrefix(p, "~") {
		return p
	}
	if len(p) == 1 {
		return homedir
	}
	if p[1] != filepath.Separator {
		return p
	}
	return strings.Replace(p, "~", homedir, 1)
}
