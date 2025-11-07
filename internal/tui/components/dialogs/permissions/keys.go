package permissions

import (
	"charm.land/bubbles/v2/key"
)

type KeyMap struct {
	Left,
	Right,
	Tab,
	Select,
	Allow,
	AllowSession,
	Deny,
	ToggleDiffMode,
	ScrollDown,
	ScrollUp key.Binding
	ScrollLeft,
	ScrollRight key.Binding
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
			key.WithKeys("a", "A", "ctrl+a"),
			key.WithHelp("a", "allow"),
		),
		AllowSession: key.NewBinding(
			key.WithKeys("s", "S", "ctrl+s"),
			key.WithHelp("s", "allow session"),
		),
		Deny: key.NewBinding(
			key.WithKeys("d", "D", "esc"),
			key.WithHelp("d", "deny"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", "ctrl+y"),
			key.WithHelp("enter", "confirm"),
		),
		ToggleDiffMode: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "toggle diff mode"),
		),
		ScrollDown: key.NewBinding(
			key.WithKeys("shift+down", "J"),
			key.WithHelp("shift+↓", "scroll down"),
		),
		ScrollUp: key.NewBinding(
			key.WithKeys("shift+up", "K"),
			key.WithHelp("shift+↑", "scroll up"),
		),
		ScrollLeft: key.NewBinding(
			key.WithKeys("shift+left", "H"),
			key.WithHelp("shift+←", "scroll left"),
		),
		ScrollRight: key.NewBinding(
			key.WithKeys("shift+right", "L"),
			key.WithHelp("shift+→", "scroll right"),
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
		k.ToggleDiffMode,
		k.ScrollDown,
		k.ScrollUp,
		k.ScrollLeft,
		k.ScrollRight,
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
		k.ToggleDiffMode,
		key.NewBinding(
			key.WithKeys("shift+left", "shift+down", "shift+up", "shift+right"),
			key.WithHelp("shift+←↓↑→", "scroll"),
		),
	}
}
