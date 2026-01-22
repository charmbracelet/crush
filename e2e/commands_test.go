package e2e

import (
	"testing"
	"time"

	"github.com/aleksclark/trifle"
)

// TestCommandsRunHelp tests run command help output.
func TestCommandsRunHelp(t *testing.T) {
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
		t.Errorf("Expected run help: %v", err)
	}
}

// TestCommandsRunMissingPrompt tests run command without prompt.
func TestCommandsRunMissingPrompt(t *testing.T) {
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
		t.Errorf("Expected error: %v", err)
	}
}

// TestCommandsProjects tests the projects command.
// Note: This test may fail if the projects database has invalid JSON.
func TestCommandsProjects(t *testing.T) {
	SkipIfE2EDisabled(t)
	t.Skip("Projects command may fail due to project database issues")
	trifle.SkipOnWindows(t)

	term, err := trifle.NewTerminal(CrushBinary(), []string{"projects"}, trifle.TerminalOptions{
		Rows: 40,
		Cols: 120,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	// Wait for process to complete.
	if err := term.WaitWithTimeout(10 * time.Second); err != nil {
		t.Fatalf("Process did not exit: %v", err)
	}

	// Projects command should produce output (may or may not have projects).
	output := term.Output()
	// Either shows projects or exits cleanly.
	if len(output) == 0 {
		t.Error("Expected some output from projects command")
	}
}

// TestCommandsSchema tests the schema command.
func TestCommandsSchema(t *testing.T) {
	SkipIfE2EDisabled(t)
	trifle.SkipOnWindows(t)

	term, err := trifle.NewTerminal(CrushBinary(), []string{"schema"}, trifle.TerminalOptions{
		Rows: 40,
		Cols: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	// Wait for output to contain schema properties.
	locator := term.GetByText("properties", trifle.WithFull())
	if err := locator.WaitVisible(5 * time.Second); err != nil {
		t.Fatalf("Expected JSON schema with properties: %v", err)
	}
}
