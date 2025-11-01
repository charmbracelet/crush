package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_FullCommandLoading(t *testing.T) {
	// Save original environment
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")
	defer func() {
		if originalXDG == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", originalXDG)
		}
		os.Setenv("HOME", originalHome)
	}()

	// Create temporary directories for all three locations
	tmpDir := t.TempDir()
	projectDir := tmpDir

	// Setup XDG_CONFIG_HOME
	xdgTmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", xdgTmpDir)

	// Setup HOME (use actual home, but create test directories)
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set, skipping integration test")
	}

	// Setup project commands directory
	projectCommandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(projectCommandsDir, 0o755))
	defer os.RemoveAll(projectCommandsDir)

	// Setup user home commands directory
	userCommandsDir := filepath.Join(homeDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(userCommandsDir, 0o755))
	defer os.RemoveAll(userCommandsDir)

	// Setup XDG commands directory
	xdgCommandsDir := filepath.Join(xdgTmpDir, "crush", "commands")
	require.NoError(t, os.MkdirAll(xdgCommandsDir, 0o755))

	// Create project command (root level)
	projectRootCmd := filepath.Join(projectCommandsDir, "project-root.md")
	require.NoError(t, os.WriteFile(projectRootCmd, []byte(`---
description: Project root command
---
# Project Root
`), 0o644))

	// Create project command with namespace
	projectNsDir := filepath.Join(projectCommandsDir, "project-ns")
	require.NoError(t, os.MkdirAll(projectNsDir, 0o755))
	projectNsCmd := filepath.Join(projectNsDir, "cmd.md")
	require.NoError(t, os.WriteFile(projectNsCmd, []byte(`---
description: Project namespaced command
---
# Project NS
`), 0o644))

	// Create user home command (root level)
	userRootCmd := filepath.Join(userCommandsDir, "user-root.md")
	require.NoError(t, os.WriteFile(userRootCmd, []byte(`---
description: User root command
---
# User Root
`), 0o644))

	// Create user home command with namespace
	userNsDir := filepath.Join(userCommandsDir, "user-ns")
	require.NoError(t, os.MkdirAll(userNsDir, 0o755))
	userNsCmd := filepath.Join(userNsDir, "cmd.md")
	require.NoError(t, os.WriteFile(userNsCmd, []byte(`---
description: User namespaced command
---
# User NS
`), 0o644))

	// Create XDG command (root level)
	xdgRootCmd := filepath.Join(xdgCommandsDir, "xdg-root.md")
	require.NoError(t, os.WriteFile(xdgRootCmd, []byte(`---
description: XDG root command
---
# XDG Root
`), 0o644))

	// Create XDG command with namespace
	xdgNsDir := filepath.Join(xdgCommandsDir, "xdg-ns")
	require.NoError(t, os.MkdirAll(xdgNsDir, 0o755))
	xdgNsCmd := filepath.Join(xdgNsDir, "cmd.md")
	require.NoError(t, os.WriteFile(xdgNsCmd, []byte(`---
description: XDG namespaced command
---
# XDG NS
`), 0o644))

	// Create registry and load commands
	registry := NewRegistry(projectDir)
	commands, err := registry.LoadCommands()
	require.NoError(t, err)

	// Verify all commands were loaded (6 total: 2 project + 2 user + 2 XDG)
	assert.Len(t, commands, 6)

	// Build command map for easier lookup
	cmdMap := make(map[string]Command)
	for _, cmd := range commands {
		cmdMap[cmd.Name] = cmd
	}

	// Verify project commands
	projectRoot, exists := cmdMap["project-root"]
	require.True(t, exists, "project-root command should exist")
	assert.Equal(t, "", projectRoot.Namespace)
	assert.Equal(t, "project", projectRoot.Source)
	assert.Equal(t, "Project root command", projectRoot.Description)

	projectNs, exists := cmdMap["project-ns:cmd"]
	require.True(t, exists, "project-ns:cmd command should exist")
	assert.Equal(t, "project-ns", projectNs.Namespace)
	assert.Equal(t, "project:project-ns", projectNs.Source)
	assert.Equal(t, "Project namespaced command", projectNs.Description)

	// Verify user home commands
	userRoot, exists := cmdMap["user-root"]
	require.True(t, exists, "user-root command should exist")
	assert.Equal(t, "", userRoot.Namespace)
	assert.Equal(t, "user", userRoot.Source)
	assert.Equal(t, "User root command", userRoot.Description)

	userNs, exists := cmdMap["user-ns:cmd"]
	require.True(t, exists, "user-ns:cmd command should exist")
	assert.Equal(t, "user-ns", userNs.Namespace)
	assert.Equal(t, "user:user-ns", userNs.Source)
	assert.Equal(t, "User namespaced command", userNs.Description)

	// Verify XDG commands
	xdgRoot, exists := cmdMap["xdg-root"]
	require.True(t, exists, "xdg-root command should exist")
	assert.Equal(t, "", xdgRoot.Namespace)
	assert.Equal(t, "user", xdgRoot.Source) // XDG commands use "user" source
	assert.Equal(t, "XDG root command", xdgRoot.Description)

	xdgNs, exists := cmdMap["xdg-ns:cmd"]
	require.True(t, exists, "xdg-ns:cmd command should exist")
	assert.Equal(t, "xdg-ns", xdgNs.Namespace)
	assert.Equal(t, "user:xdg-ns", xdgNs.Source)
	assert.Equal(t, "XDG namespaced command", xdgNs.Description)
}

