package dialog

import (
	"image"
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

// ConfirmComponent is the confirmation tab shown at the end of a
// multi-question batch. It displays an answer summary and lets the
// user confirm or go back to editing. Implements questionResponder
// so QuestionForm treats it like any other tab.
type ConfirmComponent struct {
	Styles           *styles.Styles
	Title            string
	Description      string
	QuestionLabels   []string
	QuestionRequests []question.Question
	Answers          []*question.Answer
	confirmYes       bool

	keyLeft  key.Binding
	keyRight key.Binding
	keyEnter key.Binding
	keyClose key.Binding

	focused   bool
	lastWidth int

	// OnConfirm is called when the user confirms.
	OnConfirm func()
	// OnReject is called when the user says "not yet".
	OnReject func()
}

// NewConfirmComponent creates a new confirmation component.
func NewConfirmComponent(sty *styles.Styles, title, description string, labels []string, requests []question.Question, answers []*question.Answer) *ConfirmComponent {
	if title == "" || title == "Confirm" {
		title = "Ready to go?"
	}
	return &ConfirmComponent{
		Styles:           sty,
		Title:            title,
		Description:      description,
		QuestionLabels:   labels,
		QuestionRequests: requests,
		Answers:          answers,
		confirmYes:       true,
		keyLeft:          key.NewBinding(key.WithKeys("left"), key.WithHelp("←/→", "switch")),
		keyRight:         key.NewBinding(key.WithKeys("right"), key.WithHelp("←/→", "switch")),
		keyEnter:         key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		keyClose:         CloseKey,
	}
}

// HandleKey processes input on the confirm tab. Returns true when
// the user has confirmed submission. Tab/shift+tab are NOT handled
// here; QuestionForm intercepts them for tab navigation.
func (c *ConfirmComponent) HandleKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	switch {
	case key.Matches(msg, c.keyLeft), key.Matches(msg, c.keyRight):
		c.confirmYes = !c.confirmYes
		return false, nil
	case key.Matches(msg, c.keyEnter):
		if c.confirmYes {
			if c.OnConfirm != nil {
				c.OnConfirm()
			}
			return true, nil
		}
		if c.OnReject != nil {
			c.OnReject()
		}
		return false, nil
	case key.Matches(msg, c.keyClose):
		if c.OnReject != nil {
			c.OnReject()
		}
		return false, nil
	}
	return false, nil
}

// Response returns an empty response. The confirm tab doesn't
// produce a question answer; it controls form submission.
func (c *ConfirmComponent) Response() question.Answer {
	return question.Answer{}
}

// ShortHelp returns key bindings for the status bar.
func (c *ConfirmComponent) ShortHelp() []key.Binding {
	return []key.Binding{c.keyLeft, c.keyEnter, c.keyClose}
}

// Height returns the visual height of the confirm content.
func (c *ConfirmComponent) Height() int {
	w := c.lastWidth
	if w <= 0 {
		w = choiceListMaxWidth
	}
	iconPrompt := questionIconPrompt(c.Styles, c.focused)
	h := sectionHeight(c.Title, w-lipgloss.Width(iconPrompt)) // title
	h++                                                       // blank
	if c.Description != "" {
		r := common.MarkdownRenderer(c.Styles, w)
		mu := common.LockMarkdownRenderer(r)
		mu.Lock()
		out, err := r.Render(c.Description)
		mu.Unlock()
		if err == nil {
			out = strings.TrimSuffix(out, "\n")
			h += strings.Count(out, "\n") + 1
		} else {
			h += sectionHeight(c.Description, w)
		}
		h++ // blank
	}
	h += len(c.QuestionLabels) // one bullet per question
	h++                        // blank
	h++                        // buttons
	h++                        // bottom margin
	return h
}

// Draw renders the confirmation content.
func (c *ConfirmComponent) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	c.lastWidth = area.Dx()
	y := area.Min.Y

	// Title with ? icon prompt, using confirm style.
	iconPrompt := questionIconPrompt(c.Styles, c.focused)
	qText := iconPrompt + c.Styles.Editor.QuestionConfirm.Render(
		ansi.Wrap(c.Title, area.Dx()-lipgloss.Width(iconPrompt), ""),
	)
	y += drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), qText)
	y++ // blank

	// Description.
	if c.Description != "" {
		r := common.MarkdownRenderer(c.Styles, area.Dx())
		mu := common.LockMarkdownRenderer(r)
		mu.Lock()
		desc, err := r.Render(c.Description)
		mu.Unlock()
		if err == nil {
			desc = strings.TrimSuffix(desc, "\n")
			y += drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), desc)
		} else {
			y += drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), c.Description)
		}
		y++ // blank
	}

	// Answer summary bullets in description/body style.
	bulletStyle := c.Styles.Editor.QuestionBody
	for i, label := range c.QuestionLabels {
		summary := c.answerSummary(i)
		bullet := bulletStyle.Render("• " + label + ": " + summary)
		y += drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), bullet)
	}
	y++ // blank

	// Buttons.
	buttonOpts := []common.ButtonOpts{
		{Text: "Yup!", Selected: c.confirmYes, Padding: 3, UnderlineIndex: -1},
		{Text: "Not yet", Selected: !c.confirmYes, Padding: 3, UnderlineIndex: -1},
	}
	buttons := common.ButtonGroup(c.Styles, buttonOpts, " ")
	drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), buttons)

	return nil
}

// HeightChanged always returns false.
func (c *ConfirmComponent) HeightChanged() bool { return false }

// SetFocused updates focus state.
func (c *ConfirmComponent) SetFocused(focused bool) { c.focused = focused }

// UpdateAnswers replaces the answer slice. Called by QuestionForm
// when tabbing away from a question so the summary stays current.
func (c *ConfirmComponent) UpdateAnswers(answers []*question.Answer) {
	c.Answers = answers
}

// answerSummary returns a human-readable summary of an answer.
// Choice IDs are resolved to display labels when possible.
func (c *ConfirmComponent) answerSummary(idx int) string {
	if idx >= len(c.Answers) || c.Answers[idx] == nil {
		return "(not answered)"
	}
	resp := c.Answers[idx]
	if resp.FillInText != "" {
		return resp.FillInText
	}
	if resp.Yes != nil {
		if *resp.Yes {
			return "Yes"
		}
		return "No"
	}
	if len(resp.SelectedIDs) > 0 {
		labels := make([]string, 0, len(resp.SelectedIDs))
		for _, id := range resp.SelectedIDs {
			labels = append(labels, c.choiceLabel(idx, id))
		}
		return strings.Join(labels, ", ")
	}
	return "(not answered)"
}

// choiceLabel resolves a choice ID to its display label.
func (c *ConfirmComponent) choiceLabel(qIdx int, choiceID string) string {
	if qIdx < len(c.QuestionRequests) {
		for _, ch := range c.QuestionRequests[qIdx].Choices {
			if ch.ID == choiceID {
				return ch.Label
			}
		}
	}
	return choiceID
}
