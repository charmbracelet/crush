//go:build windows

package pinentry

import (
	"context"
	"os/exec"
)

// prepareProcess is a no-op on Windows: process-group isolation is not
// applicable, and the interactive terminal handover for GPG dialogs is a
// POSIX concern.
func prepareProcess(cmd *exec.Cmd) {}

// watchCancel terminates the child when ctx is done using the context
// cancellation mechanism, since Windows has no process groups here.
func watchCancel(ctx context.Context, cmd *exec.Cmd) (stop func()) {
	stopf := context.AfterFunc(ctx, func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	return func() { stopf() }
}
