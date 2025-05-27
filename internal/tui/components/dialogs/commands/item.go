package commands

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type CommandItem interface {
	util.Model
	layout.Focusable
}

type commandItem struct {
	command Command
	focus   bool
}

func NewCommandItem(command Command) CommandItem {
	return &commandItem{
		command: command,
	}
}

// Init implements CommandItem.
func (c *commandItem) Init() tea.Cmd {
	return nil
}

// Update implements CommandItem.
func (c *commandItem) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return c, nil
}

// View implements CommandItem.
func (c *commandItem) View() tea.View {
	return tea.NewView(c.command.Title)
}

// Blur implements CommandItem.
func (c *commandItem) Blur() tea.Cmd {
	c.focus = false
	return nil
}

// Focus implements CommandItem.
func (c *commandItem) Focus() tea.Cmd {
	c.focus = true
	return nil
}

// IsFocused implements CommandItem.
func (c *commandItem) IsFocused() bool {
	return c.focus
}
