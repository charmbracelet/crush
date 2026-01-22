package e2e

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestStartupVersion tests the --version flag.
func TestStartupVersion(t *testing.T) {
	SkipIfE2EDisabled(t)
	SkipOnWindows(t)

	term := NewTestTerminal(t, []string{"--version"}, 80, 24)
	defer term.Close()

	require.True(t, WaitForText(t, term, "version", 5*time.Second),
		"Expected version information")
}

// TestStartupHelp tests the --help flag.
func TestStartupHelp(t *testing.T) {
	SkipIfE2EDisabled(t)
	SkipOnWindows(t)

	// Use a tall terminal to capture all help output.
	term := NewTestTerminal(t, []string{"--help"}, 100, 80)
	defer term.Close()

	time.Sleep(1 * time.Second)
	snap := term.Snapshot()
	output := SnapshotText(snap)
	require.Contains(t, output, "USAGE", "Expected help information")
}

// TestStartupRunHelp tests the run command help.
func TestStartupRunHelp(t *testing.T) {
	SkipIfE2EDisabled(t)
	SkipOnWindows(t)

	term := NewTestTerminal(t, []string{"run", "--help"}, 100, 40)
	defer term.Close()

	t.Run("shows run command usage", func(t *testing.T) {
		require.True(t, WaitForText(t, term, "non-interactive", 5*time.Second),
			"Expected run help")
	})

	t.Run("shows quiet flag option", func(t *testing.T) {
		require.True(t, WaitForText(t, term, "Hide spinner", 5*time.Second),
			"Expected quiet flag")
	})
}

// TestStartupRunNoPrompt tests run command error without prompt.
func TestStartupRunNoPrompt(t *testing.T) {
	SkipIfE2EDisabled(t)
	SkipOnWindows(t)

	term := NewTestTerminal(t, []string{"run"}, 80, 24)
	defer term.Close()

	require.True(t, WaitForText(t, term, "No prompt provided", 5*time.Second),
		"Expected error message")
}

// TestStartupDebugFlag tests the -d debug flag.
func TestStartupDebugFlag(t *testing.T) {
	SkipIfE2EDisabled(t)
	SkipOnWindows(t)

	term := NewTestTerminal(t, []string{"-d", "--help"}, 100, 80)
	defer term.Close()

	time.Sleep(1 * time.Second)
	snap := term.Snapshot()
	output := SnapshotText(snap)
	require.Contains(t, output, "EXAMPLES", "Expected help output with debug flag")
}

// TestStartupYoloFlag tests the -y yolo flag.
func TestStartupYoloFlag(t *testing.T) {
	SkipIfE2EDisabled(t)
	SkipOnWindows(t)

	term := NewTestTerminal(t, []string{"-y", "--help"}, 100, 80)
	defer term.Close()

	time.Sleep(1 * time.Second)
	snap := term.Snapshot()
	output := SnapshotText(snap)
	require.Contains(t, output, "EXAMPLES", "Expected help output with yolo flag")
}

// TestStartupDirs tests the dirs command.
func TestStartupDirs(t *testing.T) {
	SkipIfE2EDisabled(t)
	SkipOnWindows(t)

	term := NewTestTerminal(t, []string{"dirs"}, 80, 24)
	defer term.Close()

	// Wait for output to complete.
	cmd := exec.Command(CrushBinary(), "dirs")
	if err := term.Wait(cmd); err != nil {
		// May already be closed, that's ok.
	}

	time.Sleep(1 * time.Second)
	snap := term.Snapshot()
	output := SnapshotText(snap)
	require.Contains(t, output, "crush", "Expected crush directory info")
}
