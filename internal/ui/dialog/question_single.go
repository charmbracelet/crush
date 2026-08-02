package dialog

import (
	"fmt"
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
		cmd, handled := d.handleNoteKey(msg, d.keyClose, func() {
			// Leaving the note editor drops hover mode so arrow keys
			// move relative to the noted item, not a hovered choice.
			d.mouseActive = false
			d.closeNote(d.noteKey())
		})
		if handled {
			return false, cmd
		}
	}

	if !d.fillIn.Focused() && d.activeNoteKey == "" {
		if idx := d.numberKeyIndex(msg); idx >= 0 {
			d.mouseActive = false
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
		// In hover mode, act on the item under the mouse. Landing on
		// the fill-in focuses it (mirroring a click) instead of
		// submitting an empty answer.
		if d.mouseActive {
			d.adoptHover()
			if d.isFillIn() {
				return false, d.fillIn.Focus()
			}
		}
		d.answer(d.respond())
		return true, nil
	case key.Matches(msg, d.keyNote) && !d.fillIn.Focused():
		d.adoptHover()
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
// Response returns the current answer, reflecting live cursor
// and fill-in state so that tabbing away preserves selections.
func (d *SingleChoice) Response() question.Answer { return d.respond() }

// GetRequest returns the underlying question request.
func (d *SingleChoice) GetRequest() question.Question { return d.Request }

func (d *SingleChoice) respond() question.Answer {
	resp := question.Answer{QuestionID: d.Request.ID}
	if !d.isFillIn() && len(d.Request.Choices) > 0 {
		resp.SelectedIDs = []string{d.Request.Choices[d.cursorIdx].ID}
	}
	if val := strings.TrimSpace(d.fillIn.Value()); val != "" {
		resp.FillInText = val
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

func (d *SingleChoice) Height(width int) int { return d.height(width) }
func (d *SingleChoice) HeightChanged() bool  { return d.heightChanged() }
func (d *SingleChoice) SetFocused(f bool)    { d.setFocused(f) }
func (d *SingleChoice) SetHover(x, y int)    { d.setHover(x, y) }
func (d *SingleChoice) HandlePaste(msg tea.PasteMsg) tea.Cmd {
	return d.handlePaste(msg)
}

// HandleMouseClick checks if the click landed on a choice item and
// selects it. Does not advance — user can change their selection
// before pressing Enter or clicking another option.
func (d *SingleChoice) HandleMouseClick(x, y int) (bool, bool) {
	if d.choiceCompositor == nil {
		return false, false
	}
	hit := d.choiceCompositor.Hit(x, y)
	if hit.Empty() {
		return false, false
	}
	var idx int
	if _, err := fmt.Sscanf(hit.ID(), "choice_%d", &idx); err != nil {
		return false, false
	}
	if idx >= 0 && idx < len(d.Request.Choices) {
		d.cursorIdx = idx
		d.mouseActive = false
		d.suppressScroll = true
		d.fillIn.Blur()
		d.answer(d.respond())
		return false, true
	}
	if idx == len(d.Request.Choices) {
		// Fill-in: focus but don't submit.
		d.cursorIdx = idx
		d.mouseActive = false
		d.suppressScroll = true
		d.fillIn.Focus()
		return false, true
	}
	return false, false
}

// Draw renders the single-choice question directly to screen.
// Returns the cursor position relative to area, or nil.
func (d *SingleChoice) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	// The fill-in prompt goes pink when the fill-in is the selected
	// item, mirroring how a selected choice's label is styled. This
	// lets the pink prompt carry the selection so the gutter bar
	// doesn't have to stay lit for it.
	fillPrefix := d.Styles.Editor.QuestionBody.Render("> ")
	if d.isFillIn() {
		fillPrefix = d.Styles.Editor.QuestionSelected.Render("> ")
	}

	unselectedHeader := d.Styles.Editor.QuestionUnselected
	selectedStyle := d.Styles.Editor.QuestionSelected

	return d.drawContent(scr, area, fillPrefix, func(i int, ch question.Choice, active bool, innerWidth int) string {
		isSelected := false
		if len(d.lastResponse.SelectedIDs) > 0 {
			isSelected = d.lastResponse.SelectedIDs[0] == ch.ID
		}
		style := unselectedHeader
		// In hover mode the committed choice stays pink so the user
		// can see their selection while the bar tracks the mouse.
		// While the fill-in is focused, though, the fill-in is the
		// active answer, so suppress the choice pink to avoid the
		// selection appearing to jump onto a choice mid-edit.
		if active || (isSelected && d.mouseActive && !d.fillIn.Focused()) {
			style = selectedStyle
		}
		barWidth := 2 // "┃ " or "  ", applied by buildLines
		return style.Render(wrapIndent(ch.Label, innerWidth-barWidth, ""))
	})
}
