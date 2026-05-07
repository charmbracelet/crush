package dialog

import (
	"strings"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/sahilm/fuzzy"
)

// CommandItem wraps a uicmd.Command to implement the ListItem interface.
type CommandItem struct {
	id       string
	title    string
	shortcut string
	action   Action
	aliases  []string
	t        *styles.Styles
	m        fuzzy.Match
	cache    map[int]string
	focused  bool
}

var _ ListItem = &CommandItem{}

// NewCommandItem creates a new CommandItem.
func NewCommandItem(t *styles.Styles, id, title, shortcut string, action Action) *CommandItem {
	return &CommandItem{
		id:       id,
		t:        t,
		title:    title,
		shortcut: shortcut,
		action:   action,
	}
}

// WithAliases returns the CommandItem with the given aliases for filtering.
func (c *CommandItem) WithAliases(aliases ...string) *CommandItem {
	c.aliases = aliases
	return c
}

// Filter implements ListItem.
func (c *CommandItem) Filter() string {
	if len(c.aliases) == 0 {
		return c.title
	}
	return c.title + " " + strings.Join(c.aliases, " ")
}

// ID implements ListItem.
func (c *CommandItem) ID() string {
	return c.id
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

// Action returns the action associated with the command item.
func (c *CommandItem) Action() Action {
	return c.action
}

// Shortcut returns the shortcut associated with the command item.
func (c *CommandItem) Shortcut() string {
	return c.shortcut
}

// Render implements ListItem.
func (c *CommandItem) Render(width int) string {
	styles := ListItemStyles{
		ItemBlurred:     c.t.Dialog.NormalItem,
		ItemFocused:     c.t.Dialog.SelectedItem,
		InfoTextBlurred: c.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: c.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(styles, c.title, c.shortcut, c.focused, width, c.cache, &c.m)
}
