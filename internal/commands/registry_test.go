package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_FindCommand(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	// Create commands directory structure
	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create root command
	rootCmd := filepath.Join(commandsDir, "root.md")
	require.NoError(t, os.WriteFile(rootCmd, []byte(`---
description: Root command
---
# Root
`), 0o644))

	// Create namespaced command
	nsDir := filepath.Join(commandsDir, "frontend")
	require.NoError(t, os.MkdirAll(nsDir, 0o755))
	nsCmd := filepath.Join(nsDir, "review.md")
	require.NoError(t, os.WriteFile(nsCmd, []byte(`---
description: Frontend review
---
# Review
`), 0o644))

	// Create registry and load commands
	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Test finding root command
	cmd, err := registry.FindCommand("root")
	require.NoError(t, err)
	assert.Equal(t, "root", cmd.Name)
	assert.Equal(t, "", cmd.Namespace)
	assert.Equal(t, "project", cmd.Source)

	// Test finding namespaced command
	cmd, err = registry.FindCommand("frontend:review")
	require.NoError(t, err)
	assert.Equal(t, "frontend:review", cmd.Name)
	assert.Equal(t, "frontend", cmd.Namespace)
	assert.Equal(t, "project:frontend", cmd.Source)

	// Test finding non-existent command
	_, err = registry.FindCommand("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command not found")
}

func TestRegistry_FindCommand_NamespacedCommands(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	nestedDir := filepath.Join(commandsDir, "frontend", "components")
	require.NoError(t, os.MkdirAll(nestedDir, 0o755))

	// Create nested namespaced command
	nestedCmd := filepath.Join(nestedDir, "button.md")
	require.NoError(t, os.WriteFile(nestedCmd, []byte(`---
description: Button component
---
# Button
`), 0o644))

	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Test finding nested namespaced command
	cmd, err := registry.FindCommand("frontend:components:button")
	require.NoError(t, err)
	assert.Equal(t, "frontend:components:button", cmd.Name)
	assert.Equal(t, "frontend:components", cmd.Namespace)
	assert.Equal(t, "project:frontend:components", cmd.Source)
}

func TestRegistry_ListCommands(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create multiple commands
	cmd1 := filepath.Join(commandsDir, "cmd1.md")
	require.NoError(t, os.WriteFile(cmd1, []byte(`---
description: Command 1
---
# Cmd1
`), 0o644))

	cmd2 := filepath.Join(commandsDir, "cmd2.md")
	require.NoError(t, os.WriteFile(cmd2, []byte(`---
description: Command 2
---
# Cmd2
`), 0o644))

	nsDir := filepath.Join(commandsDir, "ns")
	require.NoError(t, os.MkdirAll(nsDir, 0o755))
	cmd3 := filepath.Join(nsDir, "cmd3.md")
	require.NoError(t, os.WriteFile(cmd3, []byte(`---
description: Command 3
---
# Cmd3
`), 0o644))

	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Test listing all commands
	commands := registry.ListCommands()
	require.Len(t, commands, 3)

	// Verify all commands are present
	cmdMap := make(map[string]Command)
	for _, cmd := range commands {
		cmdMap[cmd.Name] = cmd
	}

	assert.Contains(t, cmdMap, "cmd1")
	assert.Contains(t, cmdMap, "cmd2")
	assert.Contains(t, cmdMap, "ns:cmd3")

	// Verify ListCommands returns a copy (modifying shouldn't affect registry)
	commands[0].Name = "modified"
	// Re-fetch and verify original name still exists
	commands2 := registry.ListCommands()
	found := false
	for _, cmd := range commands2 {
		if cmd.Name == "cmd1" {
			found = true
			break
		}
	}
	assert.True(t, found, "Modifying returned slice should not affect registry")
}

func TestRegistry_ListCommands_EmptyRegistry(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	// Don't create any commands

	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	commands := registry.ListCommands()
	assert.Empty(t, commands)
}

