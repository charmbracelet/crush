package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadProjectCommands_BasicLoading(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	// Create .crush/commands directory structure
	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create a root-level command
	rootCmd := filepath.Join(commandsDir, "test-command.md")
	require.NoError(t, os.WriteFile(rootCmd, []byte(`---
description: Test command
---
# Test Command

This is a test command.
`), 0o644))

	commands, err := loadProjectCommands(projectDir)

	require.NoError(t, err)
	require.Len(t, commands, 1)
	assert.Equal(t, "test-command", commands[0].Name)
	assert.Equal(t, "", commands[0].Namespace)
	assert.Equal(t, "Test command", commands[0].Description)
	assert.Equal(t, "project", commands[0].Source)
	assert.Contains(t, commands[0].Content, "# Test Command")
}

func TestLoadProjectCommands_SubdirectoryNamespacing(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	// Create nested command structure
	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	frontendDir := filepath.Join(commandsDir, "frontend")
	componentsDir := filepath.Join(frontendDir, "components")
	require.NoError(t, os.MkdirAll(componentsDir, 0o755))

	// Create root command
	rootCmd := filepath.Join(commandsDir, "root.md")
	require.NoError(t, os.WriteFile(rootCmd, []byte(`---
description: Root command
---
# Root
`), 0o644))

	// Create namespaced command
	nsCmd := filepath.Join(frontendDir, "review.md")
	require.NoError(t, os.WriteFile(nsCmd, []byte(`---
description: Frontend review
---
# Review
`), 0o644))

	// Create nested namespaced command
	nestedCmd := filepath.Join(componentsDir, "button.md")
	require.NoError(t, os.WriteFile(nestedCmd, []byte(`---
description: Button component
---
# Button
`), 0o644))

	commands, err := loadProjectCommands(projectDir)

	require.NoError(t, err)
	require.Len(t, commands, 3)

	// Find commands by name
	cmdMap := make(map[string]Command)
	for _, cmd := range commands {
		cmdMap[cmd.Name] = cmd
	}

	// Verify root command
	root, exists := cmdMap["root"]
	require.True(t, exists)
	assert.Equal(t, "", root.Namespace)
	assert.Equal(t, "project", root.Source)

	// Verify single-level namespace
	frontend, exists := cmdMap["frontend:review"]
	require.True(t, exists)
	assert.Equal(t, "frontend", frontend.Namespace)
	assert.Equal(t, "project:frontend", frontend.Source)

	// Verify nested namespace
	button, exists := cmdMap["frontend:components:button"]
	require.True(t, exists)
	assert.Equal(t, "frontend:components", button.Namespace)
	assert.Equal(t, "project:frontend:components", button.Source)
}

func TestLoadProjectCommands_MissingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	// Don't create .crush/commands directory

	commands, err := loadProjectCommands(projectDir)

	require.NoError(t, err)
	assert.Empty(t, commands)
}

func TestLoadProjectCommands_InvalidFilesIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create .md file (should be loaded)
	validCmd := filepath.Join(commandsDir, "valid.md")
	require.NoError(t, os.WriteFile(validCmd, []byte(`---
description: Valid
---
# Valid
`), 0o644))

	// Create non-.md file (should be ignored)
	invalidFile := filepath.Join(commandsDir, "invalid.txt")
	require.NoError(t, os.WriteFile(invalidFile, []byte("not a command"), 0o644))

	// Create directory (should be skipped)
	subDir := filepath.Join(commandsDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0o755))

	commands, err := loadProjectCommands(projectDir)

	require.NoError(t, err)
	require.Len(t, commands, 1)
	assert.Equal(t, "valid", commands[0].Name)
}

func TestLoadUserHomeCommands_BasicLoading(t *testing.T) {
	// home.Dir() is cached via sync.OnceValue, so we can't easily mock it.
	// Instead, test using the actual home directory for this test.
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set, skipping test")
	}

	// Create ~/.crush/commands directory in actual home
	commandsDir := filepath.Join(homeDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))
	defer os.RemoveAll(commandsDir) // Cleanup

	// Create a command file
	cmdFile := filepath.Join(commandsDir, "user-cmd.md")
	require.NoError(t, os.WriteFile(cmdFile, []byte(`---
description: User command
---
# User Command
`), 0o644))

	commands, err := loadUserHomeCommands()

	require.NoError(t, err)
	require.Len(t, commands, 1)
	assert.Equal(t, "user-cmd", commands[0].Name)
	assert.Equal(t, "user", commands[0].Source)
}

