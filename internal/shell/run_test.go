package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRun_Echo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: "echo hi",
		Cwd:     t.TempDir(),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v (stderr=%q)", err, stderr.String())
	}
	if got := stdout.String(); got != "hi\n" {
		t.Fatalf("stdout = %q, want %q", got, "hi\n")
	}
}

func TestRun_ExitCode(t *testing.T) {
	err := Run(t.Context(), RunOptions{
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

func TestRun_Stdin(t *testing.T) {
	// Use the `read` shell builtin so the test doesn't depend on any
	// external binary being on PATH (we pass an empty Env here).
	var stdout bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: "read line; echo got:$line",
		Cwd:     t.TempDir(),
		Stdin:   strings.NewReader("hello\n"),
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); got != "got:hello\n" {
		t.Fatalf("stdout = %q, want %q", got, "got:hello\n")
	}
}

func TestRun_Env(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: `echo "$FOO"`,
		Cwd:     t.TempDir(),
		Env:     []string{"FOO=bar"},
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); got != "bar\n" {
		t.Fatalf("stdout = %q, want %q", got, "bar\n")
	}
}

func TestRun_Cwd(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: "pwd",
		Cwd:     dir,
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	// mvdan's pwd builtin resolves symlinks (e.g. /var -> /private/var on
	// macOS). Compare against a suffix so we don't get bitten by that.
	got := strings.TrimRight(stdout.String(), "\n")
	if !strings.HasSuffix(got, dir) && !strings.HasSuffix(dir, got) {
		t.Fatalf("pwd = %q, want it to match %q", got, dir)
	}
}

func TestRun_JqBuiltin(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: `echo '{"a":1}' | jq .a`,
		Cwd:     t.TempDir(),
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); got != "1\n" {
		t.Fatalf("stdout = %q, want %q", got, "1\n")
	}
}

func TestRun_ParallelIsolation(t *testing.T) {
	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make([]error, n)
	outs := make([]string, n)
	dirs := make([]string, n)
	for i := range n {
		dirs[i] = t.TempDir()
		go func(i int) {
			defer wg.Done()
			var stdout bytes.Buffer
			errs[i] = Run(t.Context(), RunOptions{
				Command: `echo "$MARKER"`,
				Cwd:     dirs[i],
				Env:     []string{fmt.Sprintf("MARKER=id-%d", i)},
				Stdout:  &stdout,
			})
			outs[i] = stdout.String()
		}(i)
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil {
			t.Errorf("goroutine %d: err = %v", i, errs[i])
			continue
		}
		want := fmt.Sprintf("id-%d\n", i)
		if outs[i] != want {
			t.Errorf("goroutine %d: stdout = %q, want %q", i, outs[i], want)
		}
	}
}

// TestRun_CtxCancel_BusyLoop verifies that a pure-shell loop respects ctx
// cancellation. mvdan's interpreter checks ctx between statements, so this
// should return quickly even without any external command. The test bounds
// its own wait via a select so a regression can't hang CI.
func TestRun_CtxCancel_BusyLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	t.Cleanup(cancel)

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, RunOptions{
			Command: "while true; do :; done",
			Cwd:     t.TempDir(),
		})
	}()

	select {
	case err := <-done:
		if !IsInterrupt(err) && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected interrupt/deadline error, got: %v", err)
		}
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("Run did not return within 1.5s after ctx cancel")
	}
}

// TestRun_CtxCancel_ExternalSleep verifies ctx cancellation reaches an
// external process via mvdan's default exec. Uses sleep, which lives in
// coreutils on Windows and /bin on Unix.
func TestRun_CtxCancel_ExternalSleep(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	t.Cleanup(cancel)

	done := make(chan error, 1)
	start := time.Now()
	go func() {
		done <- Run(ctx, RunOptions{
			Command: "sleep 30",
			Cwd:     t.TempDir(),
		})
	}()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		if elapsed > time.Second {
			t.Fatalf("sleep took too long to cancel: %v", elapsed)
		}
		if err == nil {
			t.Fatal("expected non-nil error from cancelled sleep")
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return within 1s after ctx cancel")
	}
}

func TestRun_ParseError(t *testing.T) {
	err := Run(t.Context(), RunOptions{
		Command: "echo 'unterminated",
		Cwd:     t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Fatalf("error should mention parse: %v", err)
	}
}

func TestRun_BlockFuncs(t *testing.T) {
	block := CommandsBlocker([]string{"forbidden"})
	var stderr bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command:    "forbidden",
		Cwd:        t.TempDir(),
		Stderr:     &stderr,
		BlockFuncs: []BlockFunc{block},
	})
	if err == nil {
		t.Fatal("expected error when running blocked command")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected 'not allowed' error, got: %v", err)
	}
}

func TestRun_RequiresCwd(t *testing.T) {
	err := Run(t.Context(), RunOptions{
		Command: "echo hi",
	})
	if err == nil {
		t.Fatal("expected error when Cwd is empty, got nil")
	}
	if !strings.Contains(err.Error(), "Cwd is required") {
		t.Fatalf("error should mention Cwd requirement: %v", err)
	}
}

func TestRun_DiscardsNilWriters(t *testing.T) {
	// No panic when Stdout/Stderr are nil.
	err := Run(t.Context(), RunOptions{
		Command: "echo hi; echo err >&2",
		Cwd:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}
