package tools

import (
	"github.com/charmbracelet/bubbles/v2/key"
)

type KeyMap struct {
	Toggle key.Binding
	Close  key.Binding
	Up     key.Binding
	Down   key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Toggle: key.NewBinding(
			key.WithKeys(" ", "enter"),
			key.WithHelp("space/enter", "toggle"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
			key.WithHelp("esc", "close"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
	}
}

// ShortHelp returns keybindings to be shown in the mini help view.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Toggle, k.Up, k.Down, k.Close}
}

// FullHelp returns keybindings for the expanded help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Toggle, k.Up, k.Down, k.Close},
	}
}
