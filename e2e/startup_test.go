package e2e

import (
	"testing"
	"time"

	"github.com/aleksclark/trifle"
)

// TestStartupVersion tests the --version flag.
func TestStartupVersion(t *testing.T) {
	SkipIfE2EDisabled(t)
	trifle.SkipOnWindows(t)

	term, err := trifle.NewTerminal(CrushBinary(), []string{"--version"}, trifle.TerminalOptions{
		Rows: 24,
		Cols: 80,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	// Wait for output.
	locator := term.GetByText("version", trifle.WithFull())
	if err := locator.WaitVisible(5 * time.Second); err != nil {
		t.Errorf("Expected version information: %v", err)
	}
}

// TestStartupHelp tests the --help flag.
func TestStartupHelp(t *testing.T) {
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
		t.Errorf("Expected help information: %v", err)
	}
}

// TestStartupRunHelp tests the run command help.
func TestStartupRunHelp(t *testing.T) {
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

	t.Run("shows run command usage", func(t *testing.T) {
		locator := term.GetByText("Run a single", trifle.WithFull())
		if err := locator.WaitVisible(5 * time.Second); err != nil {
			t.Errorf("Expected run help: %v", err)
		}
	})

	t.Run("shows quiet flag option", func(t *testing.T) {
		locator := term.GetByText("Hide spinner", trifle.WithFull())
		if err := locator.WaitVisible(5 * time.Second); err != nil {
			t.Errorf("Expected quiet flag: %v", err)
		}
	})
}

// TestStartupRunNoPrompt tests run command error without prompt.
func TestStartupRunNoPrompt(t *testing.T) {
	SkipIfE2EDisabled(t)
	trifle.SkipOnWindows(t)

	term, err := trifle.NewTerminal(CrushBinary(), []string{"run"}, trifle.TerminalOptions{
		Rows: 24,
		Cols: 80,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	locator := term.GetByText("No prompt provided", trifle.WithFull())
	if err := locator.WaitVisible(5 * time.Second); err != nil {
		t.Errorf("Expected error message: %v", err)
	}
}

// TestStartupDebugFlag tests the -d debug flag.
func TestStartupDebugFlag(t *testing.T) {
	SkipIfE2EDisabled(t)
	trifle.SkipOnWindows(t)

	term, err := trifle.NewTerminal(CrushBinary(), []string{"-d", "--help"}, trifle.TerminalOptions{
		Rows: 40,
		Cols: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	locator := term.GetByText("EXAMPLES", trifle.WithFull())
	if err := locator.WaitVisible(5 * time.Second); err != nil {
		t.Errorf("Expected help output with debug flag: %v", err)
	}
}

// TestStartupYoloFlag tests the -y yolo flag.
func TestStartupYoloFlag(t *testing.T) {
	SkipIfE2EDisabled(t)
	trifle.SkipOnWindows(t)

	term, err := trifle.NewTerminal(CrushBinary(), []string{"-y", "--help"}, trifle.TerminalOptions{
		Rows: 40,
		Cols: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	locator := term.GetByText("EXAMPLES", trifle.WithFull())
	if err := locator.WaitVisible(5 * time.Second); err != nil {
		t.Errorf("Expected help output with yolo flag: %v", err)
	}
}

// TestStartupDirs tests the dirs command.
func TestStartupDirs(t *testing.T) {
	SkipIfE2EDisabled(t)
	trifle.SkipOnWindows(t)

	term, err := trifle.NewTerminal(CrushBinary(), []string{"dirs"}, trifle.TerminalOptions{
		Rows: 24,
		Cols: 80,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	// Wait for output to complete.
	time.Sleep(1 * time.Second)

	h := trifle.NewTestHelper(t, term)
	h.MatchSnapshot("dirs-output.txt")
}
