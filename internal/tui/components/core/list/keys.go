package list

import (
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/tui/layout"
)

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
	End key.Binding
}

func DefaultKeyMap() KeyMap {
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
			key.WithKeys("shift+up", "shift+k"),
		),
		DownOneItem: key.NewBinding(
			key.WithKeys("shift+down", "shift+j"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("d"),
		),
		HalfPageUp: key.NewBinding(
			key.WithKeys("u"),
		),
		Home: key.NewBinding(
			key.WithKeys("g", "home"),
		),
		End: key.NewBinding(
			key.WithKeys("shift+g", "end"),
		),
	}
}

// FullHelp implements help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := layout.KeyMapToSlice(k)
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
	}
}
