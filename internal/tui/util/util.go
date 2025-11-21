package util

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

type Cursor interface {
	Cursor() *tea.Cursor
}

type Model interface {
	Init() tea.Cmd
	Update(tea.Msg) (Model, tea.Cmd)
	View() string
}

func CmdHandler(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

func ReportError(err error) tea.Cmd {
	slog.Error("Error reported", "error", err)
	return CmdHandler(InfoMsg{
		Type: InfoTypeError,
		Msg:  err.Error(),
	})
}

type InfoType int

const (
	InfoTypeInfo InfoType = iota
	InfoTypeSuccess
	InfoTypeWarn
	InfoTypeError
)

func ReportInfo(info string) tea.Cmd {
	return CmdHandler(InfoMsg{
		Type: InfoTypeInfo,
		Msg:  info,
	})
}

func ReportWarn(warn string) tea.Cmd {
	return CmdHandler(InfoMsg{
		Type: InfoTypeWarn,
		Msg:  warn,
	})
}

type (
	InfoMsg struct {
		Type InfoType
		Msg  string
		TTL  time.Duration
	}
	ClearStatusMsg struct{}
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
