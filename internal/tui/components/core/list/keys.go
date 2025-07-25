package list

import (
	"github.com/charmbracelet/bubbles/v2/key"
)

type KeyMap struct {
	Up,
	Down,
	UpOneItem,
	DownOneItem,
	PageUp,
	PageDown,
	HalfPageUp,
	HalfPageDown,
	Home,
	End key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "f"),
			key.WithHelp("f/pgdn", "page down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("b/pgup", "page up"),
		),
		HalfPageUp: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "½ page up"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "½ page down"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		UpOneItem: key.NewBinding(
			key.WithKeys("shift+up"),
			key.WithHelp("shift+↑", "up one item"),
		),
		DownOneItem: key.NewBinding(
			key.WithKeys("shift+down"),
			key.WithHelp("shift+↓", "down one item"),
		),
		Home: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g/home", "home"),
		),
		End: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G/end", "end"),
		),
	}
}

// KeyBindings implements layout.KeyMapProvider
func (k KeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.Down,
		k.Up,
		k.DownOneItem,
		k.UpOneItem,
		k.PageDown,
		k.PageUp,
		k.HalfPageDown,
		k.HalfPageUp,
		k.Home,
		k.End,
	}
}
