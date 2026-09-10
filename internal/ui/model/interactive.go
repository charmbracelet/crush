package model

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	agenttools "github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/interactive"
	"github.com/charmbracelet/crush/internal/shell"
)

// interactiveRequestMsg asks the TUI to hand the user's terminal to a
// command that needs direct interaction (password prompts, TUIs like
// pinentry-curses). It is produced by the handler returned from
// [NewInteractiveHandler], which blocks on the done channel until the
// command exits.
type interactiveRequestMsg struct {
	req  interactive.Request
	done chan interactive.Result
}

// NewInteractiveHandler returns an [interactive.Handler] that pauses
// the TUI and hands the user's terminal to the requested command. The
// TUI releases the terminal, the command runs attached to it (so the
// user can type passwords or use its UI), and when it exits the TUI
// restores the terminal and repaints.
func NewInteractiveHandler(program *tea.Program) interactive.Handler {
	return func(ctx context.Context, req interactive.Request) (interactive.Result, error) {
		done := make(chan interactive.Result, 1)
		program.Send(interactiveRequestMsg{req: req, done: done})
		select {
		case res := <-done:
			return res, nil
		case <-ctx.Done():
			return interactive.Result{}, ctx.Err()
		}
	}
}

// interactiveCommand adapts a terminal-handed-over command to
// tea.ExecCommand. Bubble Tea calls the SetStd* methods with the real
// terminal streams after releasing the terminal; Run executes the
// command through the shell interpreter and delivers the outcome to
// the waiting tool goroutine.
type interactiveCommand struct {
	req    interactive.Request
	done   chan interactive.Result
	once   sync.Once
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

// newInteractiveCommand builds the tea.ExecCommand for a request.
func newInteractiveCommand(req interactive.Request, done chan interactive.Result) *interactiveCommand {
	return &interactiveCommand{req: req, done: done}
}

func (c *interactiveCommand) SetStdin(r io.Reader) {
	if r != nil {
		c.stdin = r
	}
}

func (c *interactiveCommand) SetStdout(w io.Writer) {
	if w != nil {
		c.stdout = w
	}
}

func (c *interactiveCommand) SetStderr(w io.Writer) {
	if w != nil {
		c.stderr = w
	}
}

// deliver sends the result to the waiting tool goroutine at most once.
func (c *interactiveCommand) deliver(res interactive.Result) {
	c.once.Do(func() {
		c.done <- res
	})
}

// Run executes the command and reports its outcome. It never returns a
// transport error to Bubble Tea: exit codes are part of the result, and
// the waiting tool goroutine must always be unblocked.
func (c *interactiveCommand) Run() error {
	c.deliver(c.execute())
	return nil
}

// execute runs the command through the shell interpreter with the
// terminal streams attached. Everything the command prints also flows
// to a capped buffer so the tool result can carry a copy of it.
func (c *interactiveCommand) execute() interactive.Result {
	// Make the handoff visible: the TUI is paused with its last frame
	// still on screen, so clear the screen and say what is about to run
	// before the command takes over. This goes straight to the terminal
	// and is not part of the captured output.
	if c.stdout != nil {
		command := strings.ReplaceAll(c.req.Command, "\n", "; ")
		fmt.Fprintf(c.stdout, "%s\x1b[H\n Crush is handing the terminal over to:\n $ %s\n\n", ansi.EraseEntireScreen, command)
	}

	capture := &cappedWriter{limit: agenttools.MaxOutputLength}
	stdout := io.Discard
	if c.stdout != nil {
		stdout = c.stdout
	}
	stderr := io.Discard
	if c.stderr != nil {
		stderr = c.stderr
	}

	err := shell.RunInteractive(context.Background(), shell.RunOptions{
		Command:    c.req.Command,
		Cwd:        c.req.WorkingDir,
		Env:        os.Environ(),
		Stdin:      c.stdin,
		Stdout:     io.MultiWriter(stdout, capture),
		Stderr:     io.MultiWriter(stderr, capture),
		BlockFuncs: agenttools.BlockedCommandFuncs(),
	})

	output := strings.TrimSpace(ansi.Strip(capture.String()))
	exitCode := shell.ExitCode(err)
	if shell.IsInterrupt(err) {
		output += "\nCommand was aborted before completion"
	} else if err != nil && !shell.IsExitStatus(err) {
		// Execution-level failure (block-list rejection, parse error,
		// panic): surface the message alongside any output captured so
		// far.
		if output != "" {
			output += "\n\n"
		}
		output += err.Error()
		exitCode = 1
	}

	return interactive.Result{
		Output:   agenttools.TruncateOutput(output),
		ExitCode: exitCode,
	}
}

// cappedWriter accumulates at most limit bytes, then discards the rest.
// Interactive TUIs can emit megabytes of escape sequences; only a
// bounded sample is kept for the tool result.
type cappedWriter struct {
	buf   bytes.Buffer
	limit int
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if remaining := w.limit - w.buf.Len(); remaining > 0 {
		w.buf.Write(p[:min(remaining, len(p))])
	}
	return len(p), nil
}

func (w *cappedWriter) String() string {
	return w.buf.String()
}
