package keymap

import (
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/config"
)

// GlobalKeyBindings holds the merged app-level keybindings that are used throughout the app
var GlobalKeyBindings struct {
	Quit     key.Binding
	Help     key.Binding
	Commands key.Binding
	Sessions key.Binding
	Suspend  key.Binding
}

// InitializeGlobalKeyMap merges user custom keymaps with defaults and stores them globally
// This should be called once at app startup
func InitializeGlobalKeyMap(customKeymaps config.KeyMaps) {
	// Initialize with defaults, then override with custom keymaps
	GlobalKeyBindings.Quit = key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	)
	if quitKey, ok := customKeymaps["quit"]; ok {
		GlobalKeyBindings.Quit = key.NewBinding(
			key.WithKeys(string(quitKey)),
			key.WithHelp(string(quitKey), "quit"),
		)
	}

	GlobalKeyBindings.Help = key.NewBinding(
		key.WithKeys("ctrl+g"),
		key.WithHelp("ctrl+g", "more"),
	)
	if helpKey, ok := customKeymaps["help"]; ok {
		GlobalKeyBindings.Help = key.NewBinding(
			key.WithKeys(string(helpKey)),
			key.WithHelp(string(helpKey), "more"),
		)
	}

	GlobalKeyBindings.Commands = key.NewBinding(
		key.WithKeys("ctrl+p"),
		key.WithHelp("ctrl+p", "commands"),
	)
	if commandsKey, ok := customKeymaps["commands"]; ok {
		GlobalKeyBindings.Commands = key.NewBinding(
			key.WithKeys(string(commandsKey)),
			key.WithHelp(string(commandsKey), "commands"),
		)
	}

	GlobalKeyBindings.Sessions = key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "sessions"),
	)
	if sessionsKey, ok := customKeymaps["sessions"]; ok {
		GlobalKeyBindings.Sessions = key.NewBinding(
			key.WithKeys(string(sessionsKey)),
			key.WithHelp(string(sessionsKey), "sessions"),
		)
	}

	GlobalKeyBindings.Suspend = key.NewBinding(
		key.WithKeys("ctrl+z"),
		key.WithHelp("ctrl+z", "suspend"),
	)
	if suspendKey, ok := customKeymaps["suspend"]; ok {
		GlobalKeyBindings.Suspend = key.NewBinding(
			key.WithKeys(string(suspendKey)),
			key.WithHelp(string(suspendKey), "suspend"),
		)
	}
}

// GetGlobalQuitKey returns the merged quit key as a string
func GetGlobalQuitKey() string {
	if len(GlobalKeyBindings.Quit.Keys()) > 0 {
		return GlobalKeyBindings.Quit.Keys()[0]
	}
	return "ctrl+c"
}

// GetGlobalHelpKey returns the merged help key as a string
func GetGlobalHelpKey() string {
	if len(GlobalKeyBindings.Help.Keys()) > 0 {
		return GlobalKeyBindings.Help.Keys()[0]
	}
	return "ctrl+g"
}

// GetGlobalCommandsKey returns the merged commands key as a string
func GetGlobalCommandsKey() string {
	if len(GlobalKeyBindings.Commands.Keys()) > 0 {
		return GlobalKeyBindings.Commands.Keys()[0]
	}
	return "ctrl+p"
}

// GetGlobalSessionsKey returns the merged sessions key as a string
func GetGlobalSessionsKey() string {
	if len(GlobalKeyBindings.Sessions.Keys()) > 0 {
		return GlobalKeyBindings.Sessions.Keys()[0]
	}
	return "ctrl+s"
}

// GetGlobalSuspendKey returns the merged suspend key as a string
func GetGlobalSuspendKey() string {
	if len(GlobalKeyBindings.Suspend.Keys()) > 0 {
		return GlobalKeyBindings.Suspend.Keys()[0]
	}
	return "ctrl+z"
}