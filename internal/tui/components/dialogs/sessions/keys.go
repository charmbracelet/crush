package sessions

import (
	"github.com/charmbracelet/bubbles/v2/key"
)

type KeyMap struct {
	Select,
	Next,
	Previous,
	Delete,
	Close,
	Yes,
	No,
	LeftRight,
	EnterSpace key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter", "tab", "ctrl+y"),
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
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete session"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Yes: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "yes"),
		),
		No: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "no"),
		),
		LeftRight: key.NewBinding(
			key.WithKeys("left", "right"),
			key.WithHelp("←→", "navigate"),
		),
		EnterSpace: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter", "confirm"),
		),
	}
}

// KeyBindings implements layout.KeyMapProvider
func (k KeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.Select,
		k.Next,
		k.Previous,
		k.Delete,
		k.Close,
		k.Yes,
		k.No,
		k.LeftRight,
		k.EnterSpace,
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

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(

			key.WithKeys("down", "up"),
			key.WithHelp("↑↓", "choose"),
		),
		k.Select,
		k.Delete,
		k.Close,
	}
}
