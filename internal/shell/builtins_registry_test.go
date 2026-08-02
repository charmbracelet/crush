package shell

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestRegisterBuiltin_DispatchedThroughRun verifies that a handler registered
// via RegisterBuiltin is dispatched through the stateless Run path, receiving
// the full args slice and the I/O streams from the interpreter context.
func TestRegisterBuiltin_DispatchedThroughRun(t *testing.T) {
	RegisterBuiltin("crushtest-echo", func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		io.WriteString(stdout, "builtin:"+args[1])
		return nil
	})
	t.Cleanup(func() { delete(builtins, "crushtest-echo") })

	var stdout bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: "crushtest-echo hello",
		Cwd:     t.TempDir(),
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); got != "builtin:hello" {
		t.Fatalf("stdout = %q, want %q", got, "builtin:hello")
	}
}

// TestRegisterBuiltin_OverridesPATHBinary verifies that a registered builtin
// takes precedence over an executable of the same name found on PATH. This is
// the stated design intent of builtinHandler sitting ahead of the exec layer.
func TestRegisterBuiltin_OverridesPATHBinary(t *testing.T) {
	dir := t.TempDir()
	// Put a fake "crushtest-shadowed" binary on PATH that would print
	// "from-path" if it ran.
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(binDir, "crushtest-shadowed")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho from-path\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	RegisterBuiltin("crushtest-shadowed", func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		io.WriteString(stdout, "from-builtin")
		return nil
	})
	t.Cleanup(func() { delete(builtins, "crushtest-shadowed") })

	var stdout bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: "crushtest-shadowed",
		Cwd:     dir,
		Env:     os.Environ(),
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); got != "from-builtin" {
		t.Fatalf("builtin did not shadow PATH binary: stdout = %q, want %q", got, "from-builtin")
	}
}

// TestBuiltinHandler_UnknownFallsThrough verifies that an unregistered command
// name is passed to the underlying exec layer rather than being swallowed.
func TestBuiltinHandler_UnknownFallsThrough(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: "echo fallthrough",
		Cwd:     t.TempDir(),
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); got != "fallthrough\n" {
		t.Fatalf("stdout = %q, want %q", got, "fallthrough\n")
	}
}

// TestRegisterBuiltin_JQRegistered verifies that jq is registered through the
// same registry as every other builtin (no special-cased dispatch path).
func TestRegisterBuiltin_JQRegistered(t *testing.T) {
	if _, ok := builtins["jq"]; !ok {
		t.Fatal("jq is not registered in the builtins map")
	}
}
