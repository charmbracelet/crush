package layout

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// TODO: move this to core

type Focusable interface {
	Focus() tea.Cmd
	Blur() tea.Cmd
	IsFocused() bool
}

type Sizeable interface {
	SetSize(width, height int) tea.Cmd
	GetSize() (int, int)
}

type Help interface {
	Bindings() []key.Binding
}

type Positional interface {
	SetPosition(x, y int) tea.Cmd
}
