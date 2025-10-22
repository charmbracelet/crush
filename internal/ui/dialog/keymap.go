package dialog

import "github.com/charmbracelet/bubbles/v2/key"

// KeyMap defines key bindings for dialogs.
type KeyMap struct {
	Close key.Binding
}

// DefaultKeyMap returns the default key bindings for dialogs.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Close: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
		),
	}
}

// QuitKeyMap represents key bindings for the quit dialog.
type QuitKeyMap struct {
	LeftRight,
	EnterSpace,
	Yes,
	No,
	Tab,
	Close key.Binding
}

// DefaultQuitKeyMap returns the default key bindings for the quit dialog.
func DefaultQuitKeyMap() QuitKeyMap {
	return QuitKeyMap{
		LeftRight: key.NewBinding(
			key.WithKeys("left", "right"),
			key.WithHelp("←/→", "switch options"),
		),
		EnterSpace: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter/space", "confirm"),
		),
		Yes: key.NewBinding(
			key.WithKeys("y", "Y", "ctrl+c"),
			key.WithHelp("y/Y/ctrl+c", "yes"),
		),
		No: key.NewBinding(
			key.WithKeys("n", "N"),
			key.WithHelp("n/N", "no"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch options"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}
