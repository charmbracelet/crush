package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReloadHandler_CommandsRefreshed verifies that after reload, commands are refreshed correctly.
// This tests the core reload functionality that the handler uses.
func TestReloadHandler_CommandsRefreshed(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create initial command
	cmdFile := filepath.Join(commandsDir, "initial.md")
	require.NoError(t, os.WriteFile(cmdFile, []byte(`---
description: Initial command
---
# Initial
`), 0o644))

	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Verify initial command exists
	commands := registry.ListCommands()
	require.Len(t, commands, 1)
	assert.Equal(t, "initial", commands[0].Name)

	// Add a new command
	newCmdFile := filepath.Join(commandsDir, "new-cmd.md")
	require.NoError(t, os.WriteFile(newCmdFile, []byte(`---
description: New command
---
# New
`), 0o644))

	// Reload and verify new command appears
	err = registry.Reload()
	require.NoError(t, err)

	commands = registry.ListCommands()
	require.Len(t, commands, 2)

	// Verify both commands exist
	names := make(map[string]bool)
	for _, cmd := range commands {
		names[cmd.Name] = true
	}
	assert.True(t, names["initial"], "Initial command should still exist")
	assert.True(t, names["new-cmd"], "New command should appear after reload")
}

// TestReloadHandler_ErrorHandling verifies that reload errors are handled gracefully.
func TestReloadHandler_ErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	// Create registry with invalid directory (will cause reload to fail)
	// Use a non-existent parent directory to simulate error
	invalidDir := filepath.Join(projectDir, "nonexistent", "deep", "path")
	registry := NewRegistry(invalidDir)

	// First load will fail (directory doesn't exist)
	_, _ = registry.LoadCommands()
	// This might not error if loaders handle missing dirs gracefully
	// But reload should still work

	// Reload should handle gracefully (even if it fails)
	_ = registry.Reload()
	// Reload should not panic, even if it encounters errors
	// The handler will log errors and return error message to user
	assert.NotPanics(t, func() {
		_ = registry.Reload()
	})
}

// TestReloadHandler_CompletionProviderRefresh verifies that completion provider
// gets fresh commands after reload by creating a new registry (which is what
// the completion provider does).
func TestReloadHandler_CompletionProviderRefresh(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create initial command
	cmdFile := filepath.Join(commandsDir, "cmd1.md")
	require.NoError(t, os.WriteFile(cmdFile, []byte(`---
description: Command 1
---
# Cmd1
`), 0o644))

	// Simulate what completion provider does: create fresh registry each time
	registry1 := NewRegistry(projectDir)
	_, err := registry1.LoadCommands()
	require.NoError(t, err)

	commands1 := registry1.ListCommands()
	require.Len(t, commands1, 1)

	// Add new command
	newCmdFile := filepath.Join(commandsDir, "cmd2.md")
	require.NoError(t, os.WriteFile(newCmdFile, []byte(`---
description: Command 2
---
# Cmd2
`), 0o644))

	// Simulate reload happening (via another registry instance)
	registry2 := NewRegistry(projectDir)
	err = registry2.Reload()
	require.NoError(t, err)

	// Simulate completion provider creating fresh registry (after reload)
	// This is what startCommandCompletions() does
	registry3 := NewRegistry(projectDir)
	_, err = registry3.LoadCommands()
	require.NoError(t, err)

	commands3 := registry3.ListCommands()
	require.Len(t, commands3, 2, "Completion provider should see both commands after reload")

	// Verify both commands are present
	names := make(map[string]bool)
	for _, cmd := range commands3 {
		names[cmd.Name] = true
	}
	assert.True(t, names["cmd1"], "Command 1 should be present")
	assert.True(t, names["cmd2"], "Command 2 should be present after reload")
}

// TestReloadHandler_RemovedCommands verifies that removed commands no longer appear after reload.
func TestReloadHandler_RemovedCommands(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create two commands
	cmd1File := filepath.Join(commandsDir, "cmd1.md")
	require.NoError(t, os.WriteFile(cmd1File, []byte(`---
description: Command 1
---
# Cmd1
`), 0o644))

	cmd2File := filepath.Join(commandsDir, "cmd2.md")
	require.NoError(t, os.WriteFile(cmd2File, []byte(`---
description: Command 2
---
# Cmd2
`), 0o644))

	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Verify both exist
	commands := registry.ListCommands()
	require.Len(t, commands, 2)

	// Remove one command
	require.NoError(t, os.Remove(cmd2File))

	// Reload
	err = registry.Reload()
	require.NoError(t, err)

	// Verify only one remains
	commands = registry.ListCommands()
	require.Len(t, commands, 1)
	assert.Equal(t, "cmd1", commands[0].Name)
}

