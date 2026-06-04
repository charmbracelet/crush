package dialog

import (
	"image"
	"maps"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// YesNo is an inline yes/no confirmation component. For open-ended
// responses, use FreeText instead. Notes can be added via alt+n.
type YesNo struct {
	questionEditor
	Request    question.Question
	selectedNo bool
	focused    bool

	keyLeftRight key.Binding
	keyEnter     key.Binding
	keyYes       key.Binding
	keyNo        key.Binding
	keyClose     key.Binding

	lastResponse question.Answer
	lastWidth    int
}

// NewYesNo creates a new inline yes/no question component.
func NewYesNo(sty *styles.Styles, req question.Question) *YesNo {
	return &YesNo{
		questionEditor: newQuestionEditor(sty),
		Request:        req,
		selectedNo:     true, // Default to "No" for safety.
		keyLeftRight:   key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("←/→", "switch")),
		keyEnter:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		keyYes:         key.NewBinding(key.WithKeys("y", "Y"), key.WithHelp("y", "yes")),
		keyNo:          key.NewBinding(key.WithKeys("n", "N"), key.WithHelp("n", "no")),
		keyClose:       CloseKey,
	}
}

// HandleKey processes a key press. Returns true when the user has
// made a choice or dismissed the question.
func (d *YesNo) HandleKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	// Note editor takes priority when active.
	if d.activeNoteKey != "" && d.noteEditor.Focused() {
		cmd, handled := d.handleNoteKey(msg, d.keyClose, func() { d.closeNote("_question") })
		if handled {
			return false, cmd
		}
	}

	switch {
	case key.Matches(msg, CloseKey):
		d.answer(question.Answer{QuestionID: d.Request.ID})
		return true, nil
	case key.Matches(msg, d.keyLeftRight):
		d.selectedNo = !d.selectedNo
		return false, nil
	case key.Matches(msg, d.keyEnter):
		d.answer(d.respond(!d.selectedNo))
		return true, nil
	case key.Matches(msg, d.keyYes):
		d.answer(d.respond(true))
		return true, nil
	case key.Matches(msg, d.keyNo):
		d.answer(d.respond(false))
		return true, nil
	case key.Matches(msg, d.keyNote):
		return false, d.openNote("_question")
	}
	return false, nil
}

func (d *YesNo) answer(resp question.Answer) {
	d.lastResponse = resp
}

// Response returns the last response. Used by QuestionForm to
// collect answers from child components.
func (d *YesNo) Response() question.Answer { return d.lastResponse }

// GetRequest returns the underlying question request.
func (d *YesNo) GetRequest() question.Question { return d.Request }

// ShortHelp returns key bindings for the status bar help display.
func (d *YesNo) ShortHelp() []key.Binding {
	if d.activeNoteKey != "" && d.noteEditor.Focused() {
		return []key.Binding{d.keyClose, key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "save note"))}
	}
	return []key.Binding{d.keyLeftRight, d.keyEnter, d.keyYes, d.keyNo, d.keyNote}
}

func (d *YesNo) respond(yes bool) question.Answer {
	resp := question.Answer{
		QuestionID: d.Request.ID,
		Yes:        &yes,
	}
	if len(d.notes) > 0 {
		resp.Notes = make(map[string]string, len(d.notes))
		maps.Copy(resp.Notes, d.notes)
	}
	return resp
}

// Height returns the visual height at the default max width.
// Pure function — no render-time state.
func (d *YesNo) Height() int {
	w := d.lastWidth
	if w <= 0 {
		w = choiceListMaxWidth
	}
	iconPrompt := questionIconPrompt(d.Styles, d.focused)
	h := sectionHeight(d.Request.Text, w-lipgloss.Width(iconPrompt)) // question
	h++                                                              // blank
	if d.Request.Description != "" {
		r := common.MarkdownRenderer(d.Styles, w)
		mu := common.LockMarkdownRenderer(r)
		mu.Lock()
		out, err := r.Render(d.Request.Description)
		mu.Unlock()
		if err == nil {
			out = strings.TrimSuffix(out, "\n")
			h += strings.Count(out, "\n") + 1
		} else {
			h += sectionHeight(d.Request.Description, w)
		}
		h++ // blank
	}
	h++ // buttons
	// Note line if present.
	if d.activeNoteKey != "" || len(d.notes) > 0 {
		h++
	}
	return h
}

// Draw renders the yes/no question directly to screen.
// Returns the cursor position when the note editor is active, or nil.
func (d *YesNo) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	d.lastWidth = area.Dx()
	y := area.Min.Y

	// Draw question header.
	iconPrompt := questionIconPrompt(d.Styles, d.focused)
	qText := iconPrompt + d.Styles.Editor.QuestionUnselected.Render(
		ansi.Wrap(d.Request.Text, area.Dx()-lipgloss.Width(iconPrompt), ""),
	)
	y += drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), qText)
	y++ // blank

	// Draw optional description.
	if d.Request.Description != "" {
		r := common.MarkdownRenderer(d.Styles, area.Dx())
		mu := common.LockMarkdownRenderer(r)
		mu.Lock()
		desc, err := r.Render(d.Request.Description)
		mu.Unlock()
		if err == nil {
			desc = strings.TrimSuffix(desc, "\n")
			y += drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), desc)
		} else {
			y += drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), d.Request.Description)
		}
		y++ // blank
	}

	// Draw buttons.
	buttonOpts := []common.ButtonOpts{
		{Text: "Yes", Selected: !d.selectedNo, Padding: 3},
		{Text: "No", Selected: d.selectedNo, Padding: 3},
	}
	buttons := common.ButtonGroup(d.Styles, buttonOpts, " ")
	y += drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), buttons)

	// Draw note editor or saved note.
	cur, _ := d.drawStandaloneNote(scr, area, y, "_question")
	return cur
}

// HeightChanged always returns false — Height is now pure.
func (d *YesNo) HeightChanged() bool { return false }

// SetFocused updates the icon style based on whether the editor
// area is focused.
func (d *YesNo) SetFocused(focused bool) { d.focused = focused }
