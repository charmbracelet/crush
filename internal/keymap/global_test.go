package keymap

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
)

func TestInitializeGlobalKeyMap(t *testing.T) {
	// Test with custom keymaps
	customKeymaps := config.KeyMaps{
		config.CommandQuit:     "ctrl+q",
		config.CommandHelp:     "?",
		config.CommandCommands: "ctrl+k",
	}

	InitializeGlobalKeyMap(customKeymaps)

	// Test that custom keymaps are applied
	if len(GlobalKeyBindings.Quit.Keys()) == 0 || GlobalKeyBindings.Quit.Keys()[0] != "ctrl+q" {
		t.Errorf("Expected quit key to be 'ctrl+q', got %v", GlobalKeyBindings.Quit.Keys())
	}

	if len(GlobalKeyBindings.Help.Keys()) == 0 || GlobalKeyBindings.Help.Keys()[0] != "?" {
		t.Errorf("Expected help key to be '?', got %v", GlobalKeyBindings.Help.Keys())
	}

	if len(GlobalKeyBindings.Commands.Keys()) == 0 || GlobalKeyBindings.Commands.Keys()[0] != "ctrl+k" {
		t.Errorf("Expected commands key to be 'ctrl+k', got %v", GlobalKeyBindings.Commands.Keys())
	}

	// Test that defaults are used for unspecified keys
	if len(GlobalKeyBindings.Sessions.Keys()) == 0 || GlobalKeyBindings.Sessions.Keys()[0] != "ctrl+s" {
		t.Errorf("Expected sessions key to be 'ctrl+s' (default), got %v", GlobalKeyBindings.Sessions.Keys())
	}
}

func TestInitializeGlobalKeyMapWithDefaults(t *testing.T) {
	// Test with nil keymaps (should use defaults)
	InitializeGlobalKeyMap(nil)

	// Test that all defaults are applied
	tests := []struct {
		name     string
		binding  string
		expected string
	}{
		{"quit", GetGlobalQuitKey(), "ctrl+c"},
		{"help", GetGlobalHelpKey(), "ctrl+g"},
		{"commands", GetGlobalCommandsKey(), "ctrl+p"},
		{"sessions", GetGlobalSessionsKey(), "ctrl+s"},
		{"suspend", GetGlobalSuspendKey(), "ctrl+z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.binding != tt.expected {
				t.Errorf("Expected %s key to be %s, got %s", tt.name, tt.expected, tt.binding)
			}
		})
	}
}

func TestGlobalKeyGetters(t *testing.T) {
	// Set up custom keymaps
	customKeymaps := config.KeyMaps{
		config.CommandHelp:     "?",
		config.CommandCommands: "ctrl+k",
	}
	InitializeGlobalKeyMap(customKeymaps)

	// Test getter functions
	if GetGlobalHelpKey() != "?" {
		t.Errorf("Expected GetGlobalHelpKey() to return '?', got '%s'", GetGlobalHelpKey())
	}

	if GetGlobalCommandsKey() != "ctrl+k" {
		t.Errorf("Expected GetGlobalCommandsKey() to return 'ctrl+k', got '%s'", GetGlobalCommandsKey())
	}

	// Test that unspecified keys return defaults
	if GetGlobalQuitKey() != "ctrl+c" {
		t.Errorf("Expected GetGlobalQuitKey() to return 'ctrl+c' (default), got '%s'", GetGlobalQuitKey())
	}
}
