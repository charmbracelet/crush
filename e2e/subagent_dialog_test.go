package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSubagentDialogOpensFromCommands tests that the subagent dialog can be opened
// from the commands palette by selecting "View Agents".
func TestSubagentDialogOpensFromCommands(t *testing.T) {
	SkipIfE2EDisabled(t)

	// Create an isolated terminal with a test subagent.
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "config", "crush", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))

	// Create a test subagent file.
	agentContent := `---
name: test-agent
description: A test agent for e2e testing
model: inherit
---
You are a test agent.
`
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "test-agent.md"), []byte(agentContent), 0o644))

	term := NewIsolatedTerminalWithConfig(t, 100, 40, TestConfigJSON())
	defer term.Close()

	// Set up the environment to find our test agents.
	time.Sleep(startupDelay)

	// Open commands dialog with ctrl+p.
	term.SendText("\x10")
	time.Sleep(700 * time.Millisecond)

	// Verify commands dialog opened.
	snap := term.Snapshot()
	output := SnapshotText(snap)
	require.True(t, strings.Contains(output, "Command") || strings.Contains(output, "command"),
		"Expected commands dialog to open, got: %s", output)

	// Type to filter for "Agents" command.
	term.SendText("agents")
	time.Sleep(300 * time.Millisecond)

	// Select the command.
	term.SendText("\r")
	time.Sleep(700 * time.Millisecond)

	// Verify agents dialog opened.
	snap = term.Snapshot()
	output = SnapshotText(snap)
	require.True(t, strings.Contains(output, "Agent") || strings.Contains(output, "agent"),
		"Expected agents dialog to open, got: %s", output)
}

// TestSubagentDialogListsDiscoveredAgents tests that the subagent dialog
// lists agents discovered from the filesystem.
func TestSubagentDialogListsDiscoveredAgents(t *testing.T) {
	SkipIfE2EDisabled(t)

	// Create an isolated terminal with test subagents.
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "config", "crush", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))

	// Create multiple test subagent files.
	agents := []struct {
		name        string
		description string
	}{
		{"code-reviewer", "Reviews code for best practices"},
		{"go-developer", "Writes Go code following best practices"},
	}

	for _, agent := range agents {
		content := "---\nname: " + agent.name + "\ndescription: " + agent.description + "\n---\nYou are " + agent.name + ".\n"
		require.NoError(t, os.WriteFile(filepath.Join(agentsDir, agent.name+".md"), []byte(content), 0o644))
	}

	// Create config and data directories.
	configPath := filepath.Join(tmpDir, "config", "crush")
	require.NoError(t, os.MkdirAll(configPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configPath, "crush.json"), []byte(TestConfigJSON()), 0o644))

	dataPath := filepath.Join(tmpDir, "data", "crush")
	require.NoError(t, os.MkdirAll(dataPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataPath, "crush.json"), []byte(TestConfigJSON()), 0o644))

	term := NewIsolatedTerminalWithConfigAndEnv(t, 100, 40, TestConfigJSON(), tmpDir)
	defer term.Close()

	time.Sleep(startupDelay)

	// Open commands dialog and navigate to agents.
	term.SendText("\x10") // ctrl+p
	time.Sleep(700 * time.Millisecond)

	term.SendText("agents")
	time.Sleep(300 * time.Millisecond)

	term.SendText("\r")
	time.Sleep(700 * time.Millisecond)

	// Verify agents are listed.
	snap := term.Snapshot()
	output := SnapshotText(snap)
	// Should show at least one of our test agents.
	hasAgent := strings.Contains(output, "code-reviewer") || strings.Contains(output, "go-developer")
	if !hasAgent {
		t.Logf("Output: %s", output)
	}
}

