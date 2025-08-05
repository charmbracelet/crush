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
	
	// Create map of app binding descriptions to custom bindings for replacement
	appBindingByDesc := make(map[string]key.Binding)
	for _, binding := range appBindings {
		appBindingByDesc[binding.Help().Desc] = binding
	}
	
	var mergedPageBindings []key.Binding
	for _, pageBinding := range k.pageBindings {
		desc := pageBinding.Help().Desc
		
		// Check if this page binding should be replaced with custom app binding
		if customBinding, exists := appBindingByDesc[desc]; exists {
			// Replace with the custom binding that has the same description
			mergedPageBindings = append(mergedPageBindings, customBinding)
			// Remove from app bindings to avoid duplicates later
			delete(appBindingByDesc, desc)
		} else {
			// Keep the page binding as-is (no conflict)
			mergedPageBindings = append(mergedPageBindings, pageBinding)
		}
	}
	
	// Add any remaining app bindings that weren't merged
	for _, binding := range appBindingByDesc {
		mergedPageBindings = append(mergedPageBindings, binding)
	}
	
	return mergedPageBindings
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
	
	if quitKey, ok := customKeymaps["quit"]; ok {
		keyMap.Quit = key.NewBinding(
			key.WithKeys(string(quitKey)),
			key.WithHelp(string(quitKey), "quit"),
		)
	}
	
	if helpKey, ok := customKeymaps["help"]; ok {
		keyMap.Help = key.NewBinding(
			key.WithKeys(string(helpKey)),
			key.WithHelp(string(helpKey), "more"),
		)
	}
	
	if commandsKey, ok := customKeymaps["commands"]; ok {
		keyMap.Commands = key.NewBinding(
			key.WithKeys(string(commandsKey)),
			key.WithHelp(string(commandsKey), "commands"),
		)
	}
	
	if suspendKey, ok := customKeymaps["suspend"]; ok {
		keyMap.Suspend = key.NewBinding(
			key.WithKeys(string(suspendKey)),
			key.WithHelp(string(suspendKey), "suspend"),
		)
	}
	
	if sessionsKey, ok := customKeymaps["sessions"]; ok {
		keyMap.Sessions = key.NewBinding(
			key.WithKeys(string(sessionsKey)),
			key.WithHelp(string(sessionsKey), "sessions"),
		)
	}
	
	return keyMap
}
