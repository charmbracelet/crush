package sessions

import (
	"github.com/charmbracelet/bubbles/v2/key"
)

type SessionsListKeyMap struct {
	Select,
	Next,
	Previous,
	Rename,
	Delete,
	DeleteAll,
	Close key.Binding
}

func SessionsKeyMap() SessionsListKeyMap {
	return SessionsListKeyMap{
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
		Rename: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "rename"),
		),
		Delete: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "delete"),
		),
		DeleteAll: key.NewBinding(
			key.WithKeys("ctrl+x"),
			key.WithHelp("ctrl+x", "delete all"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// KeyBindings implements layout.KeyMapProvider
func (k SessionsListKeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.Select,
		k.Next,
		k.Previous,
		k.Rename,
		k.Delete,
		k.DeleteAll,
		k.Close,
	}
}

// FullHelp implements help.KeyMap.
func (k SessionsListKeyMap) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := k.KeyBindings()
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// ShortHelp implements help.KeyMap.
func (k SessionsListKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(

			key.WithKeys("down", "up"),
			key.WithHelp("↑↓", "choose"),
		),
		k.Select,
		k.Rename,
		k.Delete,
		k.DeleteAll,
		k.Close,
	}
}