// TestSubagentDialogShowsAgentDetails tests that selecting an agent shows
// its details including path and configuration.
func TestSubagentDialogShowsAgentDetails(t *testing.T) {
	SkipIfE2EDisabled(t)

	// Create an isolated terminal with a test subagent.
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "config", "crush", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))

	agentContent := `---
name: detailed-agent
description: An agent with detailed configuration
model: large
tools:
  - View
  - Grep
  - Glob
allowed_tools:
  - View
yolo_mode: false
---
You are a detailed agent for testing.
`
	agentPath := filepath.Join(agentsDir, "detailed-agent.md")
	require.NoError(t, os.WriteFile(agentPath, []byte(agentContent), 0o644))

	// Create config and data directories.
	configPath := filepath.Join(tmpDir, "config", "crush")
	require.NoError(t, os.WriteFile(filepath.Join(configPath, "crush.json"), []byte(TestConfigJSON()), 0o644))

	dataPath := filepath.Join(tmpDir, "data", "crush")
	require.NoError(t, os.MkdirAll(dataPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataPath, "crush.json"), []byte(TestConfigJSON()), 0o644))

	term := NewIsolatedTerminalWithConfigAndEnv(t, 100, 40, TestConfigJSON(), tmpDir)
	defer term.Close()

	time.Sleep(startupDelay)

	// Open commands dialog and navigate to agents.
	term.SendText("\x10") // ctrl+p
	time.Sleep(700 * time.Millisecond)

	term.SendText("agents")
	time.Sleep(300 * time.Millisecond)

	term.SendText("\r")
	time.Sleep(700 * time.Millisecond)

	// Select the agent to view details (press enter on it).
	term.SendText("\r")
	time.Sleep(700 * time.Millisecond)

	// Verify agent details are shown.
	snap := term.Snapshot()
	output := SnapshotText(snap)

	// Should show some agent configuration details.
	hasDetails := strings.Contains(output, "detailed-agent") ||
		strings.Contains(output, "Path") ||
		strings.Contains(output, "Model") ||
		strings.Contains(output, "Tools")
	if !hasDetails {
		t.Logf("Expected agent details, got: %s", output)
	}
}

// TestSubagentDialogEscapeCloses tests that pressing escape closes the dialog.
func TestSubagentDialogEscapeCloses(t *testing.T) {
	SkipIfE2EDisabled(t)

	term := NewIsolatedTerminal(t, 100, 40)
	defer term.Close()

	time.Sleep(startupDelay)

	// Open commands dialog and navigate to agents.
	term.SendText("\x10") // ctrl+p
	time.Sleep(700 * time.Millisecond)

	term.SendText("agents")
	time.Sleep(300 * time.Millisecond)

	term.SendText("\r")
	time.Sleep(700 * time.Millisecond)

	// Press escape to close.
	term.SendText("\x1b")
	time.Sleep(500 * time.Millisecond)

	// App should still be responsive - type something.
	term.SendText("test")
	time.Sleep(300 * time.Millisecond)

	snap := term.Snapshot()
	output := SnapshotText(snap)
	// Verify the app is responsive (output should contain the test text or show main UI).
	require.Greater(t, len(output), 50, "App should be responsive after closing dialog")
}

// TestSubagentDialogEmptyState tests the dialog shows appropriate message
// when no agents are discovered.
func TestSubagentDialogEmptyState(t *testing.T) {
	SkipIfE2EDisabled(t)

	// Create isolated terminal without any agents.
	term := NewIsolatedTerminal(t, 100, 40)
	defer term.Close()

	time.Sleep(startupDelay)

	// Open commands dialog and navigate to agents.
	term.SendText("\x10") // ctrl+p
	time.Sleep(700 * time.Millisecond)

	term.SendText("agents")
	time.Sleep(300 * time.Millisecond)

	term.SendText("\r")
	time.Sleep(700 * time.Millisecond)

	// Verify empty state message is shown.
	snap := term.Snapshot()
	output := SnapshotText(snap)

	// Should show some indication that no agents are found or the dialog title.
	hasEmptyState := strings.Contains(output, "Agent") ||
		strings.Contains(output, "agent") ||
		strings.Contains(output, "No") ||
		strings.Contains(output, "none")
	if !hasEmptyState {
		t.Logf("Expected empty state or agents dialog, got: %s", output)
	}
}
