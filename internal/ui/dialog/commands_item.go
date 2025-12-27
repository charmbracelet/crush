package dialog

import (
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/uicmd"
	"github.com/sahilm/fuzzy"
)

// CommandItem wraps a uicmd.Command to implement the ListItem interface.
type CommandItem struct {
	Cmd     uicmd.Command
	t       *styles.Styles
	m       fuzzy.Match
	cache   map[int]string
	focused bool
}

var _ ListItem = &CommandItem{}

// NewCommandItem creates a new CommandItem.
func NewCommandItem(t *styles.Styles, cmd uicmd.Command) *CommandItem {
	return &CommandItem{
		Cmd: cmd,
		t:   t,
	}
}

// Filter implements ListItem.
func (c *CommandItem) Filter() string {
	return c.Cmd.Title
}

// ID implements ListItem.
func (c *CommandItem) ID() string {
	return c.Cmd.ID
}

// SetFocused implements ListItem.
func (c *CommandItem) SetFocused(focused bool) {
	if c.focused != focused {
		c.cache = nil
	}
	c.focused = focused
}

// SetMatch implements ListItem.
func (c *CommandItem) SetMatch(m fuzzy.Match) {
	c.cache = nil
	c.m = m
}

// Render implements ListItem.
func (c *CommandItem) Render(width int) string {
	return renderItem(c.t, c.Cmd.Title, c.Cmd.Shortcut, c.focused, width, c.cache, &c.m)
}