func TestLoadUserHomeCommands_Namespacing(t *testing.T) {
	// home.Dir() is cached via sync.OnceValue, so we can't easily mock it.
	// Instead, test using the actual home directory for this test.
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set, skipping test")
	}

	// Create nested structure in actual home
	commandsDir := filepath.Join(homeDir, ".crush", "commands")
	nsDir := filepath.Join(commandsDir, "custom")
	require.NoError(t, os.MkdirAll(nsDir, 0o755))
	defer os.RemoveAll(commandsDir) // Cleanup

	cmdFile := filepath.Join(nsDir, "test.md")
	require.NoError(t, os.WriteFile(cmdFile, []byte(`---
description: Custom command
---
# Custom
`), 0o644))

	commands, err := loadUserHomeCommands()

	require.NoError(t, err)
	require.Len(t, commands, 1)
	assert.Equal(t, "custom:test", commands[0].Name)
	assert.Equal(t, "custom", commands[0].Namespace)
	assert.Equal(t, "user:custom", commands[0].Source)
}

func TestLoadUserHomeCommands_MissingDirectory(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	// Don't create .crush/commands directory

	commands, err := loadUserHomeCommands()

	require.NoError(t, err)
	assert.Empty(t, commands)
}

func TestLoadXDGCommands_BasicLoading(t *testing.T) {
	// Save original XDG_CONFIG_HOME
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if originalXDG == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", originalXDG)
		}
	}()

	tmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create $XDG_CONFIG_HOME/crush/commands directory
	commandsDir := filepath.Join(tmpDir, "crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	cmdFile := filepath.Join(commandsDir, "xdg-cmd.md")
	require.NoError(t, os.WriteFile(cmdFile, []byte(`---
description: XDG command
---
# XDG Command
`), 0o644))

	commands, err := loadXDGCommands()

	require.NoError(t, err)
	require.Len(t, commands, 1)
	assert.Equal(t, "xdg-cmd", commands[0].Name)
	assert.Equal(t, "user", commands[0].Source)
}

func TestLoadXDGCommands_FallbackToDefaultConfig(t *testing.T) {
	// Save original XDG_CONFIG_HOME
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if originalXDG == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", originalXDG)
		}
	}()

	// Unset XDG_CONFIG_HOME to trigger fallback
	os.Unsetenv("XDG_CONFIG_HOME")

	// home.Dir() is cached, so use actual home directory
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set, skipping test")
	}

	// Create ~/.config/crush/commands directory
	configDir := filepath.Join(homeDir, ".config", "crush", "commands")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	defer os.RemoveAll(filepath.Join(homeDir, ".config", "crush")) // Cleanup

	cmdFile := filepath.Join(configDir, "fallback-cmd.md")
	require.NoError(t, os.WriteFile(cmdFile, []byte(`---
description: Fallback command
---
# Fallback
`), 0o644))

	commands, err := loadXDGCommands()

	require.NoError(t, err)
	require.Len(t, commands, 1)
	assert.Equal(t, "fallback-cmd", commands[0].Name)
}

func TestLoadXDGCommands_MissingDirectory(t *testing.T) {
	// Save original XDG_CONFIG_HOME
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if originalXDG == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", originalXDG)
		}
	}()

	tmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Don't create crush/commands directory

	commands, err := loadXDGCommands()

	require.NoError(t, err)
	assert.Empty(t, commands)
}

func TestLoadXDGCommands_Namespacing(t *testing.T) {
	// Save original XDG_CONFIG_HOME
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if originalXDG == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", originalXDG)
		}
	}()

	tmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmpDir)

	commandsDir := filepath.Join(tmpDir, "crush", "commands")
	nsDir := filepath.Join(commandsDir, "mcp")
	require.NoError(t, os.MkdirAll(nsDir, 0o755))

	cmdFile := filepath.Join(nsDir, "server.md")
	require.NoError(t, os.WriteFile(cmdFile, []byte(`---
description: MCP server command
---
# MCP Server
`), 0o644))

	commands, err := loadXDGCommands()

	require.NoError(t, err)
	require.Len(t, commands, 1)
	assert.Equal(t, "mcp:server", commands[0].Name)
	assert.Equal(t, "mcp", commands[0].Namespace)
	assert.Equal(t, "user:mcp", commands[0].Source)
}

func TestLoadProjectCommands_FrontmatterParsing(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir

	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	cmdFile := filepath.Join(commandsDir, "full-featured.md")
	require.NoError(t, os.WriteFile(cmdFile, []byte(`---
description: Full featured command
argument-hint: "[arg1] [arg2]"
allowed-tools:
  - view
  - edit
  - grep
---
# Full Featured

This command has all frontmatter fields.
`), 0o644))

	commands, err := loadProjectCommands(projectDir)

	require.NoError(t, err)
	require.Len(t, commands, 1)
	cmd := commands[0]
	assert.Equal(t, "Full featured command", cmd.Description)
	assert.Equal(t, "[arg1] [arg2]", cmd.ArgumentHint)
	assert.Equal(t, []string{"view", "edit", "grep"}, cmd.AllowedTools)
	assert.Contains(t, cmd.Content, "# Full Featured")
}