func TestIntegration_NamespacingPreventsConflicts(t *testing.T) {
	// Save original environment
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")
	defer func() {
		if originalXDG == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", originalXDG)
		}
		os.Setenv("HOME", originalHome)
	}()

	tmpDir := t.TempDir()
	projectDir := tmpDir

	xdgTmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", xdgTmpDir)

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set, skipping integration test")
	}

	// Create commands with same filename but different namespaces
	projectCommandsDir := filepath.Join(projectDir, ".crush", "commands", "frontend")
	require.NoError(t, os.MkdirAll(projectCommandsDir, 0o755))
	defer os.RemoveAll(filepath.Join(projectDir, ".crush"))

	userCommandsDir := filepath.Join(homeDir, ".crush", "commands", "backend")
	require.NoError(t, os.MkdirAll(userCommandsDir, 0o755))
	defer os.RemoveAll(userCommandsDir)

	xdgCommandsDir := filepath.Join(xdgTmpDir, "crush", "commands", "api")
	require.NoError(t, os.MkdirAll(xdgCommandsDir, 0o755))

	// All have same filename "review.md" but different namespaces
	projectCmd := filepath.Join(projectCommandsDir, "review.md")
	require.NoError(t, os.WriteFile(projectCmd, []byte(`---
description: Frontend review
---
# Frontend Review
`), 0o644))

	userCmd := filepath.Join(userCommandsDir, "review.md")
	require.NoError(t, os.WriteFile(userCmd, []byte(`---
description: Backend review
---
# Backend Review
`), 0o644))

	xdgCmd := filepath.Join(xdgCommandsDir, "review.md")
	require.NoError(t, os.WriteFile(xdgCmd, []byte(`---
description: API review
---
# API Review
`), 0o644))

	// Load commands
	registry := NewRegistry(projectDir)
	commands, err := registry.LoadCommands()
	require.NoError(t, err)

	// Verify all three commands exist with different names
	assert.Len(t, commands, 3)

	cmdMap := make(map[string]Command)
	for _, cmd := range commands {
		cmdMap[cmd.Name] = cmd
	}

	// Verify namespacing prevents conflicts
	frontend, exists := cmdMap["frontend:review"]
	require.True(t, exists)
	assert.Equal(t, "frontend", frontend.Namespace)
	assert.Equal(t, "project:frontend", frontend.Source)
	assert.Equal(t, "Frontend review", frontend.Description)

	backend, exists := cmdMap["backend:review"]
	require.True(t, exists)
	assert.Equal(t, "backend", backend.Namespace)
	assert.Equal(t, "user:backend", backend.Source)
	assert.Equal(t, "Backend review", backend.Description)

	api, exists := cmdMap["api:review"]
	require.True(t, exists)
	assert.Equal(t, "api", api.Namespace)
	assert.Equal(t, "user:api", api.Source)
	assert.Equal(t, "API review", api.Description)
}

func TestIntegration_ProjectCommandsTakePrecedence(t *testing.T) {
	// Save original environment
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	tmpDir := t.TempDir()
	projectDir := tmpDir

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set, skipping integration test")
	}

	// Create command with same name in both project and user home
	projectCommandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(projectCommandsDir, 0o755))
	defer os.RemoveAll(projectCommandsDir)

	userCommandsDir := filepath.Join(homeDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(userCommandsDir, 0o755))
	defer os.RemoveAll(userCommandsDir)

	// Same filename in both locations
	projectCmd := filepath.Join(projectCommandsDir, "conflict.md")
	require.NoError(t, os.WriteFile(projectCmd, []byte(`---
description: Project version (should win)
---
# Project
`), 0o644))

	userCmd := filepath.Join(userCommandsDir, "conflict.md")
	require.NoError(t, os.WriteFile(userCmd, []byte(`---
description: User version (should lose)
---
# User
`), 0o644))

	// Load commands
	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Should only have one command after deduplication (project wins)
	commands := registry.ListCommands()
	assert.Len(t, commands, 1)

	cmd, err := registry.FindCommand("conflict")
	require.NoError(t, err)
	assert.Equal(t, "Project version (should win)", cmd.Description)
	assert.Equal(t, "project", cmd.Source)
	assert.Contains(t, cmd.Content, "# Project")
}

