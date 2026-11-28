package dialog

import (
	"slices"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/dustin/go-humanize"
	"github.com/rivo/uniseg"
	"github.com/sahilm/fuzzy"
)

// sameFuzzyMatch reports whether two fuzzy.Match values are
// observably equal. Because Match contains a slice (MatchedIndexes)
// it is not directly comparable with ==; we compare the scalar
// fields and then walk the indexes. The dialog list items use this
// to skip gratuitous version bumps when SetMatch reapplies the same
// match.
func sameFuzzyMatch(a, b fuzzy.Match) bool {
	return a.Str == b.Str &&
		a.Index == b.Index &&
		a.Score == b.Score &&
		slices.Equal(a.MatchedIndexes, b.MatchedIndexes)
}

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
	pickerRow
	session.Session
	t                *styles.Styles
	sessionsMode     sessionsMode
	updateTitleInput textinput.Model
	hideInfo         bool
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

// InputValue returns the updated title value
func (s *SessionItem) InputValue() string {
	return s.updateTitleInput.Value()
}

// HandleInput forwards input message to the update title input
func (s *SessionItem) HandleInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	s.updateTitleInput, cmd = s.updateTitleInput.Update(msg)
	s.Bump()
	return cmd
}

// Cursor returns the cursor of the update title input
func (s *SessionItem) Cursor() *tea.Cursor {
	return s.updateTitleInput.Cursor()
}

// InfoText returns the secondary text shown on the right of the item.
func (s *SessionItem) InfoText() string {
	return humanize.Time(time.Unix(s.UpdatedAt, 0))
}

// SetHideInfo controls whether the timestamp info column is shown. The
// dialog hides it uniformly when it would crowd the title.
func (s *SessionItem) SetHideInfo(v bool) {
	if s.hideInfo == v {
		return
	}
	s.hideInfo = v
	s.invalidate()
}

// Render returns the string representation of the session item.
func (s *SessionItem) Render(width int) string {
	info := s.InfoText()
	if s.hideInfo {
		info = ""
	}
	styles := ListItemStyles{
		ItemBlurred:     s.t.Dialog.NormalItem,
		ItemFocused:     s.t.Dialog.SelectedItem,
		InfoTextBlurred: s.t.Dialog.Sessions.InfoBlurred,
		InfoTextFocused: s.t.Dialog.Sessions.InfoFocused,
	}

	switch s.sessionsMode {
	case sessionsModeDeleting:
		styles.ItemBlurred = s.t.Dialog.Sessions.DeletingItemBlurred
		styles.ItemFocused = s.t.Dialog.Sessions.DeletingItemFocused
	case sessionsModeUpdating:
		styles.ItemBlurred = s.t.Dialog.Sessions.RenamingItemBlurred
		styles.ItemFocused = s.t.Dialog.Sessions.RenamingingItemFocused
		if s.focused {
			const cursorPadding = 1
			inputWidth := max(0, width-styles.ItemFocused.GetHorizontalFrameSize()-cursorPadding)
			s.updateTitleInput.SetWidth(inputWidth)
			s.updateTitleInput.Placeholder = ansi.Truncate(s.Title, width, "…")
			return styles.ItemFocused.Render(s.updateTitleInput.View())
		}
	}

	return s.render(styles, s.Title, info, width)
}

// sessionItems takes a slice of [session.Session]s and convert them to a slice
// of [ListItem]s.
func sessionItems(t *styles.Styles, mode sessionsMode, sessions ...session.Session) []list.FilterableItem {
	items := make([]list.FilterableItem, len(sessions))
	for i, s := range sessions {
		item := &SessionItem{pickerRow: newPickerRow(), Session: s, t: t, sessionsMode: mode}
		if mode == sessionsModeUpdating {
			item.updateTitleInput = textinput.New()
			item.updateTitleInput.SetVirtualCursor(false)
			item.updateTitleInput.Prompt = ""
			item.updateTitleInput.SetStyles(sessionRenameInputStyles(t))
			item.updateTitleInput.SetValue(s.Title)
			item.updateTitleInput.Focus()
		}
		items[i] = item
	}
	return items
}

// sessionRenameInputStyles returns the styles of the input that renames a
// session.
func sessionRenameInputStyles(t *styles.Styles) textinput.Styles {
	inputStyle := t.TextInput
	inputStyle.Focused.Placeholder = t.Dialog.Sessions.RenamingPlaceholder
	return inputStyle
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
