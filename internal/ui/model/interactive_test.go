package model

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/interactive"
	"github.com/stretchr/testify/require"
)

func TestInteractiveCommand_Execute(t *testing.T) {
	var stdout bytes.Buffer
	done := make(chan interactive.Result, 1)
	cmd := newInteractiveCommand(interactive.Request{
		Command:    "read line; echo got:$line",
		WorkingDir: t.TempDir(),
	}, done)
	cmd.SetStdin(strings.NewReader("hello\n"))
	cmd.SetStdout(&stdout)

	require.NoError(t, cmd.Run())

	res := <-done
	require.Equal(t, 0, res.ExitCode)
	require.Equal(t, "got:hello", res.Output, "captured output must be stripped of ANSI and trimmed")
	require.Contains(t, stdout.String(), "got:hello\n", "the command output must also reach the terminal writer")
	require.Contains(t, stdout.String(), "handing the terminal over", "the handoff must be made visible on the terminal")
	require.NotContains(t, res.Output, "handing the terminal over", "the banner is not part of the captured output")
}

func TestInteractiveCommand_ExecuteExitCode(t *testing.T) {
	done := make(chan interactive.Result, 1)
	cmd := newInteractiveCommand(interactive.Request{
		Command:    "exit 3",
		WorkingDir: t.TempDir(),
	}, done)

	require.NoError(t, cmd.Run(), "Run must not report transport errors to Bubble Tea")
	res := <-done
	require.Equal(t, 3, res.ExitCode)
}

func TestInteractiveCommand_ExecuteBlockedCommand(t *testing.T) {
	done := make(chan interactive.Result, 1)
	cmd := newInteractiveCommand(interactive.Request{
		Command:    "wget http://example.com",
		WorkingDir: t.TempDir(),
	}, done)

	require.NoError(t, cmd.Run())
	res := <-done
	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Output, "not allowed")
}

func TestInteractiveCommand_DeliversOnce(t *testing.T) {
	done := make(chan interactive.Result, 1)
	cmd := newInteractiveCommand(interactive.Request{
		Command:    "true",
		WorkingDir: t.TempDir(),
	}, done)

	cmd.deliver(interactive.Result{ExitCode: 0})
	cmd.deliver(interactive.Result{ExitCode: 9})

	require.Len(t, done, 1, "only the first result may be delivered")
	res := <-done
	require.Zero(t, res.ExitCode)
}
