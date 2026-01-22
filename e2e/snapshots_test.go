package e2e

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSnapshotHelp tests help output snapshot.
func TestSnapshotHelp(t *testing.T) {
	SkipIfE2EDisabled(t)

	// Use a tall terminal to capture all help output.
	term := NewTestTerminal(t, []string{"--help"}, 100, 80)
	defer term.Close()

	// Give time for output to appear.
	time.Sleep(1 * time.Second)

	snap := term.Snapshot()
	output := SnapshotText(snap)

	require.Contains(t, output, "USAGE", "Expected USAGE in help output")
	require.Contains(t, output, "FLAGS", "Expected FLAGS in help output")
}

// TestSnapshotVersion tests version output matches expected format.
func TestSnapshotVersion(t *testing.T) {
	SkipIfE2EDisabled(t)

	term := NewTestTerminal(t, []string{"--version"}, 80, 20)
	defer term.Close()

	require.True(t, WaitForText(t, term, "version", 5*time.Second),
		"Expected version")

	// Check that the version matches the expected format.
	// Pattern: v<semver>-<timestamp>-<commit>+<dirty>?
	snap := term.Snapshot()
	output := SnapshotText(snap)
	versionPattern := regexp.MustCompile(`crush version v\d+\.\d+\.\d+-\d+\.\d+-[a-f0-9]+(\+dirty)?`)
	require.True(t, versionPattern.MatchString(output),
		"Version output doesn't match expected format.\nGot: %s", strings.TrimSpace(output))
}

// TestSnapshotRunHelp tests run help snapshot.
func TestSnapshotRunHelp(t *testing.T) {
	SkipIfE2EDisabled(t)

	term := NewTestTerminal(t, []string{"run", "--help"}, 100, 40)
	defer term.Close()

	require.True(t, WaitForText(t, term, "non-interactive", 5*time.Second),
		"Expected non-interactive")

	snap := term.Snapshot()
	output := SnapshotText(snap)
	require.Contains(t, output, "Run a single", "Expected run command description")
}
