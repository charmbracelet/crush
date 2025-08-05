package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/config"
)

func TestDefaultKeyMap(t *testing.T) {
	keyMap := DefaultKeyMap()

	tests := []struct {
		name     string
		binding  key.Binding
		expected string
	}{
		{"quit", keyMap.Quit, "ctrl+c"},
		{"help", keyMap.Help, "ctrl+g"},
		{"commands", keyMap.Commands, "ctrl+p"},
		{"suspend", keyMap.Suspend, "ctrl+z"},
		{"sessions", keyMap.Sessions, "ctrl+s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.binding.Keys()) == 0 {
				t.Errorf("Expected %s to have keys, but got empty", tt.name)
				return
			}
			if tt.binding.Keys()[0] != tt.expected {
				t.Errorf("Expected %s key to be %s, got %s", tt.name, tt.expected, tt.binding.Keys()[0])
			}
		})
	}
}

func TestNewKeyMapWithCustom_NilKeymaps(t *testing.T) {
	keyMap := NewKeyMapWithCustom(nil)
	defaultKeyMap := DefaultKeyMap()

	tests := []struct {
		name     string
		custom   key.Binding
		default_ key.Binding
	}{
		{"quit", keyMap.Quit, defaultKeyMap.Quit},
		{"help", keyMap.Help, defaultKeyMap.Help},
		{"commands", keyMap.Commands, defaultKeyMap.Commands},
		{"suspend", keyMap.Suspend, defaultKeyMap.Suspend},
		{"sessions", keyMap.Sessions, defaultKeyMap.Sessions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.custom.Keys()) == 0 || len(tt.default_.Keys()) == 0 {
				t.Errorf("Expected both custom and default %s to have keys", tt.name)
				return
			}
			if tt.custom.Keys()[0] != tt.default_.Keys()[0] {
				t.Errorf("Expected custom %s key to match default %s, got %s vs %s",
					tt.name, tt.name, tt.custom.Keys()[0], tt.default_.Keys()[0])
			}
		})
	}
}

func TestNewKeyMapWithCustom_EmptyKeymaps(t *testing.T) {
	customKeymaps := make(config.KeyMaps)
	keyMap := NewKeyMapWithCustom(customKeymaps)
	defaultKeyMap := DefaultKeyMap()

	if keyMap.Quit.Keys()[0] != defaultKeyMap.Quit.Keys()[0] {
		t.Errorf("Expected quit key to remain default when empty keymaps provided")
	}
}

func TestNewKeyMapWithCustom_PartialOverride(t *testing.T) {
	customKeymaps := config.KeyMaps{
		config.CommandHelp:     "?",
		config.CommandCommands: "ctrl+k",
	}
	keyMap := NewKeyMapWithCustom(customKeymaps)

	tests := []struct {
		name     string
		binding  key.Binding
		expected string
	}{
		{"quit", keyMap.Quit, "ctrl+c"},         // should remain default
		{"help", keyMap.Help, "?"},              // should be custom
		{"commands", keyMap.Commands, "ctrl+k"}, // should be custom
		{"suspend", keyMap.Suspend, "ctrl+z"},   // should remain default
		{"sessions", keyMap.Sessions, "ctrl+s"}, // should remain default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.binding.Keys()) == 0 {
				t.Errorf("Expected %s to have keys, but got empty", tt.name)
				return
			}
			if tt.binding.Keys()[0] != tt.expected {
				t.Errorf("Expected %s key to be %s, got %s", tt.name, tt.expected, tt.binding.Keys()[0])
			}
		})
	}
}

func TestNewKeyMapWithCustom_FullOverride(t *testing.T) {
	customKeymaps := config.KeyMaps{
		config.CommandQuit:     "ctrl+q",
		config.CommandHelp:     "h",
		config.CommandCommands: "ctrl+space",
		config.CommandSuspend:  "ctrl+j",
		config.CommandSessions: "ctrl+l",
	}
	keyMap := NewKeyMapWithCustom(customKeymaps)

	tests := []struct {
		name     string
		binding  key.Binding
		expected string
	}{
		{"quit", keyMap.Quit, "ctrl+q"},
		{"help", keyMap.Help, "h"},
		{"commands", keyMap.Commands, "ctrl+space"},
		{"suspend", keyMap.Suspend, "ctrl+j"},
		{"sessions", keyMap.Sessions, "ctrl+l"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.binding.Keys()) == 0 {
				t.Errorf("Expected %s to have keys, but got empty", tt.name)
				return
			}
			if tt.binding.Keys()[0] != tt.expected {
				t.Errorf("Expected %s key to be %s, got %s", tt.name, tt.expected, tt.binding.Keys()[0])
			}
		})
	}
}

