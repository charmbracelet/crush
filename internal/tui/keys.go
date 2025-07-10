package tui

import (
	"github.com/charmbracelet/bubbles/v2/key"
)

type KeyMap struct {
	Quit     key.Binding
	Help     key.Binding
	Commands key.Binding
	Sessions key.Binding

	pageBindings []key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("ctrl+g", "help"),
		),
		Commands: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "commands"),
		),
		Sessions: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "sessions"),
		),
	}
}

// FullHelp implements help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := []key.Binding{
		k.Commands,
		k.Sessions,
		k.Quit,
		k.Help,
	}
	slice = k.prependEscAndTab(slice)
	slice = append(slice, k.pageBindings...)
	// remove duplicates
	seen := make(map[string]bool)
	cleaned := []key.Binding{}
	for _, b := range slice {
		if !seen[b.Help().Key] {
			seen[b.Help().Key] = true
			cleaned = append(cleaned, b)
		}
	}

	for i := 0; i < len(cleaned); i += 3 {
		end := min(i+3, len(cleaned))
		m = append(m, cleaned[i:end])
	}
	return m
}

func (k KeyMap) prependEscAndTab(bindings []key.Binding) []key.Binding {
	var cancel key.Binding
	var tab key.Binding
	for _, b := range k.pageBindings {
		if b.Help().Key == "esc" {
			cancel = b
		}
		if b.Help().Key == "tab" {
			tab = b
		}
	}
	if tab.Help().Key != "" {
		bindings = append([]key.Binding{tab}, bindings...)
	}
	if cancel.Help().Key != "" {
		bindings = append([]key.Binding{cancel}, bindings...)
	}
	return bindings
}

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	bindings := []key.Binding{
		k.Commands,
		k.Sessions,
		k.Quit,
		k.Help,
	}
	return k.prependEscAndTab(bindings)
}
