package dialog

import (
	"image"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
)

// newQuestionTextarea creates a configured textarea for question
// input. All question textareas share the same base configuration;
// only placeholder and char limit vary.
func newQuestionTextarea(sty *styles.Styles, placeholder string, charLimit int) textarea.Model {
	ta := textarea.New()
	taStyles := sty.Editor.Textarea
	taStyles.Cursor.Color = sty.Editor.PromptYoloDotsFocused.GetForeground()
	ta.SetStyles(taStyles)
	ta.Placeholder = placeholder
	ta.ShowLineNumbers = false
	ta.CharLimit = charLimit
	ta.MaxWidth = choiceListMaxWidth
	ta.SetVirtualCursor(false)
	ta.DynamicHeight = true
	ta.MinHeight = 1
	ta.MaxHeight = 3
	ta.SetHeight(1)
	ta.SetPromptFunc(0, func(textarea.PromptInfo) string { return "" })
	ta.KeyMap.InsertNewline = key.NewBinding(key.WithDisabled())
	ta.Blur()
	return ta
}

// questionEditor owns the fill-in textarea, note editor, and notes
// map shared across all question component types. Components embed
// this struct and call its methods instead of reimplementing editor
// logic.
type questionEditor struct {
	Styles *styles.Styles

	fillIn        textarea.Model
	noteEditor    textarea.Model
	activeNoteKey string // non-empty when a note editor is open
	notes         map[string]string

	keyNote key.Binding
	navUp   key.Binding
	navDown key.Binding
}

// newQuestionEditor creates a questionEditor with configured
// fill-in and note textareas.
func newQuestionEditor(sty *styles.Styles) questionEditor {
	return questionEditor{
		Styles:     sty,
		fillIn:     newQuestionTextarea(sty, "Something else?", 500),
		noteEditor: newQuestionTextarea(sty, "Add a note...", 300),
		notes:      make(map[string]string),
		keyNote:    key.NewBinding(key.WithKeys("alt+n"), key.WithHelp("alt+n", "note")),
		navUp:      key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
		navDown:    key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
	}
}

// openNote opens the note editor for the given key, pre-populating
// it with any existing note text.
func (e *questionEditor) openNote(noteKey string) tea.Cmd {
	e.activeNoteKey = noteKey
	if existing, ok := e.notes[noteKey]; ok {
		e.noteEditor.SetValue(existing)
	} else {
		e.noteEditor.Reset()
	}
	return e.noteEditor.Focus()
}

// closeNote saves the current note text and closes the editor.
func (e *questionEditor) closeNote(noteKey string) {
	e.activeNoteKey = ""
	val := strings.TrimSpace(e.noteEditor.Value())
	if val != "" {
		e.notes[noteKey] = val
	} else {
		delete(e.notes, noteKey)
	}
	e.noteEditor.Blur()
}

// handleNoteKey processes keys when the note editor is focused.
// Returns (cmd, handled). When handled is true the caller should
// not process the key further. onClose is called for the close
// key so the caller can control what happens after closing.
func (e *questionEditor) handleNoteKey(msg tea.KeyPressMsg, closeKey key.Binding, onClose func()) (tea.Cmd, bool) {
	switch {
	case key.Matches(msg, closeKey):
		onClose()
		return nil, true
	case key.Matches(msg, e.navUp), key.Matches(msg, e.navDown):
		onClose()
		return nil, false
	default:
		if key.Matches(msg, key.NewBinding(key.WithKeys("enter"))) {
			onClose()
			return nil, true
		}
		var cmd tea.Cmd
		e.noteEditor, cmd = e.noteEditor.Update(msg)
		return cmd, true
	}
}

// drawFillIn appends fill-in rows to lines. When focused, renders
// the live textarea; otherwise shows saved text or placeholder.
// styleFilled controls whether non-empty fill-in text gets the
// selected (pink) style. Pass true for single-choice where the
// fill-in IS the answer; false for multi-choice where it's supplementary.
func (e *questionEditor) drawFillIn(lines *[]contentLine, innerWidth int, bar, barInactive, fillPrefix string, isActive bool, styleFilled bool) {
	bodyStyle := e.Styles.Editor.QuestionBody
	prefixWidth := lipgloss.Width(fillPrefix)

	if isActive && e.fillIn.Focused() {
		e.fillIn.SetWidth(innerWidth - 2 - prefixWidth)
		indent := strings.Repeat(" ", prefixWidth)
		for j, tl := range strings.Split(e.fillIn.View(), "\n") {
			text := bar + fillPrefix + tl
			if j > 0 {
				text = barInactive + indent + tl
			}
			*lines = append(*lines, contentLine{text: text, fillInRow: j == 0, cursorItem: true})
		}
		return
	}

	val := strings.TrimSpace(e.fillIn.Value())
	if val != "" {
		rendered := e.Styles.Editor.QuestionUnselected.Render(val)
		if styleFilled {
			rendered = e.Styles.Editor.QuestionSelected.Render(val)
		}
		*lines = append(*lines, contentLine{text: bar + fillPrefix + rendered, cursorItem: isActive})
		return
	}
	*lines = append(*lines, contentLine{text: bar + fillPrefix + bodyStyle.Render("Something else?"), cursorItem: isActive})
}