func TestNewKeyMapWithCustom_HelpMessages(t *testing.T) {
	customKeymaps := config.KeyMaps{
		config.CommandHelp:     "?",
		config.CommandCommands: "ctrl+k",
	}
	keyMap := NewKeyMapWithCustom(customKeymaps)

	tests := []struct {
		name         string
		binding      key.Binding
		expectedKey  string
		expectedHelp string
	}{
		{"help", keyMap.Help, "?", "more"},
		{"commands", keyMap.Commands, "ctrl+k", "commands"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.binding.Keys()) == 0 {
				t.Errorf("Expected %s to have keys, but got empty", tt.name)
				return
			}
			if tt.binding.Keys()[0] != tt.expectedKey {
				t.Errorf("Expected %s key to be %s, got %s", tt.name, tt.expectedKey, tt.binding.Keys()[0])
			}
			if tt.binding.Help().Desc != tt.expectedHelp {
				t.Errorf("Expected %s help to be %s, got %s", tt.name, tt.expectedHelp, tt.binding.Help().Desc)
			}
		})
	}
}

func TestKeyMap_HelpInterface(t *testing.T) {
	customKeymaps := config.KeyMaps{
		config.CommandHelp:     "?",
		config.CommandCommands: "ctrl+k",
	}
	keyMap := NewKeyMapWithCustom(customKeymaps)

	// Add mock page bindings that simulate real page help including matching descriptions
	mockPageBinding := key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit"))
	pageCommandsBinding := key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "commands")) // default "commands" desc
	pageHelpBinding := key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("ctrl+g", "more"))         // default "more" desc
	pageQuitBinding := key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit"))         // default "quit" desc
	keyMap.pageBindings = []key.Binding{mockPageBinding, pageCommandsBinding, pageHelpBinding, pageQuitBinding}

	// Test ShortHelp merges correctly
	shortHelp := keyMap.ShortHelp()
	if len(shortHelp) != 6 { // enter + commands + more + quit + sessions + suspend
		t.Errorf("Expected 6 bindings in ShortHelp, got %d", len(shortHelp))
	}

	// Test that custom keymaps replace defaults with same description
	foundCustomHelp := false
	foundDefaultHelp := false
	for _, binding := range shortHelp {
		if binding.Help().Desc == "more" {
			if len(binding.Keys()) > 0 && binding.Keys()[0] == "?" {
				foundCustomHelp = true
			} else if len(binding.Keys()) > 0 && binding.Keys()[0] == "ctrl+g" {
				foundDefaultHelp = true
			}
		}
	}
	if !foundCustomHelp {
		t.Error("Custom help key '?' with desc 'more' not found in ShortHelp output")
	}
	if foundDefaultHelp {
		t.Error("Default help key 'ctrl+g' should be replaced by custom key, but was found")
	}

	foundCustomCommands := false
	foundDefaultCommands := false
	for _, binding := range shortHelp {
		if binding.Help().Desc == "commands" {
			if len(binding.Keys()) > 0 && binding.Keys()[0] == "ctrl+k" {
				foundCustomCommands = true
			} else if len(binding.Keys()) > 0 && binding.Keys()[0] == "ctrl+p" {
				foundDefaultCommands = true
			}
		}
	}
	if !foundCustomCommands {
		t.Error("Custom commands key 'ctrl+k' with desc 'commands' not found in ShortHelp output")
	}
	if foundDefaultCommands {
		t.Error("Default commands key 'ctrl+p' should be replaced by custom key, but was found")
	}

	// Test that quit uses default (not customized)
	foundDefaultQuit := false
	for _, binding := range shortHelp {
		if binding.Help().Desc == "quit" && len(binding.Keys()) > 0 && binding.Keys()[0] == "ctrl+c" {
			foundDefaultQuit = true
			break
		}
	}
	if !foundDefaultQuit {
		t.Error("Default quit key 'ctrl+c' should remain as is")
	}

	// Test that non-matching page binding is included
	foundEnter := false
	for _, binding := range shortHelp {
		if len(binding.Keys()) > 0 && binding.Keys()[0] == "enter" && binding.Help().Desc == "submit" {
			foundEnter = true
			break
		}
	}
	if !foundEnter {
		t.Error("Non-matching page binding 'enter' should be included")
	}

	// Test FullHelp structure
	fullHelp := keyMap.FullHelp()
	if len(fullHelp) != 1 {
		t.Errorf("Expected 1 group in FullHelp, got %d", len(fullHelp))
	}
	if len(fullHelp[0]) != len(shortHelp) {
		t.Errorf("Expected FullHelp to match ShortHelp length, got %d vs %d", len(fullHelp[0]), len(shortHelp))
	}
}
