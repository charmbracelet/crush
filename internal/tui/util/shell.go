package util

import (
	"context"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// shellCommand wraps a shell interpreter to implement tea.ExecCommand.
type shellCommand struct {
	ctx    context.Context
	file   *syntax.File
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (s *shellCommand) SetStdin(r io.Reader) {
	s.stdin = r
}

func (s *shellCommand) SetStdout(w io.Writer) {
	s.stdout = w
}

func (s *shellCommand) SetStderr(w io.Writer) {
	s.stderr = w
}

func (s *shellCommand) Run() error {
	runner, err := interp.New(
		interp.StdIO(s.stdin, s.stdout, s.stderr),
	)
	if err != nil {
		return err
	}
	return runner.Run(s.ctx, s.file)
}

// ExecShell executes a shell command string using tea.Exec.
// The command is parsed and executed via mvdan.cc/sh/v3/interp, allowing
// proper handling of shell syntax like quotes and arguments.
func ExecShell(ctx context.Context, cmdStr string, callback tea.ExecCallback) tea.Cmd {
	parsed, err := syntax.NewParser().Parse(strings.NewReader(cmdStr), "")
	if err != nil {
		return ReportError(err)
	}

	cmd := &shellCommand{
		ctx:  ctx,
		file: parsed,
	}
	return tea.Exec(cmd, callback)
}
