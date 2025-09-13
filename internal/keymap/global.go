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

// InitializeGlobalKeyMap merges user custom keymaps with defaults and stores them for use while the app is running
func InitializeGlobalKeyMap(customKeymaps config.KeyMaps) {
	if quitKey, ok := customKeymaps[config.CommandQuit]; ok {
		GlobalKeyBindings.Quit = key.NewBinding(
			key.WithKeys(string(quitKey)),
			key.WithHelp(string(quitKey), "quit"),
		)
	} else {
		GlobalKeyBindings.Quit = key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		)
	}

	if helpKey, ok := customKeymaps[config.CommandHelp]; ok {
		GlobalKeyBindings.Help = key.NewBinding(
			key.WithKeys(string(helpKey)),
			key.WithHelp(string(helpKey), "more"),
		)
	} else {
		GlobalKeyBindings.Help = key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("ctrl+g", "more"),
		)
	}

	if commandsKey, ok := customKeymaps[config.CommandCommands]; ok {
		GlobalKeyBindings.Commands = key.NewBinding(
			key.WithKeys(string(commandsKey)),
			key.WithHelp(string(commandsKey), "commands"),
		)
	} else {
		GlobalKeyBindings.Commands = key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "commands"),
		)
	}

	if sessionsKey, ok := customKeymaps[config.CommandSessions]; ok {
		GlobalKeyBindings.Sessions = key.NewBinding(
			key.WithKeys(string(sessionsKey)),
			key.WithHelp(string(sessionsKey), "sessions"),
		)
	} else {
		GlobalKeyBindings.Sessions = key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "sessions"),
		)
	}

	if suspendKey, ok := customKeymaps[config.CommandSuspend]; ok {
		GlobalKeyBindings.Suspend = key.NewBinding(
			key.WithKeys(string(suspendKey)),
			key.WithHelp(string(suspendKey), "suspend"),
		)
	} else {
		GlobalKeyBindings.Suspend = key.NewBinding(
			key.WithKeys("ctrl+z"),
			key.WithHelp("ctrl+z", "suspend"),
		)
	}
}

// GetGlobalQuitKey returns the resolved quit keymap
func GetGlobalQuitKey() string {
	if len(GlobalKeyBindings.Quit.Keys()) > 0 {
		return GlobalKeyBindings.Quit.Keys()[0]
	}
	return "ctrl+c"
}

// GetGlobalHelpKey returns the resolved help keymap
func GetGlobalHelpKey() string {
	if len(GlobalKeyBindings.Help.Keys()) > 0 {
		return GlobalKeyBindings.Help.Keys()[0]
	}
	return "ctrl+g"
}

// GetGlobalCommandsKey returns the resolved commands keymap
func GetGlobalCommandsKey() string {
	if len(GlobalKeyBindings.Commands.Keys()) > 0 {
		return GlobalKeyBindings.Commands.Keys()[0]
	}
	return "ctrl+p"
}

// GetGlobalSessionsKey returns the session keymap
func GetGlobalSessionsKey() string {
	if len(GlobalKeyBindings.Sessions.Keys()) > 0 {
		return GlobalKeyBindings.Sessions.Keys()[0]
	}
	return "ctrl+s"
}

// GetGlobalSuspendKey returns the sus
func GetGlobalSuspendKey() string {
	if len(GlobalKeyBindings.Suspend.Keys()) > 0 {
		return GlobalKeyBindings.Suspend.Keys()[0]
	}
	return "ctrl+z"
}
