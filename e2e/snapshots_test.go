package e2e

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aleksclark/trifle"
)

// TestSnapshotHelp tests help output snapshot.
func TestSnapshotHelp(t *testing.T) {
	SkipIfE2EDisabled(t)
	trifle.SkipOnWindows(t)

	term, err := trifle.NewTerminal(CrushBinary(), []string{"--help"}, trifle.TerminalOptions{
		Rows: 40,
		Cols: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	locator := term.GetByText("USAGE", trifle.WithFull())
	if err := locator.WaitVisible(5 * time.Second); err != nil {
		t.Fatalf("Expected USAGE: %v", err)
	}

	h := trifle.NewTestHelper(t, term)
	h.MatchSnapshot("help-output.txt")
}

// TestSnapshotVersion tests version output matches expected format.
func TestSnapshotVersion(t *testing.T) {
	SkipIfE2EDisabled(t)
	trifle.SkipOnWindows(t)

	term, err := trifle.NewTerminal(CrushBinary(), []string{"--version"}, trifle.TerminalOptions{
		Rows: 20,
		Cols: 80,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	locator := term.GetByText("version", trifle.WithFull())
	if err := locator.WaitVisible(5 * time.Second); err != nil {
		t.Fatalf("Expected version: %v", err)
	}

	// Check that the version matches the expected format.
	// Pattern: v<semver>-<timestamp>-<commit>+<dirty>?
	output := term.Output()
	versionPattern := regexp.MustCompile(`crush version v\d+\.\d+\.\d+-\d+\.\d+-[a-f0-9]+(\+dirty)?`)
	if !versionPattern.MatchString(output) {
		t.Errorf("Version output doesn't match expected format.\nGot: %s", strings.TrimSpace(output))
	}
}

// TestSnapshotRunHelp tests run help snapshot.
func TestSnapshotRunHelp(t *testing.T) {
	SkipIfE2EDisabled(t)
	trifle.SkipOnWindows(t)

	term, err := trifle.NewTerminal(CrushBinary(), []string{"run", "--help"}, trifle.TerminalOptions{
		Rows: 40,
		Cols: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	locator := term.GetByText("non-interactive", trifle.WithFull())
	if err := locator.WaitVisible(5 * time.Second); err != nil {
		t.Fatalf("Expected non-interactive: %v", err)
	}

	h := trifle.NewTestHelper(t, term)
	h.MatchSnapshot("run-help.txt")
}
