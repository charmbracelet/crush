package tui

import (
	"charm.land/bubbles/v2/key"
)

type KeyMap struct {
	Quit     key.Binding
	Help     key.Binding
	Commands key.Binding
	Suspend  key.Binding
	Models   key.Binding
	Sessions key.Binding

	pageBindings []key.Binding
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
		Models: key.NewBinding(
			key.WithKeys("ctrl+l", "ctrl+m"),
			key.WithHelp("ctrl+l", "models"),
		),
		Sessions: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "sessions"),
		),
	}
}