func TestRegistry_FindCommand_ErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Test empty string
	_, err = registry.FindCommand("")
	assert.Error(t, err)

	// Test invalid namespace format (but valid name)
	_, err = registry.FindCommand("invalid:command:that:does:not:exist")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command not found")
}

func TestRegistry_Reload(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create initial command
	cmd1 := filepath.Join(commandsDir, "cmd1.md")
	require.NoError(t, os.WriteFile(cmd1, []byte(`---
description: Command 1
---
# Cmd1
`), 0o644))

	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Verify initial command exists
	cmd, err := registry.FindCommand("cmd1")
	require.NoError(t, err)
	assert.Equal(t, "Command 1", cmd.Description)

	// Add a new command file
	cmd2 := filepath.Join(commandsDir, "cmd2.md")
	require.NoError(t, os.WriteFile(cmd2, []byte(`---
description: Command 2
---
# Cmd2
`), 0o644))

	// Reload registry
	err = registry.Reload()
	require.NoError(t, err)

	// Verify both commands exist
	cmd, err = registry.FindCommand("cmd1")
	require.NoError(t, err)
	assert.Equal(t, "Command 1", cmd.Description)

	cmd, err = registry.FindCommand("cmd2")
	require.NoError(t, err)
	assert.Equal(t, "Command 2", cmd.Description)

	// Verify ListCommands returns both
	commands := registry.ListCommands()
	assert.Len(t, commands, 2)
}

func TestRegistry_Reload_ModifiedCommand(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create initial command
	cmdFile := filepath.Join(commandsDir, "test.md")
	require.NoError(t, os.WriteFile(cmdFile, []byte(`---
description: Original description
---
# Original
`), 0o644))

	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Verify original
	cmd, err := registry.FindCommand("test")
	require.NoError(t, err)
	assert.Equal(t, "Original description", cmd.Description)

	// Modify the command file
	require.NoError(t, os.WriteFile(cmdFile, []byte(`---
description: Updated description
---
# Updated
`), 0o644))

	// Reload
	err = registry.Reload()
	require.NoError(t, err)

	// Verify updated
	cmd, err = registry.FindCommand("test")
	require.NoError(t, err)
	assert.Equal(t, "Updated description", cmd.Description)
	assert.Contains(t, cmd.Content, "# Updated")
}

func TestRegistry_Reload_RemovedCommand(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create two commands
	cmd1 := filepath.Join(commandsDir, "cmd1.md")
	require.NoError(t, os.WriteFile(cmd1, []byte(`---
description: Command 1
---
# Cmd1
`), 0o644))

	cmd2 := filepath.Join(commandsDir, "cmd2.md")
	require.NoError(t, os.WriteFile(cmd2, []byte(`---
description: Command 2
---
# Cmd2
`), 0o644))

	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Verify both exist
	_, err = registry.FindCommand("cmd1")
	require.NoError(t, err)
	_, err = registry.FindCommand("cmd2")
	require.NoError(t, err)

	// Remove one command
	require.NoError(t, os.Remove(cmd2))

	// Reload
	err = registry.Reload()
	require.NoError(t, err)

	// Verify cmd1 still exists
	_, err = registry.FindCommand("cmd1")
	require.NoError(t, err)

	// Verify cmd2 is gone
	_, err = registry.FindCommand("cmd2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command not found")

	// Verify ListCommands only returns cmd1
	commands := registry.ListCommands()
	assert.Len(t, commands, 1)
	assert.Equal(t, "cmd1", commands[0].Name)
}

func TestRegistry_Reload_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create initial command
	cmdFile := filepath.Join(commandsDir, "test.md")
	require.NoError(t, os.WriteFile(cmdFile, []byte(`---
description: Test
---
# Test
`), 0o644))

	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Verify command exists
	_, err = registry.FindCommand("test")
	require.NoError(t, err)

	// Remove all commands
	require.NoError(t, os.Remove(cmdFile))

	// Reload
	err = registry.Reload()
	require.NoError(t, err)

	// Verify registry is empty
	commands := registry.ListCommands()
	assert.Empty(t, commands)

	_, err = registry.FindCommand("test")
	assert.Error(t, err)
}

