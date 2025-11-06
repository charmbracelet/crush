package commands

import (
	"charm.land/bubbles/v2/key"
)

type CommandsDialogKeyMap struct {
	Select,
	Next,
	Previous,
	Tab,
	Close key.Binding
}

func DefaultCommandsDialogKeyMap() CommandsDialogKeyMap {
	return CommandsDialogKeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter", "ctrl+y"),
			key.WithHelp("enter", "confirm"),
		),
		Next: key.NewBinding(
			key.WithKeys("down", "ctrl+n"),
			key.WithHelp("↓", "next item"),
		),
		Previous: key.NewBinding(
			key.WithKeys("up", "ctrl+p"),
			key.WithHelp("↑", "previous item"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch selection"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// KeyBindings implements layout.KeyMapProvider
func (k CommandsDialogKeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.Select,
		k.Next,
		k.Previous,
		k.Tab,
		k.Close,
	}
}

// FullHelp implements help.KeyMap.
func (k CommandsDialogKeyMap) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := k.KeyBindings()
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// ShortHelp implements help.KeyMap.
func (k CommandsDialogKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Tab,
		key.NewBinding(
			key.WithKeys("down", "up"),
			key.WithHelp("↑↓", "choose"),
		),
		k.Select,
		k.Close,
	}
}

type ArgumentsDialogKeyMap struct {
	Confirm  key.Binding
	Next     key.Binding
	Previous key.Binding
	Close    key.Binding
}

func DefaultArgumentsDialogKeyMap() ArgumentsDialogKeyMap {
	return ArgumentsDialogKeyMap{
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),

		Next: key.NewBinding(
			key.WithKeys("tab", "down"),
			key.WithHelp("tab/↓", "next"),
		),
		Previous: key.NewBinding(
			key.WithKeys("shift+tab", "up"),
			key.WithHelp("shift+tab/↑", "previous"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// KeyBindings implements layout.KeyMapProvider
func (k ArgumentsDialogKeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.Confirm,
		k.Next,
		k.Previous,
		k.Close,
	}
}

// FullHelp implements help.KeyMap.
func (k ArgumentsDialogKeyMap) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := k.KeyBindings()
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// ShortHelp implements help.KeyMap.
func (k ArgumentsDialogKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Confirm,
		k.Next,
		k.Previous,
		k.Close,
	}
}
