package splash

import (
	"github.com/charmbracelet/bubbles/v2/key"
)

type KeyMap struct {
	Cancel key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}
