package dialog

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/dustin/go-humanize"
	"github.com/rivo/uniseg"
	"github.com/sahilm/fuzzy"
)

// ListItem represents a selectable and searchable item in a dialog list.
type ListItem interface {
	list.FilterableItem
	list.Focusable
	list.MatchSettable

	// ID returns the unique identifier of the item.
	ID() string
}

// SessionItem wraps a [session.Session] to implement the [ListItem] interface.
type SessionItem struct {
	session.Session
	t                *styles.Styles
	sessionsMode     sessionsMode
	m                fuzzy.Match
	cache            map[int]string
	updateTitleInput textinput.Model
	focused          bool
}

var _ ListItem = &SessionItem{}

// Filter returns the filterable value of the session.
func (s *SessionItem) Filter() string {
	return s.Title
}

// ID returns the unique identifier of the session.
func (s *SessionItem) ID() string {
	return s.Session.ID
}

// SetMatch sets the fuzzy match for the session item.
func (s *SessionItem) SetMatch(m fuzzy.Match) {
	s.cache = nil
	s.m = m
}

// InputValue returns the updated title value
func (s *SessionItem) InputValue() string {
	return s.updateTitleInput.Value()
}

// HandleInput forwards input message to the update title input
func (s *SessionItem) HandleInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	s.updateTitleInput, cmd = s.updateTitleInput.Update(msg)
	return cmd
}

// Cursor returns the cursor of the update title input
func (s *SessionItem) Cursor() *tea.Cursor {
	return s.updateTitleInput.Cursor()
}

// Render returns the string representation of the session item.
func (s *SessionItem) Render(width int) string {
	info := humanize.Time(time.Unix(s.UpdatedAt, 0))
	styles := ListIemStyles{
		ItemBlurred:     s.t.Dialog.NormalItem,
		ItemFocused:     s.t.Dialog.SelectedItem,
		InfoTextBlurred: s.t.Subtle,
		InfoTextFocused: s.t.Base,
	}

	switch s.sessionsMode {
	case sessionsModeDeleting:
		styles.ItemBlurred = s.t.Dialog.Sessions.DeletingItemBlurred
		styles.ItemFocused = s.t.Dialog.Sessions.DeletingItemFocused
	case sessionsModeUpdating:
		styles.ItemBlurred = s.t.Dialog.Sessions.UpdatingItemBlurred
		styles.ItemFocused = s.t.Dialog.Sessions.UpdatingItemFocused
		if s.focused {
			inputWidth := width - styles.InfoTextFocused.GetHorizontalFrameSize()
			s.updateTitleInput.SetWidth(inputWidth)
			s.updateTitleInput.Placeholder = ansi.Truncate(s.Title, width, "…")
			return styles.ItemFocused.Render(s.updateTitleInput.View())
		}
	}

	return renderItem(styles, s.Title, info, s.focused, width, s.cache, &s.m)
}

type ListIemStyles struct {
	ItemBlurred     lipgloss.Style
	ItemFocused     lipgloss.Style
	InfoTextBlurred lipgloss.Style
	InfoTextFocused lipgloss.Style
}

func renderItem(t ListIemStyles, title string, info string, focused bool, width int, cache map[int]string, m *fuzzy.Match) string {
	if cache == nil {
		cache = make(map[int]string)
	}

	cached, ok := cache[width]
	if ok {
		return cached
	}

	style := t.ItemBlurred
	if focused {
		style = t.ItemFocused
	}

	var infoText string
	var infoWidth int
	lineWidth := width
	if len(info) > 0 {
		infoText = fmt.Sprintf(" %s ", info)
		if focused {
			infoText = t.InfoTextFocused.Render(infoText)
		} else {
			infoText = t.InfoTextBlurred.Render(infoText)
		}

		infoWidth = lipgloss.Width(infoText)
	}

	title = ansi.Truncate(title, max(0, lineWidth-infoWidth), "")
	titleWidth := lipgloss.Width(title)
	gap := strings.Repeat(" ", max(0, lineWidth-titleWidth-infoWidth))
	content := title
	if m != nil && len(m.MatchedIndexes) > 0 {
		var lastPos int
		parts := make([]string, 0)
		ranges := matchedRanges(m.MatchedIndexes)
		for _, rng := range ranges {
			start, stop := bytePosToVisibleCharPos(title, rng)
			if start > lastPos {
				parts = append(parts, title[lastPos:start])
			}
			// NOTE: We're using [ansi.Style] here instead of [lipglosStyle]
			// because we can control the underline start and stop more
			// precisely via [ansi.AttrUnderline] and [ansi.AttrNoUnderline]
			// which only affect the underline attribute without interfering
			// with other style
			parts = append(parts,
				ansi.NewStyle().Underline(true).String(),
				title[start:stop+1],
				ansi.NewStyle().Underline(false).String(),
			)
			lastPos = stop + 1
		}
		if lastPos < len(title) {
			parts = append(parts, title[lastPos:])
		}

		content = strings.Join(parts, "")
	}

	content = style.Render(content + gap + infoText)
	cache[width] = content
	return content
}

// SetFocused sets the focus state of the session item.
func (s *SessionItem) SetFocused(focused bool) {
	if s.focused != focused {
		s.cache = nil
	}
	s.focused = focused
}

// sessionItems takes a slice of [session.Session]s and convert them to a slice
// of [ListItem]s.
func sessionItems(t *styles.Styles, mode sessionsMode, sessions ...session.Session) []list.FilterableItem {
	items := make([]list.FilterableItem, len(sessions))
	for i, s := range sessions {
		item := &SessionItem{Session: s, t: t, sessionsMode: mode}
		if mode == sessionsModeUpdating {
			item.updateTitleInput = textinput.New()
			item.updateTitleInput.SetVirtualCursor(false)
			item.updateTitleInput.Prompt = ""
			inputStyle := t.TextInput
			inputStyle.Focused.Placeholder = inputStyle.Focused.Placeholder.Foreground(t.FgHalfMuted)
			item.updateTitleInput.SetStyles(inputStyle)
			item.updateTitleInput.Focus()
		}
		items[i] = item
	}
	return items
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
