//go:build !windows

package mcp

import (
	"os/exec"
	"syscall"
	"time"
)

// configureStdioProcess puts a stdio MCP server child in its own process group
// and makes context cancellation kill that whole group.
//
// A stdio server frequently spawns its own children (signal-mcp launches
// signal-cli, an npx-based server launches node). os/exec's default
// cancellation only signals the direct child, so those grandchildren are
// orphaned with PPID 1 and run forever; production accumulated 15+ such
// processes over two days. Setpgid makes the child a process-group leader
// (pgid == pid) and the Cancel hook signals the negated pid so every process in
// the group is reaped whenever the session's context is cancelled (on Close, a
// StateError transition, or a lazy renew).
func configureStdioProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true

	// Replaces os/exec's default cancel (which kills only cmd.Process). A
	// negative pid targets the whole process group.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	// Without a WaitDelay, a leaked descendant that keeps a stdio pipe open can
	// block cmd.Wait indefinitely even after the group is signalled.
	cmd.WaitDelay = 5 * time.Second
}
