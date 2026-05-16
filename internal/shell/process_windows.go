//go:build windows

package shell

import (
	"os/exec"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// killTimeout is zero on Windows — we send one hard kill via
// TerminateJobObject rather than a two-stage interrupt-then-kill.
// The process-group-level equivalent of SIGINT (GenerateConsoleCtrlEvent)
// is unreliable for non-console processes and job objects make the
// two-stage dance unnecessary.
const killTimeout = 0 * time.Second

// jobHandles maps child PID → Windows job object handle. An entry
// exists from startedCmd (immediately after cmd.Start()) through
// cleanupCmd (after cmd.Wait()). killCmd uses the handle to terminate
// the entire job tree on cancellation.
var jobHandles sync.Map // map[int]windows.Handle

// prepareCmd is a no-op on Windows. We assign the process to a job
// object in startedCmd (called right after cmd.Start()), accepting a
// micro-race where the process could fork before assignment. In
// practice the window is too small for any real command to exploit it.
func prepareCmd(cmd *exec.Cmd) {}

// startedCmd creates a job object and assigns the newborn process to
// it so that on cancellation we can terminate the entire process tree
// via TerminateJobObject. It is called immediately after cmd.Start().
func startedCmd(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid

	jh, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return
	}

	// Open a handle to the child for AssignProcessToJobObject.
	ph, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(pid))
	if err != nil {
		windows.CloseHandle(jh)
		return
	}
	defer windows.CloseHandle(ph)

	if err := windows.AssignProcessToJobObject(jh, ph); err != nil {
		windows.CloseHandle(jh)
		return
	}

	jobHandles.Store(pid, jh)
}

// interruptCmd is a no-op on Windows — killCmd does the hard work
// via TerminateJobObject, which is equivalent to SIGKILL on the
// entire job tree.
func interruptCmd(cmd *exec.Cmd) error {
	return nil
}

// killCmd terminates the entire job tree via TerminateJobObject.
// This is the Windows equivalent of sending SIGKILL to a Unix
// process group.
func killCmd(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if v, ok := jobHandles.Load(cmd.Process.Pid); ok {
		_ = windows.TerminateJobObject(v.(windows.Handle), 1)
	}
	return nil
}

// cleanupCmd removes the job handle entry after the process exits.
// On normal exit, the main process is already gone and grandchildren
// (if any) are intentionally left running. On cancel, killCmd will
// have already called TerminateJobObject, so the job is empty.
func cleanupCmd(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if v, ok := jobHandles.Load(cmd.Process.Pid); ok {
		windows.CloseHandle(v.(windows.Handle))
		jobHandles.Delete(cmd.Process.Pid)
	}
}
