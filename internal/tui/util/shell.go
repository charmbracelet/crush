package util

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/uiutil"
)

// ExecShell parses a shell command string and executes it with exec.Command.
// Uses shell.Fields for proper handling of shell syntax like quotes and
// arguments while preserving TTY handling for terminal editors.
func ExecShell(ctx context.Context, cmdStr string, callback tea.ExecCallback) tea.Cmd {
	return uiutil.ExecShell(ctx, cmdStr, callback)
}
