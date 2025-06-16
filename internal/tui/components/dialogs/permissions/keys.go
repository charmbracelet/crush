package permissions

import (
	"github.com/charmbracelet/bubbles/v2/key"
)

type KeyMap struct {
	Left,
	Right,
	Tab,
	Select,
	Allow,
	AllowSession,
	Deny key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←", "previous"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→", "next"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch"),
		),
		Allow: key.NewBinding(
			key.WithKeys("a", "ctrl+a"),
			key.WithHelp("a", "allow"),
		),
		AllowSession: key.NewBinding(
			key.WithKeys("s", "ctrl+s"),
			key.WithHelp("s", "allow session"),
		),
		Deny: key.NewBinding(
			key.WithKeys("d", "ctrl+d"),
			key.WithHelp("d", "deny"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", "tab", "ctrl+y"),
			key.WithHelp("enter", "confirm"),
		),
	}
}

// KeyBindings implements layout.KeyMapProvider
func (k KeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.Left,
		k.Right,
		k.Tab,
		k.Select,
		k.Allow,
		k.AllowSession,
		k.Deny,
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
		k.Allow,
		k.AllowSession,
		k.Deny,
		k.Select,
	}
}
