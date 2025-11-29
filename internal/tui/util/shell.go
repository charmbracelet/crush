package util

import (
	"context"
	"errors"
	"os/exec"

	tea "charm.land/bubbletea/v2"
	"mvdan.cc/sh/v3/shell"
)

// ExecShell parses a shell command string and executes it with exec.Command.
// Uses shell.Fields for proper handling of shell syntax like quotes and
// arguments while preserving TTY handling for terminal editors.
func ExecShell(ctx context.Context, cmdStr string, callback tea.ExecCallback) tea.Cmd {
	fields, err := shell.Fields(cmdStr, nil)
	if err != nil {
		return ReportError(err)
	}
	if len(fields) == 0 {
		return ReportError(errors.New("empty command"))
	}

	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
	return tea.ExecProcess(cmd, callback)
}
