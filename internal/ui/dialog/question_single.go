package dialog

import (
	"maps"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
)

// SingleChoice is an inline single-choice question component.
// It embeds choiceList for shared navigation, fill-in, and
// rendering scaffold.
type SingleChoice struct {
	choiceList

	keyEnter key.Binding

	lastResponse question.Answer
}

// NewSingleChoice creates a new inline single-choice component.
func NewSingleChoice(sty *styles.Styles, req question.Question) *SingleChoice {
	return &SingleChoice{
		choiceList: newChoiceList(sty, req),
		keyEnter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	}
}

// HandleKey processes key events. Returns true when the user has
// made a selection or dismissed the question.
func (d *SingleChoice) HandleKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	// Note editor takes priority when active.
	if d.activeNoteKey != "" && d.noteEditor.Focused() {
		cmd, handled := d.handleNoteKey(msg, d.keyClose, func() { d.closeNote(d.noteKey()) })
		if handled {
			return false, cmd
		}
	}

	if !d.fillIn.Focused() && d.activeNoteKey == "" {
		if idx := d.numberKeyIndex(msg); idx >= 0 {
			d.cursorIdx = idx
			return false, nil
		}
	}

	if done, cmd, handled := d.handleFillInFocused(msg, d.keyEnter, func() (bool, tea.Cmd) {
		d.fillIn.Blur()
		d.answer(d.respond())
		return true, nil
	}, func() (bool, tea.Cmd) {
		val := strings.TrimSpace(d.fillIn.Value())
		if val != "" {
			d.answer(d.respondFillIn(val))
			return true, nil
		}
		return false, nil
	}); handled {
		return done, cmd
	}

	switch {
	case key.Matches(msg, d.keyClose):
		d.answer(question.Answer{QuestionID: d.Request.ID})
		return true, nil
	case key.Matches(msg, d.keyEnter):
		d.answer(d.respond())
		return true, nil
	case key.Matches(msg, d.keyNote) && !d.fillIn.Focused():
		return false, d.openNote(d.noteKey())
	}
	if d.handleNavKey(msg) {
		return false, nil
	}
	return false, nil
}

func (d *SingleChoice) answer(resp question.Answer) {
	d.lastResponse = resp
}

// Response returns the last response. Used by QuestionForm to
// collect answers from child components.
func (d *SingleChoice) Response() question.Answer { return d.lastResponse }

// GetRequest returns the underlying question request.
func (d *SingleChoice) GetRequest() question.Question { return d.Request }

func (d *SingleChoice) respond() question.Answer {
	resp := question.Answer{QuestionID: d.Request.ID}
	if !d.isFillIn() && len(d.Request.Choices) > 0 {
		resp.SelectedIDs = []string{d.Request.Choices[d.cursorIdx].ID}
	}
	if len(d.notes) > 0 {
		resp.Notes = make(map[string]string, len(d.notes))
		maps.Copy(resp.Notes, d.notes)
	}
	return resp
}

func (d *SingleChoice) respondFillIn(text string) question.Answer {
	return question.Answer{QuestionID: d.Request.ID, FillInText: text}
}

// numKeyBinding returns a display-only binding showing the valid
// number shortcut range for the given choice count.
func numKeyBinding(n int) key.Binding {
	if n <= 0 {
		n = 1
	}
	if n > 9 {
		n = 9
	}
	label := "1-" + strconv.Itoa(n)
	return key.NewBinding(key.WithKeys("1-9"), key.WithHelp(label, "quick select"))
}

// ShortHelp returns key bindings for the status bar.
func (d *SingleChoice) ShortHelp() []key.Binding {
	if d.activeNoteKey != "" && d.noteEditor.Focused() {
		return []key.Binding{d.keyClose, key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "save note"))}
	}
	if d.isFillIn() && d.fillIn.Focused() {
		return []key.Binding{d.navUp, d.keyEnter, d.keyClose}
	}
	return []key.Binding{d.keyUp, d.keyDown, d.keyEnter, numKeyBinding(len(d.Request.Choices)), d.keyNote, d.keyClose}
}

func (d *SingleChoice) Height() int         { return d.height(choiceListMaxWidth + 4) }
func (d *SingleChoice) HeightChanged() bool { return d.heightChanged() }
func (d *SingleChoice) SetFocused(f bool)   { d.setFocused(f) }

// Draw renders the single-choice question directly to screen.
// Returns the cursor position relative to area, or nil.
func (d *SingleChoice) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	fillPrefix := d.Styles.Editor.QuestionBody.Render("> ")
	if d.isFillIn() && strings.TrimSpace(d.fillIn.Value()) != "" {
		fillPrefix = d.Styles.Editor.QuestionSelected.Render("> ")
	}

	innerWidth := min(area.Dx()-4, choiceListMaxWidth)
	unselectedHeader := d.Styles.Editor.QuestionUnselected
	selectedStyle := d.Styles.Editor.QuestionSelected

	return d.drawContent(scr, area, fillPrefix, func(i int, ch question.Choice, active bool) string {
		style := unselectedHeader
		if active {
			style = selectedStyle
		}
		barWidth := 2 // "┃ " or "  ", applied by buildLines
		return style.Render(wrapIndent(ch.Label, innerWidth-barWidth, ""))
	})
}
