//go:build windows

package mcp

import "os/exec"

// configureStdioProcess is a no-op on platforms without POSIX process groups.
// os/exec's default context cancellation still kills the direct child.
func configureStdioProcess(cmd *exec.Cmd) {}
