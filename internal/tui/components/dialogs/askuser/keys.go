package askuser

import (
	"charm.land/bubbles/v2/key"
)

// KeyMap defines the keybindings for the ask user dialog.
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Toggle  key.Binding
	Select  key.Binding
	Other   key.Binding
	Cancel  key.Binding
	Confirm key.Binding
}

// DefaultKeyMap returns the default keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "prev option"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "next option"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "prev question"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "next question"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Other: key.NewBinding(
			key.WithKeys("o", "O"),
			key.WithHelp("o", "other"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("ctrl+enter"),
			key.WithHelp("ctrl+enter", "submit all"),
		),
	}
}

// KeyBindings returns all keybindings for help display.
func (k KeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
		k.Toggle,
		k.Select,
		k.Other,
		k.Cancel,
	}
}

// ShortHelp returns keybindings for compact help.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
		k.Select,
		k.Other,
		k.Cancel,
	}
}

// FullHelp returns keybindings for full help display.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.KeyBindings()}
}
