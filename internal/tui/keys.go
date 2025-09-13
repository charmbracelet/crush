package tui

import (
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/config"
)

type KeyMap struct {
	Quit     key.Binding
	Help     key.Binding
	Commands key.Binding
	Suspend  key.Binding
	Sessions key.Binding

	pageBindings []key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	appBindings := []key.Binding{
		k.Quit,
		k.Help,
		k.Commands,
		k.Sessions,
		k.Suspend,
	}

	// Create a map of descriptions to app bindings for quick lookup
	appBindingByDesc := make(map[string]key.Binding, len(appBindings))
	for _, binding := range appBindings {
		appBindingByDesc[binding.Help().Desc] = binding
	}

	// Start with page bindings, replacing any that conflict with app bindings
	result := make([]key.Binding, 0, len(k.pageBindings)+len(appBindings))
	for _, pageBinding := range k.pageBindings {
		if appBinding, hasConflict := appBindingByDesc[pageBinding.Help().Desc]; hasConflict {
			// Use app binding instead of page binding
			result = append(result, appBinding)
			delete(appBindingByDesc, pageBinding.Help().Desc) // Mark as used
		} else {
			// No conflict, keep page binding
			result = append(result, pageBinding)
		}
	}

	// Add any remaining app bindings that weren't used to resolve conflicts
	for _, binding := range appBindingByDesc {
		result = append(result, binding)
	}

	return result
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		k.ShortHelp(),
	}
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("ctrl+g", "more"),
		),
		Commands: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "commands"),
		),
		Suspend: key.NewBinding(
			key.WithKeys("ctrl+z"),
			key.WithHelp("ctrl+z", "suspend"),
		),
		Sessions: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "sessions"),
		),
	}
}

func NewKeyMapWithCustom(customKeymaps config.KeyMaps) KeyMap {
	keyMap := DefaultKeyMap()

	if customKeymaps == nil {
		return keyMap
	}

	if quitKey, ok := customKeymaps[config.CommandQuit]; ok {
		keyMap.Quit = key.NewBinding(
			key.WithKeys(string(quitKey)),
			key.WithHelp(string(quitKey), "quit"),
		)
	}

	if helpKey, ok := customKeymaps[config.CommandHelp]; ok {
		keyMap.Help = key.NewBinding(
			key.WithKeys(string(helpKey)),
			key.WithHelp(string(helpKey), "more"),
		)
	}

	if commandsKey, ok := customKeymaps[config.CommandCommands]; ok {
		keyMap.Commands = key.NewBinding(
			key.WithKeys(string(commandsKey)),
			key.WithHelp(string(commandsKey), "commands"),
		)
	}

	if suspendKey, ok := customKeymaps[config.CommandSuspend]; ok {
		keyMap.Suspend = key.NewBinding(
			key.WithKeys(string(suspendKey)),
			key.WithHelp(string(suspendKey), "suspend"),
		)
	}

	if sessionsKey, ok := customKeymaps[config.CommandSessions]; ok {
		keyMap.Sessions = key.NewBinding(
			key.WithKeys(string(sessionsKey)),
			key.WithHelp(string(sessionsKey), "sessions"),
		)
	}

	return keyMap
}
