package commands

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/tui/components/core/list"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
	"github.com/rivo/uniseg"
)

type CommandItem interface {
	util.Model
	layout.Focusable
	layout.Sizeable
	Command() Command
}

type commandItem struct {
	width        int
	command      Command
	focus        bool
	matchIndexes []int
}

func NewCommandItem(command Command) CommandItem {
	return &commandItem{
		command:      command,
		matchIndexes: make([]int, 0),
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
	t := theme.CurrentTheme()

	baseStyle := styles.BaseStyle()
	titleStyle := baseStyle.Width(c.width).Foreground(t.Text())
	titleMatchStyle := baseStyle.Foreground(t.Text()).Underline(true)

	if c.focus {
		titleStyle = titleStyle.Foreground(t.Background()).Background(t.Primary()).Bold(true)
		titleMatchStyle = titleMatchStyle.Foreground(t.Background()).Background(t.Primary()).Bold(true)
	}
	var ranges []lipgloss.Range
	truncatedTitle := ansi.Truncate(c.command.Title, c.width, "…")
	text := titleStyle.Render(truncatedTitle)
	if len(c.matchIndexes) > 0 {
		for _, rng := range matchedRanges(c.matchIndexes) {
			// ansi.Cut is grapheme and ansi sequence aware, we match against a ansi.Stripped string, but we might still have graphemes.
			// all that to say that rng is byte positions, but we need to pass it down to ansi.Cut as char positions.
			// so we need to adjust it here:
			start, stop := bytePosToVisibleCharPos(text, rng)
			ranges = append(ranges, lipgloss.NewRange(start, stop+1, titleMatchStyle))
		}
		text = lipgloss.StyleRanges(text, ranges...)
	}
	return tea.NewView(text)
}

// Command implements CommandItem.
func (c *commandItem) Command() Command {
	return c.command
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

// GetSize implements CommandItem.
func (c *commandItem) GetSize() (int, int) {
	return c.width, 2
}

// SetSize implements CommandItem.
func (c *commandItem) SetSize(width int, height int) tea.Cmd {
	c.width = width
	return nil
}

func (c *commandItem) FilterValue() string {
	return c.command.Title
}

func (c *commandItem) MatchIndexes(indexes []int) {
	c.matchIndexes = indexes
}

func matchedRanges(in []int) [][2]int {
	if len(in) == 0 {
		return [][2]int{}
	}
	current := [2]int{in[0], in[0]}
	if len(in) == 1 {
		return [][2]int{current}
	}
	var out [][2]int
	for i := 1; i < len(in); i++ {
		if in[i] == current[1]+1 {
			current[1] = in[i]
		} else {
			out = append(out, current)
			current = [2]int{in[i], in[i]}
		}
	}
	out = append(out, current)
	return out
}

func bytePosToVisibleCharPos(str string, rng [2]int) (int, int) {
	bytePos, byteStart, byteStop := 0, rng[0], rng[1]
	pos, start, stop := 0, 0, 0
	gr := uniseg.NewGraphemes(str)
	for byteStart > bytePos {
		if !gr.Next() {
			break
		}
		bytePos += len(gr.Str())
		pos += max(1, gr.Width())
	}
	start = pos
	for byteStop > bytePos {
		if !gr.Next() {
			break
		}
		bytePos += len(gr.Str())
		pos += max(1, gr.Width())
	}
	stop = pos
	return start, stop
}

type ItemSection interface {
	util.Model
	layout.Sizeable
	list.SectionHeader
}
type itemSectionModel struct {
	width int
	title string
}

func NewItemSection(title string) ItemSection {
	return &itemSectionModel{
		title: title,
	}
}

func (m *itemSectionModel) Init() tea.Cmd {
	return nil
}

func (m *itemSectionModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m *itemSectionModel) View() tea.View {
	t := theme.CurrentTheme()
	title := ansi.Truncate(m.title, m.width-1, "…")
	style := styles.BaseStyle().Padding(1, 0, 0, 0).Width(m.width).Foreground(t.TextMuted()).Bold(true)
	if len(title) < m.width {
		remainingWidth := m.width - lipgloss.Width(title)
		if remainingWidth > 0 {
			title += " " + strings.Repeat("─", remainingWidth-1)
		}
	}
	return tea.NewView(style.Render(title))
}

func (m *itemSectionModel) GetSize() (int, int) {
	return m.width, 1
}

func (m *itemSectionModel) SetSize(width int, height int) tea.Cmd {
	m.width = width
	return nil
}

func (m *itemSectionModel) IsSectionHeader() bool {
	return true
}
