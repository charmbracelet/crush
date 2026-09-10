package shell

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunInteractive_EchoAndStdin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := RunInteractive(t.Context(), RunOptions{
		Command: "read line; echo got:$line",
		Cwd:     t.TempDir(),
		Stdin:   strings.NewReader("hello\n"),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err != nil {
		t.Fatalf("RunInteractive returned error: %v (stderr=%q)", err, stderr.String())
	}
	if got := stdout.String(); got != "got:hello\n" {
		t.Fatalf("stdout = %q, want %q", got, "got:hello\n")
	}
}

func TestRunInteractive_ExitCode(t *testing.T) {
	err := RunInteractive(t.Context(), RunOptions{
		Command: "exit 7",
		Cwd:     t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error for exit 7, got nil")
	}
	if code := ExitCode(err); code != 7 {
		t.Fatalf("ExitCode = %d, want 7", code)
	}
}

func TestRunInteractive_PreservesCallerEnv(t *testing.T) {
	// Interactive runs must not force the non-interactive overrides
	// (EDITOR=false, PAGER=cat, ...): the command may need the user's
	// real environment to open an editor or pager.
	var stdout bytes.Buffer
	err := RunInteractive(t.Context(), RunOptions{
		Command: `echo "$EDITOR|$PAGER"`,
		Cwd:     t.TempDir(),
		Env:     []string{"EDITOR=vim", "PAGER=less"},
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatalf("RunInteractive returned error: %v", err)
	}
	if got := stdout.String(); got != "vim|less\n" {
		t.Fatalf("stdout = %q, want %q", got, "vim|less\n")
	}
}

func TestRunInteractive_KeepsGPGTTY(t *testing.T) {
	// The interactive path must keep GPG_TTY so gpg can tell the agent
	// which terminal to draw pinentry on once the terminal is handed
	// over. Non-interactive runs scrub it (see TestRun_ScrubsGPGTTY).
	var stdout bytes.Buffer
	err := RunInteractive(t.Context(), RunOptions{
		Command: `echo "${GPG_TTY:-unset}"`,
		Cwd:     t.TempDir(),
		Env:     []string{"GPG_TTY=/dev/ttys999"},
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatalf("RunInteractive returned error: %v", err)
	}
	if got := stdout.String(); got != "/dev/ttys999\n" {
		t.Fatalf("stdout = %q, want %q", got, "/dev/ttys999\n")
	}
}

func TestRun_ScrubsGPGTTY(t *testing.T) {
	// Non-interactive runs must scrub GPG_TTY: without a controlling
	// terminal, a daemon-spawned pinentry would scribble over Crush's
	// TUI and hang waiting for input that never arrives. Scrubbing makes
	// gpg fail fast instead.
	var stdout bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: `echo "${GPG_TTY:-unset}"`,
		Cwd:     t.TempDir(),
		Env:     []string{"GPG_TTY=/dev/ttys999"},
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); got != "unset\n" {
		t.Fatalf("stdout = %q, want %q", got, "unset\n")
	}
}

func TestRunInteractive_EnforcesBlockFuncs(t *testing.T) {
	err := RunInteractive(t.Context(), RunOptions{
		Command: "definitely-forbidden-command",
		Cwd:     t.TempDir(),
		BlockFuncs: []BlockFunc{
			func(args []string) bool {
				return len(args) > 0 && args[0] == "definitely-forbidden-command"
			},
		},
	})
	if err == nil {
		t.Fatal("expected blocked command to fail")
	}
	if code := ExitCode(err); code != 1 {
		t.Fatalf("ExitCode = %d, want 1", code)
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("err = %v, want it to mention the block", err)
	}
}

func TestRunInteractive_ParseError(t *testing.T) {
	err := RunInteractive(t.Context(), RunOptions{
		Command: "echo 'unterminated",
		Cwd:     t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "could not parse command") {
		t.Fatalf("err = %v, want a parse error", err)
	}
}

func TestRunInteractive_ContextCancellationKillsChild(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	t.Cleanup(cancel)

	err := RunInteractive(ctx, RunOptions{
		Command: "sleep 30",
		Cwd:     t.TempDir(),
		Env:     os.Environ(),
	})
	if !IsInterrupt(err) {
		t.Fatalf("err = %v, want an interrupt error", err)
	}
}
