package tui

import (
	"github.com/charmbracelet/bubbles/v2/key"
)

type KeyMap struct {
	Logs     key.Binding
	Quit     key.Binding
	Help     key.Binding
	Commands key.Binding
	Sessions key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Logs: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("ctrl+l", "logs"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),

		Help: key.NewBinding(
			key.WithKeys("ctrl+_"),
			key.WithHelp("ctrl+?", "toggle help"),
		),
		Commands: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "commands"),
		),
		Sessions: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "sessions"),
		),
	}
}

// FullHelp implements help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := k.KeyBindings()
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// KeyBindings implements layout.KeyMapProvider
func (k KeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.Logs,
		k.Quit,
		k.Help,
		k.Commands,
		k.Sessions,
	}
}

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{}
}
