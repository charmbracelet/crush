package dialog

import (
	"strings"
	"time"

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
	list.FocusStylable
	list.MatchSettable

	// ID returns the unique identifier of the item.
	ID() string
}

// SessionItem wraps a [session.Session] to implement the [ListItem] interface.
type SessionItem struct {
	session.Session
	t       *styles.Styles
	m       fuzzy.Match
	focused bool
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
	s.m = m
}

// SetFocused set the current items focus state
func (s *SessionItem) SetFocused(focused bool) {
	s.focused = focused
}

// Render returns the string representation of the session item.
func (s *SessionItem) Render(width int) string {
	lastUpdated := humanize.Time(time.Unix(s.UpdatedAt, 0))
	if !s.focused {
		lastUpdated = s.t.Subtle.Render(lastUpdated)
	}
	lastUpdated = " " + lastUpdated
	ageLen := lipgloss.Width(lastUpdated)
	title := s.Title
	titleLen := lipgloss.Width(title)
	title = ansi.Truncate(title, max(0, width-ageLen), "…")
	right := lipgloss.NewStyle().AlignHorizontal(lipgloss.Right).Width(width - titleLen).Render(lastUpdated)

	if matches := len(s.m.MatchedIndexes); matches > 0 {
		var lastPos int
		parts := make([]string, 0)
		// TODO: Use [ansi.Style].Underline true/false to underline only the
		// matched parts instead of using [lipgloss.StyleRanges] since it can
		// be cheaper with less allocations.
		ranges := matchedRanges(s.m.MatchedIndexes)
		for _, rng := range ranges {
			start, stop := bytePosToVisibleCharPos(title, rng)
			if start > lastPos {
				parts = append(parts, title[lastPos:start])
			}
			// NOTE: We're using [ansi.Style] here instead of [lipgloss.Style]
			// because we can control the underline start and stop more
			// precisely via [ansi.AttrUnderline] and [ansi.AttrNoUnderline]
			// which only affect the underline attribute without interfering
			// with other styles.
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
		return strings.Join(parts, "") + right
	}
	return title + right
}

// FocusStyle returns the style to be applied when the item is focused.
func (s *SessionItem) FocusStyle() lipgloss.Style {
	return s.t.Dialog.SelectedItem
}

// BlurStyle returns the style to be applied when the item is blurred.
func (s *SessionItem) BlurStyle() lipgloss.Style {
	return s.t.Dialog.NormalItem
}

// sessionItems takes a slice of [session.Session]s and convert them to a slice
// of [ListItem]s.
func sessionItems(t *styles.Styles, sessions ...session.Session) []list.FilterableItem {
	items := make([]list.FilterableItem, len(sessions))
	for i, s := range sessions {
		items[i] = &SessionItem{Session: s, t: t}
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
