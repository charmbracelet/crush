package dialog

import (
	"fmt"
	"maps"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
)

// MultiChoice is an inline multi-choice question component.
// It embeds choiceList for shared navigation, fill-in, and
// rendering scaffold.
type MultiChoice struct {
	choiceList

	selected  map[int]bool
	keyToggle key.Binding
	keyDone   key.Binding

	lastResponse question.Answer
}

// NewMultiChoice creates a new inline multi-choice component.
func NewMultiChoice(sty *styles.Styles, req question.Question) *MultiChoice {
	cl := newChoiceList(sty, req)
	cl.styleFillInAsSelected = true
	return &MultiChoice{
		choiceList: cl,
		selected:   make(map[int]bool),
		keyToggle:  key.NewBinding(key.WithKeys(" ", "space"), key.WithHelp("space", "toggle")),
		keyDone:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "done")),
	}
}

// HandleKey processes key events. Returns true when the user has
// submitted or dismissed the question.
func (d *MultiChoice) HandleKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	// Note editor takes priority when active.
	if d.activeNoteKey != "" && d.noteEditor.Focused() {
		cmd, handled := d.handleNoteKey(msg, d.keyClose, func() { d.closeNote(d.noteKey()) })
		if handled {
			return false, cmd
		}
	}

	if !d.fillIn.Focused() && d.activeNoteKey == "" {
		if idx := d.numberKeyIndex(msg); idx >= 0 {
			d.mouseActive = false
			d.cursorIdx = idx
			d.selected[idx] = !d.selected[idx]
			if !d.selected[idx] {
				delete(d.selected, idx)
			}
			return false, nil
		}
	}

	if done, cmd, handled := d.handleFillInFocused(msg, d.keyDone, func() (bool, tea.Cmd) {
		d.fillIn.Blur()
		return false, nil
	}, func() (bool, tea.Cmd) {
		val := strings.TrimSpace(d.fillIn.Value())
		if val != "" {
			d.answer(d.respond())
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
	case key.Matches(msg, d.keyDone):
		if d.isFillIn() && !d.fillIn.Focused() {
			d.fillIn.Focus()
			return false, d.fillIn.Focus()
		}
		d.answer(d.respond())
		return true, nil
	case key.Matches(msg, d.keyToggle):
		if d.isFillIn() {
			if !d.fillIn.Focused() {
				d.fillIn.Focus()
				return false, d.fillIn.Focus()
			}
			var cmd tea.Cmd
			d.fillIn, cmd = d.fillIn.Update(msg)
			return false, cmd
		}
		d.selected[d.cursorIdx] = !d.selected[d.cursorIdx]
		if !d.selected[d.cursorIdx] {
			delete(d.selected, d.cursorIdx)
		}
	case key.Matches(msg, d.keyNote) && !d.fillIn.Focused():
		return false, d.openNote(d.noteKey())
	}
	if d.handleNavKey(msg) {
		return false, nil
	}
	return false, nil
}

func (d *MultiChoice) answer(resp question.Answer) {
	d.lastResponse = resp
}

// Response returns the last response. Used by QuestionForm to
// collect answers from child components.
// Response returns the current answer, reflecting live toggle
// state so that tabbing away preserves selections.
func (d *MultiChoice) Response() question.Answer { return d.respond() }

// GetRequest returns the underlying question request.
func (d *MultiChoice) GetRequest() question.Question { return d.Request }

func (d *MultiChoice) respond() question.Answer {
	resp := question.Answer{QuestionID: d.Request.ID}
	for i := range d.Request.Choices {
		if d.selected[i] {
			resp.SelectedIDs = append(resp.SelectedIDs, d.Request.Choices[i].ID)
		}
	}
	val := strings.TrimSpace(d.fillIn.Value())
	if val != "" {
		resp.FillInText = val
	}
	if len(d.notes) > 0 {
		resp.Notes = make(map[string]string, len(d.notes))
		maps.Copy(resp.Notes, d.notes)
	}
	return resp
}

// ShortHelp returns key bindings for the status bar.
func (d *MultiChoice) ShortHelp() []key.Binding {
	if d.activeNoteKey != "" && d.noteEditor.Focused() {
		return []key.Binding{d.keyClose, key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "save note"))}
	}
	if d.isFillIn() && d.fillIn.Focused() {
		return []key.Binding{d.navUp, d.keyDone, d.keyClose}
	}
	return []key.Binding{d.keyUp, d.keyDown, d.keyToggle, numKeyBinding(len(d.Request.Choices)), d.keyNote, d.keyDone, d.keyClose}
}

func (d *MultiChoice) Height() int         { return d.height(choiceListMaxWidth + 4) }
func (d *MultiChoice) HeightChanged() bool { return d.heightChanged() }
func (d *MultiChoice) SetFocused(f bool)   { d.setFocused(f) }
func (d *MultiChoice) SetHover(x, y int)   { d.setHover(x, y) }
func (d *MultiChoice) HandlePaste(msg tea.PasteMsg) tea.Cmd {
	return d.handlePaste(msg)
}

// HandleMouseClick checks if the click landed on a choice item and
// toggles it, or focuses the fill-in. Returns done=false since
// multi-choice requires explicit submission.
func (d *MultiChoice) HandleMouseClick(x, y int) (bool, bool) {
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
	if idx == len(d.Request.Choices) {
		// Fill-in item.
		d.cursorIdx = idx
		d.mouseActive = false
		d.suppressScroll = true
		d.fillIn.Focus()
		return false, true
	}
	if idx >= 0 && idx < len(d.Request.Choices) {
		d.cursorIdx = idx
		d.mouseActive = false
		d.suppressScroll = true
		d.fillIn.Blur()
		d.selected[idx] = !d.selected[idx]
		if !d.selected[idx] {
			delete(d.selected, idx)
		}
		return false, true
	}
	return false, false
}

// Draw renders the multi-choice question directly to screen.
// Returns the cursor position relative to area, or nil.
func (d *MultiChoice) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	fillPrefix := d.Styles.Editor.QuestionBody.Render("> ")
	if strings.TrimSpace(d.fillIn.Value()) != "" {
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
		check := d.Styles.Editor.QuestionCheckOff.Render() + " "
		if d.selected[i] {
			check = d.Styles.Editor.QuestionCheckOn.Render() + " "
		}
		checkWidth := lipgloss.Width(check)
		barWidth := 2 // "┃ " or "  ", applied by buildLines
		labelIndent := strings.Repeat(" ", checkWidth)
		return check + style.Render(wrapIndent(ch.Label, innerWidth-barWidth-checkWidth, labelIndent))
	})
}
