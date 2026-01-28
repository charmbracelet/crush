// Package crush provides the public entry point for building custom Crush binaries.
//
// This package exposes the Execute function that xcrush and plugins can use
// to build custom Crush binaries with compile-time extensions.
package crush

import (
	"github.com/charmbracelet/crush/internal/cmd"
)

// Execute runs the main Crush command.
// This is the entry point for custom builds with plugins.
func Execute() {
	cmd.Execute()
}
