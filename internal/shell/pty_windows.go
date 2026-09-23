//go:build windows

package shell

import (
	"os"
	"os/exec"

	"github.com/charmbracelet/x/xpty"
)

// configureInteractiveProcess is a no-op on Windows: ConPTY wires the
// console itself.
func configureInteractiveProcess(_ *exec.Cmd, _ xpty.Pty) error { return nil }

// killInteractiveProcess kills the child. ConPTY tears down the console
// with it.
func killInteractiveProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill() //nolint:wrapcheck
}

// signaledExitCode is a no-op on Windows.
func signaledExitCode(_ error) (int, bool) { return 0, false }

// platformInteractiveShell resolves the user's shell. An empty command
// starts the shell interactively.
func platformInteractiveShell(command string) (string, []string) {
	shellPath := os.Getenv("COMSPEC")
	if shellPath == "" {
		shellPath = "cmd.exe"
	}
	if command == "" {
		return shellPath, nil
	}
	return shellPath, []string{"/c", command}
}
