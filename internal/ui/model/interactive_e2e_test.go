package model

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/interactive"
	"github.com/stretchr/testify/require"
)

// execTestModel mirrors the production UI's handling of
// interactiveRequestMsg (tea.Exec terminal handoff) for a minimal
// program.
type execTestModel struct {
	handler func(tea.Msg) tea.Cmd
}

func (m execTestModel) Init() tea.Cmd { return nil }

func (m execTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case interactiveRequestMsg:
		req := msg.(interactiveRequestMsg)
		cmd := newInteractiveCommand(req.req, req.done)
		return m, tea.Exec(cmd, func(err error) tea.Msg {
			if err != nil {
				cmd.deliver(interactive.Result{
					Output:   "Failed to hand the terminal to the command: " + err.Error(),
					ExitCode: 1,
				})
			}
			return nil
		})
	}
	return m, nil
}

func (m execTestModel) View() tea.View { return tea.NewView("") }

// TestInteractiveHandler_EndToEnd drives a real tea.Program over pipes and
// verifies the full handoff chain: interactive.Run -> program.Send ->
// Update -> tea.Exec -> shell.RunInteractive -> done.
func TestInteractiveHandler_EndToEnd(t *testing.T) {
	stdinR, stdinW, err := os.Pipe()
	require.NoError(t, err)
	defer stdinR.Close()
	defer stdinW.Close()
	stdoutR, stdoutW, err := os.Pipe()
	require.NoError(t, err)
	defer stdoutR.Close()
	defer stdoutW.Close()

	program := tea.NewProgram(
		execTestModel{},
		tea.WithInput(stdinR),
		tea.WithOutput(stdoutW),
	)
	interactive.SetHandler(NewInteractiveHandler(program))
	t.Cleanup(func() { interactive.SetHandler(nil) })

	go func() {
		_, _ = program.Run()
	}()

	resCh := make(chan interactive.Result, 1)
	go func() {
		res, err := interactive.Run(context.Background(), interactive.Request{
			Command:    "read line; echo got:$line",
			WorkingDir: t.TempDir(),
		})
		require.NoError(t, err)
		resCh <- res
	}()

	// Feed the command's stdin once the terminal has been handed over.
	// The interactive command reads from the program's input stream.
	go func() {
		// The command runs async; write is buffered by the pipe until read.
		_, _ = stdinW.Write([]byte("secret\n"))
	}()

	select {
	case res := <-resCh:
		require.Equal(t, 0, res.ExitCode)
		require.Contains(t, res.Output, "got:secret")
	case <-time.After(10 * time.Second):
		t.Fatal("interactive run never completed: the handoff chain is stuck")
	}

	program.Quit()
	// Close our write end so ReadAll can reach EOF once the program exits
	// and releases its copy.
	require.NoError(t, stdoutW.Close())
	_, _ = io.ReadAll(stdoutR)
}
