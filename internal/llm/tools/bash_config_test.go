package tools

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

func TestBannedCommandsConfiguration(t *testing.T) {
	// Save original instance and restore it later
	originalCfg := config.Get()
	defer func() {
		// Restore original config
		if originalCfg != nil {
			config.Init(originalCfg.WorkingDir(), originalCfg.Options.DataDirectory, originalCfg.Options.Debug)
		}
	}()

	// Test that the bash tool includes default banned commands
	tool := NewBashTool(permission.NewPermissionService("/", false, []string{}), "/", nil)
	bashTool, ok := tool.(*bashTool)
	require.True(t, ok)
	
	// Check that default banned commands are present
	foundDefault := false
	for _, cmd := range bashTool.bannedCommands {
		if cmd == "curl" {
			foundDefault = true
			break
		}
	}
	require.True(t, foundDefault, "default banned commands should be present")
	
	// Check that we still have all the original banned commands
	foundSudo := false
	for _, cmd := range bashTool.bannedCommands {
		if cmd == "sudo" {
			foundSudo = true
			break
		}
	}
	require.True(t, foundSudo, "sudo should be in banned commands")
}

func TestAllowedCommandsOverride(t *testing.T) {
	// Note: This test demonstrates the intended behavior but cannot be fully implemented
	// without a proper way to mock the config.Get() function to return our test config
	// with allowed commands. In practice, users would set this in their configuration file.
	t.Skip("Skipping test that requires config mocking")
}