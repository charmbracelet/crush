package filepicker

import (
	"charm.land/bubbles/v2/key"
)

// KeyMap defines keyboard bindings for dialog management.
type KeyMap struct {
	Select,
	Down,
	Up,
	Forward,
	Backward,
	Close key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "accept"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("down/j", "move down"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("up/k", "move up"),
		),
		Forward: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("right/l", "move forward"),
		),
		Backward: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("left/h", "move backward"),
		),

		Close: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "close/exit"),
		),
	}
}

// KeyBindings implements layout.KeyMapProvider
func (k KeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.Select,
		k.Down,
		k.Up,
		k.Forward,
		k.Backward,
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
		key.NewBinding(
			key.WithKeys("right", "l", "left", "h", "up", "k", "down", "j"),
			key.WithHelp("↑↓←→", "navigate"),
		),
		k.Select,
		k.Close,
	}
}
