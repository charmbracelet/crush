package layout

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
)

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

type Positionable interface {
	SetPosition(x, y int) tea.Cmd
}

// KeyMapProvider defines an interface for types that can provide their key bindings as a slice
type KeyMapProvider interface {
	KeyBindings() []key.Binding
}