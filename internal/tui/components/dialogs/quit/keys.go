package quit

import (
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/opencode-ai/opencode/internal/tui/layout"
)

// KeyMap defines the keyboard bindings for the quit dialog.
type KeyMap struct {
	LeftRight,
	EnterSpace,
	Yes,
	No,
	Tab,
	Close key.Binding
}

func DefaultKeymap() KeyMap {
	return KeyMap{
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
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
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
		k.LeftRight,
		k.EnterSpace,
	}
}