// drawNote appends note rows to lines for the given key. When the
// note editor is active, renders the live textarea; otherwise shows
// saved note text or nothing.
func (e *questionEditor) drawNote(lines *[]contentLine, innerWidth int, bar, barInactive, noteKey string, isActive bool) {
	noteStyle := e.Styles.Editor.QuestionNote
	isEditing := e.activeNoteKey == noteKey && e.noteEditor.Focused()
	const notePrefix = "> "

	if isEditing && e.noteEditor.Focused() {
		prefixWidth := lipgloss.Width(notePrefix)
		e.noteEditor.SetWidth(innerWidth - 2 - prefixWidth)
		indent := strings.Repeat(" ", prefixWidth)
		for j, tl := range strings.Split(e.noteEditor.View(), "\n") {
			text := bar + notePrefix + tl
			if j > 0 {
				text = barInactive + indent + tl
			}
			*lines = append(*lines, contentLine{text: text, noteRow: j == 0, cursorItem: true})
		}
		return
	}

	if saved, ok := e.notes[noteKey]; ok && saved != "" {
		dimmed := noteStyle.Render(saved)
		for _, ln := range strings.Split(dimmed, "\n") {
			*lines = append(*lines, contentLine{text: bar + notePrefix + ln, cursorItem: isActive})
		}
	}
}

// fillInCursor returns the hardware cursor position for the fill-in
// textarea when it's focused. areaMinX is the left edge of the
// content area; prefixWidth is the visual width of the "> " prompt.
func (e *questionEditor) fillInCursor(screenRow, areaMinX, prefixWidth int) *tea.Cursor {
	if !e.fillIn.Focused() {
		return nil
	}
	tc := e.fillIn.Cursor()
	if tc == nil {
		return nil
	}
	tc.X += areaMinX + 1 + prefixWidth
	tc.Y += screenRow
	return tc
}

// noteCursor returns the hardware cursor position for the note
// editor when it's focused.
func (e *questionEditor) noteCursor(screenRow, areaMinX, prefixWidth int) *tea.Cursor {
	if !e.noteEditor.Focused() {
		return nil
	}
	tc := e.noteEditor.Cursor()
	if tc == nil {
		return nil
	}
	tc.X += areaMinX + 1 + prefixWidth
	tc.Y += screenRow
	return tc
}

// hasNote reports whether a note exists for the given key.
func (e *questionEditor) hasNote(key string) bool {
	_, ok := e.notes[key]
	return ok
}

// drawStandaloneNote draws a note editor or saved note directly
// onto the screen (not via line list). Used by YesNo which doesn't
// use the line-list model. Returns the cursor or nil.
func (e *questionEditor) drawStandaloneNote(scr uv.Screen, area uv.Rectangle, y int, noteKey string) (*tea.Cursor, int) {
	const notePrefix = "> "

	if e.activeNoteKey != "" && e.noteEditor.Focused() {
		y++
		prefixWidth := lipgloss.Width(notePrefix)
		e.noteEditor.SetWidth(area.Dx() - 2 - prefixWidth)
		noteView := e.noteEditor.View()
		var cur *tea.Cursor
		for j, ln := range strings.Split(noteView, "\n") {
			text := notePrefix + ln
			if j > 0 {
				text = strings.Repeat(" ", prefixWidth) + ln
			}
			lines := drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, y+1), text)
			if j == 0 {
				if tc := e.noteEditor.Cursor(); tc != nil {
					tc.X += prefixWidth
					tc.Y += y - area.Min.Y
					cur = tc
				}
			}
			y += lines
		}
		return cur, y
	}

	if saved, ok := e.notes[noteKey]; ok && saved != "" {
		y++
		noteStyle := e.Styles.Editor.QuestionNote
		drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, y+1), notePrefix+noteStyle.Render(saved))
		y++
	}

	return nil, y
}
