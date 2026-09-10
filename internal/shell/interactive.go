package shell

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// RunInteractive parses and executes a shell command with the user's
// controlling terminal attached to child processes.
//
// Unlike [Run] and the stateful [Shell], child processes are NOT
// isolated from Crush's session: they keep the controlling terminal and
// the foreground process group, so programs that need direct user
// interaction (pinentry-curses, SSH passphrase prompts, full-screen
// TUIs, editors) behave as if the user ran the command themselves. In
// exchange, a misbehaving child can send signals to Crush or scribble
// over the terminal; the caller is responsible for having released
// the terminal first (in the TUI this happens via tea.Exec, which
// pauses the UI for the duration of the run).
//
// The non-interactive environment overrides from [Run] (EDITOR=false,
// PAGER=cat, etc.) are NOT applied: interactive commands should get the
// user's real environment so things like `git commit` can open the
// user's editor. Pass os.Environ() as Env unless a custom environment
// is needed.
//
// Errors follow the same conventions as [Run]: inspect with
// [IsInterrupt] and [ExitCode].
func RunInteractive(ctx context.Context, opts RunOptions) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("command execution panic: %v", r)
		}
	}()

	if opts.Cwd == "" {
		return fmt.Errorf("shell.RunInteractive: Cwd is required")
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	stdin := opts.Stdin
	if stdin == nil {
		stdin = strings.NewReader("")
	}

	line, err := syntax.NewParser().Parse(strings.NewReader(opts.Command), "")
	if err != nil {
		return fmt.Errorf("could not parse command: %w", err)
	}

	runner, err := newInteractiveRunner(opts.Cwd, opts.Env, stdin, stdout, stderr, opts.BlockFuncs)
	if err != nil {
		return fmt.Errorf("could not run command: %w", err)
	}

	return runner.Run(ctx, line)
}

// newInteractiveRunner is [newRunner] without terminal isolation and
// without the forced non-interactive environment: children must be
// able to reach the user's terminal.
func newInteractiveRunner(cwd string, env []string, stdin io.Reader, stdout, stderr io.Writer, blockFuncs []BlockFunc) (*interp.Runner, error) {
	return interp.New(
		interp.StdIO(stdin, stdout, stderr),
		interp.Interactive(false),
		interp.Env(expand.ListEnviron(env...)),
		interp.Dir(cwd),
		interactiveExecHandlerOption(blockFuncs),
	)
}

// interactiveExecHandlerOption is [execHandlerOption] with the
// process-group isolation removed, so children keep Crush's
// controlling terminal. The standard middleware chain (builtins, script
// dispatch, block list, coreutils) still applies.
func interactiveExecHandlerOption(blockFuncs []BlockFunc) interp.RunnerOption {
	handler := interactiveExecHandler(defaultKillTimeout)
	for _, mw := range slices.Backward(standardHandlers(blockFuncs)) {
		handler = mw(handler)
	}
	// ExecHandlers always appends DefaultExecHandler which lacks process
	// group isolation, so we use the deprecated ExecHandler instead.
	return interp.ExecHandler(handler)
}
