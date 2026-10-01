//go:build !windows

package shell

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

	"github.com/charmbracelet/x/xpty"
)

// configureInteractiveProcess makes the child a session leader with the PTY
// slave as its controlling terminal. This is the deliberate opposite of
// [isolateProcess]: an interactive session owns its PTY, so the child can
// and should take it over for job control, signals, and /dev/tty access.
//
// Ctty is left at 0 because xpty wires the slave to the child's stdin, and
// the kernel needs the descriptor index rather than the parent's fd.
func configureInteractiveProcess(cmd *exec.Cmd, _ xpty.Pty) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true
	return nil
}

// killInteractiveProcess kills the whole process group, not just the shell,
// so TUIs that fork grandchildren are cleaned up too.
func killInteractiveProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	// Setsid made the child its own process group leader, so a negative
	// PID reaches every descendant. ESRCH means it already exited.
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil &&
		!errors.Is(err, syscall.ESRCH) {
		return cmd.Process.Kill() //nolint:wrapcheck
	}
	return nil
}

// signaledExitCode converts a signaled exit to the 128+signal convention.
func signaledExitCode(err error) (int, bool) {
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		return 0, false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return 0, false
	}
	return 128 + int(status.Signal()), true
}

// platformInteractiveShell resolves the user's shell. An empty command
// starts the shell interactively so it draws a prompt.
func platformInteractiveShell(command string) (string, []string) {
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = "sh"
	}
	if command == "" {
		return shellPath, []string{"-i"}
	}
	return shellPath, []string{"-c", command}
}
