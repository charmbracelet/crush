package models

import (
	"charm.land/bubbles/v2/key"
)

type KeyMap struct {
	Select,
	Next,
	Previous,
	Tab,
	Close key.Binding

	isAPIKeyHelp  bool
	isAPIKeyValid bool
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter", "ctrl+y"),
			key.WithHelp("enter", "choose"),
		),
		Next: key.NewBinding(
			key.WithKeys("down", "ctrl+n"),
			key.WithHelp("↓", "next item"),
		),
		Previous: key.NewBinding(
			key.WithKeys("up", "ctrl+p"),
			key.WithHelp("↑", "previous item"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "toggle type"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "exit"),
		),
	}
}

// KeyBindings implements layout.KeyMapProvider
func (k KeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.Select,
		k.Next,
		k.Previous,
		k.Tab,
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
	if k.isAPIKeyHelp && !k.isAPIKeyValid {
		return []key.Binding{
			k.Close,
		}
	} else if k.isAPIKeyValid {
		return []key.Binding{
			k.Select,
		}
	}
	return []key.Binding{
		key.NewBinding(
			key.WithKeys("down", "up"),
			key.WithHelp("↑↓", "choose"),
		),
		k.Tab,
		k.Select,
		k.Close,
	}
}
