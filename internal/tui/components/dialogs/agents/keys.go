package agents

import (
	"charm.land/bubbles/v2/key"
)

// AgentsDialogKeyMap defines the keybindings for the agents dialog.
type AgentsDialogKeyMap struct {
	Select   key.Binding
	Next     key.Binding
	Previous key.Binding
	Close    key.Binding
}

// ShortHelp returns a list of keybindings for the short help view.
func (k AgentsDialogKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.Next, k.Previous, k.Close}
}

// FullHelp returns a list of keybindings for the full help view.
func (k AgentsDialogKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Select, k.Next, k.Previous, k.Close},
	}
}

// DefaultAgentsDialogKeyMap returns the default keybindings for the agents dialog.
func DefaultAgentsDialogKeyMap() AgentsDialogKeyMap {
	return AgentsDialogKeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Next: key.NewBinding(
			key.WithKeys("down", "j", "tab"),
			key.WithHelp("↓/j/tab", "next"),
		),
		Previous: key.NewBinding(
			key.WithKeys("up", "k", "shift+tab"),
			key.WithHelp("↑/k/shift+tab", "previous"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
			key.WithHelp("esc", "close"),
		),
	}
}
