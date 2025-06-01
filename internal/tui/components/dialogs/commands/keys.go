package commands

import (
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/opencode-ai/opencode/internal/tui/layout"
)

type CommandsDialogKeyMap struct {
	Select   key.Binding
	Next     key.Binding
	Previous key.Binding
}

func DefaultCommandsDialogKeyMap() CommandsDialogKeyMap {
	return CommandsDialogKeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter", "tab", "ctrl+y"),
			key.WithHelp("enter", "confirm"),
		),
		Next: key.NewBinding(
			key.WithKeys("down", "ctrl+n"),
			key.WithHelp("↓", "next item"),
		),
		Previous: key.NewBinding(
			key.WithKeys("up", "ctrl+p"),
			key.WithHelp("↑", "previous item"),
		),
	}
}

// FullHelp implements help.KeyMap.
func (k CommandsDialogKeyMap) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := layout.KeyMapToSlice(k)
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// ShortHelp implements help.KeyMap.
func (k CommandsDialogKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Select,
		k.Next,
		k.Previous,
	}
}

type ArgumentsDialogKeyMap struct {
	Confirm  key.Binding
	Next     key.Binding
	Previous key.Binding
}

func DefaultArgumentsDialogKeyMap() ArgumentsDialogKeyMap {
	return ArgumentsDialogKeyMap{
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),

		Next: key.NewBinding(
			key.WithKeys("tab", "down"),
			key.WithHelp("tab/↓", "next"),
		),
		Previous: key.NewBinding(
			key.WithKeys("shift+tab", "up"),
			key.WithHelp("shift+tab/↑", "previous"),
		),
	}
}

// FullHelp implements help.KeyMap.
func (k ArgumentsDialogKeyMap) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := layout.KeyMapToSlice(k)
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// ShortHelp implements help.KeyMap.
func (k ArgumentsDialogKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Confirm,
		k.Next,
		k.Previous,
	}
}
