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

	editor       textarea.Model
	scrollOffset int  // lines scrolled past the top of the textarea viewport
	wheelActive  bool // wheel-scroll mode: skip cursor-follow until next key press
	keyEnter     key.Binding
	keyClose     key.Binding

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
	d.wheelActive = false
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

// Response returns the current answer, including any unsaved
// editor content so that tabbing away preserves typed text.
func (d *FreeText) Response() question.Answer {
	if val := strings.TrimSpace(d.editor.Value()); val != "" {
		return question.Answer{QuestionID: d.Request.ID, FillInText: val}
	}
	return d.lastResponse
}

// GetRequest returns the underlying question request.
func (d *FreeText) GetRequest() question.Question { return d.Request }

// ShortHelp returns key bindings for the status bar.
func (d *FreeText) ShortHelp() []key.Binding {
	return []key.Binding{d.keyEnter, d.keyClose}
}

// Height returns the visual height at the default max width.
func (d *FreeText) Height(width int) int {
	w := width
	if w <= 0 {
		w = d.lastWidth
	}
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
	h++                    // trailing blank for bottom padding
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

	// Draw textarea with > prompt prefix, viewport clipping, and scrollbar.
	promptPrefix := d.Styles.Editor.QuestionBody.Render("> ")
	prefixWidth := lipgloss.Width(promptPrefix)

	// Available height for the textarea viewport (subtract trailing padding).
	viewport := area.Max.Y - y - 1
	if viewport < 1 {
		viewport = 1
	}

	// Pre-compute whether we need a scrollbar column by estimating
	// line count at full width first. This avoids the double-render
	// that caused width oscillation.
	fullWidth := min(area.Dx()-2-prefixWidth, choiceListMaxWidth)
	d.editor.SetWidth(fullWidth)
	view := d.editor.View()
	viewLines := strings.Split(view, "\n")
	totalLines := len(viewLines)

	overflow := totalLines > viewport
	textWidth := area.Dx()
	sbX := 0
	if overflow {
		textWidth-- // reserve 1 column for scrollbar
		sbX = area.Min.X + textWidth
		// Re-render at narrower width only if it changed.
		narrowWidth := min(textWidth-2-prefixWidth, choiceListMaxWidth)
		if narrowWidth != fullWidth {
			d.editor.SetWidth(narrowWidth)
			view = d.editor.View()
			viewLines = strings.Split(view, "\n")
			totalLines = len(viewLines)
		}
	}

	// Clamp scroll offset to valid bounds.
	maxScroll := max(0, totalLines-viewport)
	d.scrollOffset = min(max(0, d.scrollOffset), maxScroll)

	// Auto-scroll to keep cursor visible, unless in wheel-scroll mode.
	if !d.wheelActive {
		if tc := d.editor.Cursor(); tc != nil {
			cursorLine := tc.Y
			if cursorLine < d.scrollOffset {
				d.scrollOffset = cursorLine
			} else if cursorLine >= d.scrollOffset+viewport {
				d.scrollOffset = cursorLine - viewport + 1
			}
		}
		// Re-clamp after cursor adjustment.
		d.scrollOffset = min(max(0, d.scrollOffset), maxScroll)
	}
	// Don't clear wheelActive here; it persists until the next key press.

	var cur *tea.Cursor
	for screenRow := range viewport {
		lineIdx := d.scrollOffset + screenRow
		if lineIdx >= totalLines {
			break
		}
		ln := viewLines[lineIdx]
		text := promptPrefix + ln
		if lineIdx > 0 {
			text = strings.Repeat(" ", prefixWidth) + ln
		}
		drawStyledText(scr, image.Rect(area.Min.X, y, area.Min.X+textWidth, y+1), text)

		// Cursor: only on the line matching the editor's cursor row.
		if tc := d.editor.Cursor(); tc != nil && tc.Y == lineIdx {
			tc.X += prefixWidth
			tc.Y = y - area.Min.Y
			if tc.X < textWidth {
				cur = tc
			}
		}
		y++
	}

	// Scrollbar.
	if overflow {
		sb := common.Scrollbar(d.Styles, viewport, totalLines, viewport, d.scrollOffset)
		if sb != "" {
			uv.NewStyledString(sb).Draw(scr, image.Rect(sbX, area.Max.Y-viewport-1, sbX+1, area.Max.Y-1))
		}
	}

	// Clamp cursor X to visible text width.
	if cur != nil {
		if cur.X < 0 {
			cur.X = 0
		} else if cur.X >= textWidth {
			cur.X = textWidth - 1
		}
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

// SetHover is a no-op for free text questions.
func (d *FreeText) SetHover(x, y int) {}

// HandleMouseClick is a no-op for free text questions.
func (d *FreeText) HandleMouseClick(x, y int) (bool, bool) { return false, false }

// HandleWheel scrolls the textarea viewport.
func (d *FreeText) HandleWheel(deltaX, deltaY float64) {
	if deltaY != 0 {
		d.scrollOffset += int(deltaY)
		d.wheelActive = true
	}
}

// HandlePaste forwards paste events to the editor textarea.
func (d *FreeText) HandlePaste(msg tea.PasteMsg) tea.Cmd {
	var cmd tea.Cmd
	d.editor, cmd = d.editor.Update(msg)
	return cmd
}
