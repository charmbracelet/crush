//go:build windows

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// detachRoleEnv names the part the test binary plays when
// TestDetachProcess_ChildrenGetNoConsoleWindow re-executes it.
const detachRoleEnv = "CRUSH_TEST_DETACH_ROLE"

// The detached server must own a console that has no window, so that the
// console programs it runs (git, LSP and MCP servers, bash tool commands)
// inherit it, as they inherit the terminal's console in local mode.
// DETACHED_PROCESS leaves the server without a console, so Windows gives
// each of those programs a new, visible console window instead, and it
// makes Windows ignore CREATE_NO_WINDOW.
func TestDetachProcess_HiddenConsole(t *testing.T) {
	t.Parallel()

	c := exec.CommandContext(t.Context(), "crush", "server")
	detachProcess(c)

	flags := c.SysProcAttr.CreationFlags
	require.NotZero(t, flags&windows.CREATE_NO_WINDOW,
		"the server must get a console without a window")
	require.Zero(t, flags&windows.DETACHED_PROCESS,
		"DETACHED_PROCESS leaves the server without a console")
}

// A console program that the detached server starts the way it starts git,
// LSP and MCP servers and bash tool commands must not pop up a console
// window of its own.
func TestDetachProcess_ChildrenGetNoConsoleWindow(t *testing.T) {
	t.Parallel()

	c := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestDetachProcessHelper$")
	c.Env = append(os.Environ(), detachRoleEnv+"=server")
	detachProcess(c)
	var stderr strings.Builder
	c.Stderr = &stderr

	out, err := c.Output()
	require.NoError(t, err, stderr.String())
	require.Equal(t, "false", strings.TrimSpace(string(out)),
		"a console program run by the detached server got a visible console window")
}

// TestDetachProcessHelper is not a real test. It is the process that
// TestDetachProcess_ChildrenGetNoConsoleWindow starts, in the role that
// detachRoleEnv names.
func TestDetachProcessHelper(t *testing.T) {
	switch os.Getenv(detachRoleEnv) {
	case "server":
		// Start a console program with plain os/exec and no creation flags,
		// as the server does, and pass on its answer.
		c := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestDetachProcessHelper$")
		c.Env = append(os.Environ(), detachRoleEnv+"=child")
		out, err := c.Output()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(string(out))
		os.Exit(0)
	case "child":
		// Report whether this process has a visible console window.
		hwnd, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
		fmt.Print(hwnd != 0 && windows.IsWindowVisible(windows.HWND(hwnd)))
		os.Exit(0)
	}
}
