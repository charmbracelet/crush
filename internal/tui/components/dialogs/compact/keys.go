package compact

import (
	"github.com/charmbracelet/bubbles/v2/key"
)

// KeyMap defines the key bindings for the compact dialog.
type KeyMap struct {
	ChangeSelection key.Binding
	Select          key.Binding
	Y               key.Binding
	N               key.Binding
	Close           key.Binding
}

// DefaultKeyMap returns the default key bindings for the compact dialog.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		ChangeSelection: key.NewBinding(
			key.WithKeys("tab", "left", "right", "h", "l"),
			key.WithHelp("tab/←/→", "toggle selection"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Y: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "yes"),
		),
		N: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "no"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// KeyBindings implements layout.KeyMapProvider
func (k KeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.ChangeSelection,
		k.Select,
		k.Y,
		k.N,
		k.Close,
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
		k.ChangeSelection,
		k.Select,
		k.Close,
	}
}
