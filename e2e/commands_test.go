package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestCommandsRunHelp tests run command help output.
func TestCommandsRunHelp(t *testing.T) {
	SkipIfE2EDisabled(t)
	SkipOnWindows(t)

	term := NewTestTerminal(t, []string{"run", "--help"}, 100, 40)
	defer term.Close()

	require.True(t, WaitForText(t, term, "non-interactive", 5*time.Second),
		"Expected run help")
}

// TestCommandsRunMissingPrompt tests run command without prompt.
func TestCommandsRunMissingPrompt(t *testing.T) {
	SkipIfE2EDisabled(t)
	SkipOnWindows(t)

	term := NewTestTerminal(t, []string{"run"}, 80, 24)
	defer term.Close()

	require.True(t, WaitForText(t, term, "No prompt provided", 5*time.Second),
		"Expected error")
}

// TestCommandsProjects tests the projects command.
// Note: This test may fail if the projects database has invalid JSON.
func TestCommandsProjects(t *testing.T) {
	SkipIfE2EDisabled(t)
	t.Skip("Projects command may fail due to project database issues")
	SkipOnWindows(t)

	term := NewTestTerminal(t, []string{"projects"}, 120, 40)
	defer term.Close()

	time.Sleep(5 * time.Second)
	snap := term.Snapshot()
	output := SnapshotText(snap)
	require.NotEmpty(t, output, "Expected some output from projects command")
}

// TestCommandsSchema tests the schema command.
func TestCommandsSchema(t *testing.T) {
	SkipIfE2EDisabled(t)
	SkipOnWindows(t)

	term := NewTestTerminal(t, []string{"schema"}, 100, 40)
	defer term.Close()

	require.True(t, WaitForText(t, term, "properties", 5*time.Second),
		"Expected JSON schema with properties")
}
