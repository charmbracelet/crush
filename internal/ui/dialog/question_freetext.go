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
//
// The question header, description, and textarea share a single
// scroll buffer so the title scrolls away as the answer grows,
// matching the choice and confirm components. This keeps the
// textarea from being permanently squeezed by fixed header rows.
func (d *FreeText) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	d.lastWidth = area.Dx()
	viewport := area.Dy()

	promptPrefix := d.Styles.Editor.QuestionBody.Render("> ")
	prefixWidth := lipgloss.Width(promptPrefix)
	iconPrompt := questionIconPrompt(d.Styles, d.focused)
	iconWidth := lipgloss.Width(iconPrompt)

	// ftLine is a single buffer row. cursorX >= 0 marks the row
	// carrying the textarea cursor and its column (incl. prefix).
	type ftLine struct {
		text    string
		cursorX int
	}

	// build renders the full content (header, description, textarea)
	// into a flat line buffer at the given content width. Returns
	// the buffer and the row index of the cursor (-1 if none).
	build := func(contentWidth int) ([]ftLine, int) {
		var lines []ftLine
		cursorRow := -1

		header := iconPrompt + d.Styles.Editor.QuestionUnselected.Render(
			ansi.Wrap(d.Request.Text, contentWidth-iconWidth, ""),
		)
		for _, l := range strings.Split(header, "\n") {
			lines = append(lines, ftLine{text: l, cursorX: -1})
		}
		lines = append(lines, ftLine{cursorX: -1}) // blank

		if d.Request.Description != "" {
			r := common.MarkdownRenderer(d.Styles, contentWidth)
			mu := common.LockMarkdownRenderer(r)
			mu.Lock()
			desc, err := r.Render(d.Request.Description)
			mu.Unlock()
			if err != nil {
				desc = d.Request.Description
			}
			desc = strings.TrimSuffix(desc, "\n")
			for _, l := range strings.Split(desc, "\n") {
				lines = append(lines, ftLine{text: l, cursorX: -1})
			}
			lines = append(lines, ftLine{cursorX: -1}) // blank
		}

		d.editor.SetWidth(contentWidth - 2 - prefixWidth)
		tc := d.editor.Cursor()
		for j, ln := range strings.Split(d.editor.View(), "\n") {
			text := promptPrefix + ln
			if j > 0 {
				text = strings.Repeat(" ", prefixWidth) + ln
			}
			cursorX := -1
			if tc != nil && tc.Y == j {
				cursorRow = len(lines)
				cursorX = tc.X + prefixWidth
			}
			lines = append(lines, ftLine{text: text, cursorX: cursorX})
		}
		lines = append(lines, ftLine{cursorX: -1}) // trailing bottom padding, matches Height()
		return lines, cursorRow
	}

	// Build at full width; reserve a scrollbar column and rebuild
	// only if the content overflows the viewport.
	contentWidth := area.Dx()
	lines, cursorRow := build(contentWidth)
	overflow := viewport > 0 && len(lines) > viewport
	if overflow {
		contentWidth--
		lines, cursorRow = build(contentWidth)
	}

	// Clamp scroll, then keep the cursor row visible unless the
	// user is wheel-scrolling.
	maxScroll := max(0, len(lines)-viewport)
	d.scrollOffset = min(max(0, d.scrollOffset), maxScroll)
	if !d.wheelActive && cursorRow >= 0 {
		if cursorRow < d.scrollOffset {
			d.scrollOffset = cursorRow
		} else if cursorRow >= d.scrollOffset+viewport {
			d.scrollOffset = cursorRow - viewport + 1
		}
		d.scrollOffset = min(max(0, d.scrollOffset), maxScroll)
	}

	// Blit the visible window and place the cursor. The cursor is
	// returned relative to the area's top-left, matching the
	// InlineEditor contract.
	var cur *tea.Cursor
	baseCursor := d.editor.Cursor()
	for screenRow := range viewport {
		idx := d.scrollOffset + screenRow
		if idx >= len(lines) {
			break
		}
		ln := lines[idx]
		y := area.Min.Y + screenRow
		drawStyledText(scr, image.Rect(area.Min.X, y, area.Min.X+contentWidth, y+1), ln.text)
		if ln.cursorX >= 0 && ln.cursorX < contentWidth && baseCursor != nil {
			c := *baseCursor
			c.X = ln.cursorX
			c.Y = screenRow
			cur = &c
		}
	}

	// Scrollbar.
	if overflow {
		sb := common.Scrollbar(d.Styles, viewport, len(lines), viewport, d.scrollOffset)
		if sb != "" {
			x := area.Max.X - 1
			uv.NewStyledString(sb).Draw(scr, image.Rect(x, area.Min.Y, x+1, area.Min.Y+viewport))
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
