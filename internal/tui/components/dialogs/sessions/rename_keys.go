package sessions

import (
	"github.com/charmbracelet/bubbles/v2/key"
)

type SessionRenameKeys struct {
	Confirm,
	Close key.Binding
}

func SessionRenameKeyMap() SessionRenameKeys {
	return SessionRenameKeys{
		Confirm: key.NewBinding(
			key.WithKeys("enter", "tab", "ctrl+y"),
			key.WithHelp("enter", "confirm"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// KeyBindings implements layout.KeyMapProvider
func (k SessionRenameKeys) KeyBindings() []key.Binding {
	return []key.Binding{
		k.Confirm,
		k.Close,
	}
}

// FullHelp implements help.KeyMap.
func (k SessionRenameKeys) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := k.KeyBindings()
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// ShortHelp implements help.KeyMap.
func (k SessionRenameKeys) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Confirm,
		k.Close,
	}
}
