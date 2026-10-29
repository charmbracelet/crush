//go:build windows

package cmd

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// TestDetachProcess_HiddenConsole guards the detached server running without
// a console, so every console program it starts (git, LSP, MCP, bash) pops
// up a visible window of its own.
func TestDetachProcess_HiddenConsole(t *testing.T) {
	t.Parallel()

	c := exec.CommandContext(t.Context(), "crush", "server")
	detachProcess(c)
	flags := c.SysProcAttr.CreationFlags
	require.NotZero(t, flags&windows.CREATE_NO_WINDOW)
	require.Zero(t, flags&windows.DETACHED_PROCESS, "DETACHED_PROCESS leaves it without a console")
}
