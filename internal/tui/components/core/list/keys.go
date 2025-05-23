package list

import "github.com/charmbracelet/bubbles/v2/key"

type KeyMap struct {
	Down,
	Up,
	NDown,
	NUp,
	DownOneItem,
	UpOneItem,
	HalfPageDown,
	HalfPageUp,
	Home,
	End,
	Submit key.Binding
}

func DefaultKeymap() KeyMap {
	return KeyMap{
		Down: key.NewBinding(
			key.WithKeys("down", "ctrl+j", "ctrl+n"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "ctrl+k", "ctrl+p"),
		),
		NDown: key.NewBinding(
			key.WithKeys("j"),
		),
		NUp: key.NewBinding(
			key.WithKeys("k"),
		),
		UpOneItem: key.NewBinding(
			key.WithKeys("shift+up"),
		),
		DownOneItem: key.NewBinding(
			key.WithKeys("shift+down"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("ctrl+d"),
		),
		HalfPageUp: key.NewBinding(
			key.WithKeys("ctrl+u"),
		),
		Home: key.NewBinding(
			key.WithKeys("g", "home"),
		),
		End: key.NewBinding(
			key.WithKeys("shift+g", "end"),
		),
		Submit: key.NewBinding(
			key.WithKeys("enter", "space"),
			key.WithHelp("enter/space", "select"),
		),
	}
}

// FullHelp implements help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding { return nil }

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(
			key.WithKeys("up", "down"),
			key.WithHelp("↓↑", "navigate"),
		),
		k.Submit,
	}
}
