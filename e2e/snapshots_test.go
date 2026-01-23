package e2e

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSnapshotHelp tests help output using terminal state assertions.
func TestSnapshotHelp(t *testing.T) {
	SkipIfE2EDisabled(t)

	// Use a tall terminal to capture all help output.
	term := NewTestTerminal(t, []string{"--help"}, 100, 80)
	defer term.Close()

	// Wait for output to appear.
	time.Sleep(1 * time.Second)

	// Verify key content is present.
	snap := term.Snapshot()
	output := SnapshotText(snap)
	require.Contains(t, output, "USAGE", "Expected USAGE in help output")
	require.Contains(t, output, "FLAGS", "Expected FLAGS in help output")
	require.Contains(t, output, "COMMANDS", "Expected COMMANDS in help output")

	// Verify terminal state - help command shouldn't use alt screen.
	AssertAltScreen(t, snap, false)
	AssertDimensions(t, snap, 100, 80)
}

// TestSnapshotVersion tests version output matches expected format.
func TestSnapshotVersion(t *testing.T) {
	SkipIfE2EDisabled(t)

	term := NewTestTerminal(t, []string{"--version"}, 80, 20)
	defer term.Close()

	time.Sleep(500 * time.Millisecond)

	snap := term.Snapshot()
	output := SnapshotText(snap)

	// Check that the version matches the expected format.
	// Pattern: v<semver>-<timestamp>-<commit>+<dirty>?
	versionPattern := regexp.MustCompile(`crush version v\d+\.\d+\.\d+-\d+\.\d+-[a-f0-9]+(\+dirty)?`)
	require.True(t, versionPattern.MatchString(output),
		"Version output doesn't match expected format.\nGot: %s", strings.TrimSpace(output))

	// Short-lived command should not use alt screen.
	AssertAltScreen(t, snap, false)
}

// TestSnapshotRunHelp tests run help output.
func TestSnapshotRunHelp(t *testing.T) {
	SkipIfE2EDisabled(t)

	term := NewTestTerminal(t, []string{"run", "--help"}, 100, 80)
	defer term.Close()

	time.Sleep(500 * time.Millisecond)

	snap := term.Snapshot()
	output := SnapshotText(snap)

	require.Contains(t, output, "Run a single", "Expected run command description")
	require.Contains(t, output, "non-interactive", "Expected non-interactive in help")
	require.Contains(t, output, "--quiet", "Expected --quiet flag")
}

// TestSnapshotSchema tests schema command JSON output.
func TestSnapshotSchema(t *testing.T) {
	SkipIfE2EDisabled(t)

	term := NewTestTerminal(t, []string{"schema"}, 120, 200)
	defer term.Close()

	time.Sleep(1 * time.Second)

	snap := term.Snapshot()
	output := SnapshotText(snap)

	// Schema is very long, so we just check for fields that appear in the output.
	// The output shows the tail of the schema since it scrolls.
	require.Contains(t, output, "type", "Expected type field in schema")
	require.Contains(t, output, "properties", "Expected properties field")
	AssertAltScreen(t, snap, false)
}

// TestSnapshotDirs tests dirs command output.
func TestSnapshotDirs(t *testing.T) {
	SkipIfE2EDisabled(t)

	term := NewTestTerminal(t, []string{"dirs"}, 100, 30)
	defer term.Close()

	time.Sleep(1 * time.Second)

	snap := term.Snapshot()
	output := SnapshotText(snap)

	// Should show directory paths.
	require.Contains(t, output, "crush", "Expected crush in directory paths")
}
