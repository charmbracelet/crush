package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestStartupVersion tests the --version flag.
func TestStartupVersion(t *testing.T) {
	SkipIfE2EDisabled(t)

	term := NewTestTerminal(t, []string{"--version"}, 80, 24)
	defer term.Close()

	require.True(t, WaitForText(t, term, "version", 5*time.Second),
		"Expected version information")

	// Verify terminal state for short-lived command.
	snap := term.Snapshot()
	AssertAltScreen(t, snap, false)
}

// TestStartupHelp tests the --help flag.
func TestStartupHelp(t *testing.T) {
	SkipIfE2EDisabled(t)

	// Use a tall terminal to capture all help output.
	term := NewTestTerminal(t, []string{"--help"}, 100, 80)
	defer term.Close()

	time.Sleep(1 * time.Second)

	snap := term.Snapshot()
	output := SnapshotText(snap)

	require.Contains(t, output, "USAGE", "Expected help information")
	require.Contains(t, output, "FLAGS", "Expected FLAGS section")
	AssertAltScreen(t, snap, false)
	AssertDimensions(t, snap, 100, 80)
}

// TestStartupRunHelp tests the run command help.
func TestStartupRunHelp(t *testing.T) {
	SkipIfE2EDisabled(t)

	term := NewTestTerminal(t, []string{"run", "--help"}, 100, 80)
	defer term.Close()

	time.Sleep(500 * time.Millisecond)

	snap := term.Snapshot()
	output := SnapshotText(snap)

	t.Run("shows run command usage", func(t *testing.T) {
		require.Contains(t, output, "non-interactive", "Expected run help")
	})

	t.Run("shows quiet flag option", func(t *testing.T) {
		require.Contains(t, output, "Hide spinner", "Expected quiet flag")
	})

	t.Run("terminal state is correct", func(t *testing.T) {
		AssertAltScreen(t, snap, false)
	})
}

// TestStartupRunNoPrompt tests run command error without prompt.
func TestStartupRunNoPrompt(t *testing.T) {
	SkipIfE2EDisabled(t)

	term := NewTestTerminal(t, []string{"run"}, 80, 24)
	defer term.Close()

	require.True(t, WaitForText(t, term, "No prompt provided", 5*time.Second),
		"Expected error message")

	snap := term.Snapshot()
	AssertAltScreen(t, snap, false)
}

// TestStartupDebugFlag tests the -d debug flag.
func TestStartupDebugFlag(t *testing.T) {
	SkipIfE2EDisabled(t)

	term := NewTestTerminal(t, []string{"-d", "--help"}, 100, 80)
	defer term.Close()

	time.Sleep(1 * time.Second)

	snap := term.Snapshot()
	output := SnapshotText(snap)

	require.Contains(t, output, "EXAMPLES", "Expected help output with debug flag")
	AssertAltScreen(t, snap, false)
}

// TestStartupYoloFlag tests the -y yolo flag.
func TestStartupYoloFlag(t *testing.T) {
	SkipIfE2EDisabled(t)

	term := NewTestTerminal(t, []string{"-y", "--help"}, 100, 80)
	defer term.Close()

	time.Sleep(1 * time.Second)

	snap := term.Snapshot()
	output := SnapshotText(snap)

	require.Contains(t, output, "EXAMPLES", "Expected help output with yolo flag")
	AssertAltScreen(t, snap, false)
}

// TestStartupDirs tests the dirs command.
func TestStartupDirs(t *testing.T) {
	SkipIfE2EDisabled(t)

	term := NewTestTerminal(t, []string{"dirs"}, 80, 24)
	defer term.Close()

	time.Sleep(1 * time.Second)

	snap := term.Snapshot()
	output := SnapshotText(snap)

	require.Contains(t, output, "crush", "Expected crush directory info")
	AssertAltScreen(t, snap, false)
	AssertDimensions(t, snap, 80, 24)
}
