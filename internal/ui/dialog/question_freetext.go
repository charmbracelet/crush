package dialog

import (
	"image"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// FreeText is an open-ended text input component for questions
// that need a narrative answer rather than a selection.
type FreeText struct {
	Styles  *styles.Styles
	Request question.Question
	focused bool

	editor   textarea.Model
	keyEnter key.Binding
	keyClose key.Binding

	lastResponse question.Answer
	lastWidth    int
}

// NewFreeText creates a new free-text question component.
func NewFreeText(sty *styles.Styles, req question.Question) *FreeText {
	ta := newQuestionTextarea(sty, "Type your answer...", 1000)
	ta.MinHeight = 3
	ta.MaxHeight = 8
	ta.SetHeight(3)

	return &FreeText{
		Styles:   sty,
		Request:  req,
		editor:   ta,
		keyEnter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit")),
		keyClose: CloseKey,
	}
}

// HandleKey processes a key press. Returns true when the user has
// submitted or dismissed the question.
func (d *FreeText) HandleKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	switch {
	case key.Matches(msg, d.keyClose):
		d.answer(question.Answer{QuestionID: d.Request.ID})
		return true, nil
	case key.Matches(msg, d.keyEnter):
		val := strings.TrimSpace(d.editor.Value())
		if val != "" {
			d.answer(question.Answer{
				QuestionID: d.Request.ID,
				FillInText: val,
			})
			return true, nil
		}
		return false, nil
	default:
		var cmd tea.Cmd
		d.editor, cmd = d.editor.Update(msg)
		return false, cmd
	}
}

func (d *FreeText) answer(resp question.Answer) {
	d.lastResponse = resp
}

// Response returns the last response.
func (d *FreeText) Response() question.Answer { return d.lastResponse }

// GetRequest returns the underlying question request.
func (d *FreeText) GetRequest() question.Question { return d.Request }

// ShortHelp returns key bindings for the status bar.
func (d *FreeText) ShortHelp() []key.Binding {
	return []key.Binding{d.keyEnter, d.keyClose}
}

// Height returns the visual height at the default max width.
func (d *FreeText) Height() int {
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
	h += d.editor.Height() // textarea
	return h
}

// Draw renders the free-text question directly to screen.
// Returns the cursor position, or nil.
func (d *FreeText) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
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

	// Draw textarea with > prompt prefix.
	promptPrefix := d.Styles.Editor.QuestionBody.Render("> ")
	prefixWidth := lipgloss.Width(promptPrefix)
	d.editor.SetWidth(min(area.Dx()-2-prefixWidth, choiceListMaxWidth))
	view := d.editor.View()
	var cur *tea.Cursor
	for j, ln := range strings.Split(view, "\n") {
		text := promptPrefix + ln
		if j > 0 {
			text = strings.Repeat(" ", prefixWidth) + ln
		}
		drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, y+1), text)
		if j == 0 {
			if tc := d.editor.Cursor(); tc != nil {
				tc.X += prefixWidth
				tc.Y += y - area.Min.Y
				cur = tc
			}
		}
		y++
	}

	return cur
}

// HeightChanged reports whether the textarea height changed.
func (d *FreeText) HeightChanged() bool { return false }

// SetFocused updates focus state.
func (d *FreeText) SetFocused(focused bool) {
	d.focused = focused
	if focused {
		d.editor.Focus()
	} else {
		d.editor.Blur()
	}
}
