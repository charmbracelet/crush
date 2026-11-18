//go:build windows

package shell

import (
	"os/exec"
	"time"

	"mvdan.cc/sh/v3/interp"
)

// defaultKillTimeout matches mvdan's DefaultExecHandler default.
const defaultKillTimeout = 2 * time.Second

// isolateProcess is a no-op on Windows. Session isolation via Setsid is a
// Unix-only concept; Windows uses CREATE_NEW_PROCESS_GROUP which mvdan's
// default handler already handles adequately.
func isolateProcess(_ *exec.Cmd) {}

// killGroupOnCancel only sets a WaitDelay on Windows; negative-PID signal
// targeting is Unix-only, so cancellation keeps the default process kill.
func killGroupOnCancel(cmd *exec.Cmd) {
	cmd.WaitDelay = defaultKillTimeout + time.Second
}

// processGroupExecHandler returns interp.DefaultExecHandler on Windows.
func processGroupExecHandler(killTimeout time.Duration) interp.ExecHandlerFunc {
	return interp.DefaultExecHandler(killTimeout)
}
