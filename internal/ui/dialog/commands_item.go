package dialog

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

// Segment is one option in a row that shows every option at once, like the
// approval axis. Showing all of them beats a toggle because the user can see
// what the alternatives are, and which one is in force, without cycling to
// find out.
type Segment struct {
	Text string
	// Active marks the option currently in force.
	Active bool
}

// CommandItem wraps a uicmd.Command to implement the ListItem interface.
type CommandItem struct {
	*list.Versioned
	id          string
	title       string
	shortcut    string
	description string
	action      Action
	aliases     []string
	segments    []Segment
	t           *styles.Styles
	m           fuzzy.Match
	cache       map[int]string
	focused     bool
	hideInfo    bool
}

var _ ListItem = &CommandItem{Versioned: list.NewVersioned()}

// NewCommandItem creates a new CommandItem.
func NewCommandItem(t *styles.Styles, id, title, shortcut string, action Action) *CommandItem {
	return &CommandItem{
		Versioned: list.NewVersioned(),
		id:        id,
		t:         t,
		title:     title,
		shortcut:  shortcut,
		action:    action,
	}
}

// Finished implements list.Item. Command items are render-stable
// outside of explicit SetFocused / SetMatch.
func (c *CommandItem) Finished() bool {
	return true
}

// WithSegments renders the title as several options in a row, with the active
// one marked. The row stays filterable by every option's text, so typing "yolo"
// still finds it.
func (c *CommandItem) WithSegments(segments ...Segment) *CommandItem {
	c.SetSegments(segments)
	return c
}

// SetSegments replaces the options shown, so a row can follow the setting it
// displays without the list being rebuilt around it.
func (c *CommandItem) SetSegments(segments []Segment) {
	if sameSegments(c.segments, segments) {
		return
	}
	c.segments = segments
	c.cache = nil
	if c.Versioned != nil {
		c.Bump()
	}
}

// WithAliases returns the CommandItem with the given aliases for filtering.
func (c *CommandItem) WithAliases(aliases ...string) *CommandItem {
	c.aliases = aliases
	return c
}

// WithDescription returns the CommandItem with a description displayed below
// the title.
func (c *CommandItem) WithDescription(desc string) *CommandItem {
	c.description = desc
	return c
}

// Filter implements ListItem.
func (c *CommandItem) Filter() string {
	// Every option a row shows has to be searchable, so a row that displays
	// several of them contributes all of their text, not just its title.
	terms := []string{c.title}
	if len(c.segments) > 0 {
		terms = append(terms, segmentTexts(c.segments)...)
	}
	terms = append(terms, c.aliases...)
	if c.description != "" {
		terms = append(terms, c.description)
	}
	return strings.Join(terms, " ")
}

// ID implements ListItem.
func (c *CommandItem) ID() string {
	return c.id
}

// SetFocused implements ListItem.
func (c *CommandItem) SetFocused(focused bool) {
	if c.focused == focused {
		return
	}
	c.cache = nil
	c.focused = focused
	if c.Versioned != nil {
		c.Bump()
	}
}

// SetMatch implements ListItem.
func (c *CommandItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(c.m, m) {
		return
	}
	c.cache = nil
	c.m = m
	if c.Versioned != nil {
		c.Bump()
	}
}

// Action returns the action associated with the command item.
func (c *CommandItem) Action() Action {
	return c.action
}

// Shortcut returns the shortcut associated with the command item.
func (c *CommandItem) Shortcut() string {
	return c.shortcut
}

// InfoText implements infoColumnItem; the command shortcut is its info.
func (c *CommandItem) InfoText() string {
	return c.shortcut
}

// SetHideInfo controls whether the shortcut hint column is shown. The
// dialog hides it uniformly when it would crowd the command names.
func (c *CommandItem) SetHideInfo(v bool) {
	if c.hideInfo == v {
		return
	}
	c.cache = nil
	c.hideInfo = v
	if c.Versioned != nil {
		c.Bump()
	}
}

// Render implements ListItem.
func (c *CommandItem) Render(width int) string {
	styles := ListItemStyles{
		ItemBlurred:     c.t.Dialog.NormalItem,
		ItemFocused:     c.t.Dialog.SelectedItem,
		InfoTextBlurred: c.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: c.t.Dialog.ListItem.InfoFocused,
	}
	shortcut := c.shortcut
	if c.hideInfo {
		shortcut = ""
	}
	title, match := c.title, &c.m
	if len(c.segments) > 0 {
		title, match = c.renderSegments(), nil
	}
	rendered := renderItem(styles, title, shortcut, c.focused, width, c.cache, match)
	if c.description != "" {
		descStyle := c.t.Dialog.SecondaryText
		if c.focused {
			descStyle = c.t.Dialog.SelectedItem
		}
		contentWidth := max(0, width-descStyle.GetHorizontalFrameSize()+1)
		description := ansi.Truncate(strings.TrimSpace(c.description), contentWidth, "...")
		descVisWidth := lipgloss.Width(description)
		gap := strings.Repeat(" ", max(0, contentWidth-descVisWidth))
		if description == "" {
			description = " "
		}
		rendered = lipgloss.JoinVertical(lipgloss.Left, rendered, descStyle.Render(description+gap))
	}
	return rendered
}

// renderSegments draws the options in a row with the active one marked. The
// marker is a pair of brackets as well as a colour, so the row still says which
// option is in force on a monochrome terminal or to a reader who cannot
// distinguish the accent from the body text.
//
// On a focused row the item style paints its own background and foreground, and
// an accent drawn on top of that background would disappear into it; weight
// carries the mark there instead.
func (c *CommandItem) renderSegments() string {
	active, inactive := c.t.Dialog.PrimaryText, c.t.Dialog.SecondaryText
	if c.focused {
		active = lipgloss.NewStyle().Bold(true)
		inactive = lipgloss.NewStyle()
	}

	parts := make([]string, 0, len(c.segments)+1)
	// The title names the axis and inherits the row's own style, so it reads
	// the same whether the row is focused or not.
	parts = append(parts, c.title)
	for _, seg := range c.segments {
		if seg.Active {
			parts = append(parts, active.Render("["+seg.Text+"]"))
			continue
		}
		parts = append(parts, inactive.Render(seg.Text))
	}
	return strings.Join(parts, " ")
}

// segmentTexts is the plain text of the options, used for filtering and for
// telling whether a row has actually changed.
func segmentTexts(segments []Segment) []string {
	texts := make([]string, len(segments))
	for i, seg := range segments {
		texts[i] = seg.Text
	}
	return texts
}

func sameSegments(a, b []Segment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
