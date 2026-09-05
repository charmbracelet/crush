package dialog

import (
	"image"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

// PlanHandoffInline is a small inline prompt rendered at the bottom of the
// editor area when the plan agent signals it is ready for execution.
// It replaces the textarea temporarily, asking the user to switch to code
// mode or request changes to the plan.
type PlanHandoffInline struct {
	com                    *common.Common
	requestChangesSelected bool
	editing                bool
	focused                bool
	compositor             *lipgloss.Compositor
	hoverX                 int
	hoverY                 int
	editor                 textarea.Model
	editorTextArea         image.Rectangle
	selectionAnchor        planHandoffSelectionPoint
	selectionHead          planHandoffSelectionPoint
	selectionSet           bool
	selecting              bool

	heightChanged bool

	// OnConfirm is called when the user confirms switching to code mode.
	// The returned tea.Cmd is queued by the UI to perform the switch and
	// start the coder agent.
	OnConfirm func() tea.Cmd
	// OnRequestChanges is called with the user's feedback when they submit it.
	OnRequestChanges func(string) tea.Cmd

	pendingCmd tea.Cmd // set during mouse confirmation; retrieved via PendingCmd

	keyLeftRight key.Binding
	keyEnter     key.Binding
	keyNewline   key.Binding
	keyYes       key.Binding
	keyNo        key.Binding
	keyClose     key.Binding
}

var (
	_ InlineEditor            = (*PlanHandoffInline)(nil)
	_ CollapsibleInlineEditor = (*PlanHandoffInline)(nil)
	_ ResizableInlineEditor   = (*PlanHandoffInline)(nil)
)

const (
	planHandoffQuestion        = "Ready to start coding?"
	planHandoffFeedbackPrompt  = "What should change?"
	planHandoffCollapsedPrompt = "Plan ready · Tab for actions"
)

type planHandoffChoiceLayout struct {
	question string
	buttons  []common.ButtonOpts
	spacing  string
	height   int
}

type planHandoffSelectionPoint struct {
	offset int
	row    int
	col    int
}

// NewPlanHandoffInline creates an inline plan handoff prompt. Wire its
// callbacks before setting it as the active inline editor.
func NewPlanHandoffInline(com *common.Common) *PlanHandoffInline {
	editor := newQuestionTextarea(com.Styles, "Describe the changes...", 1000)
	editor.MinHeight = 3
	editor.MaxHeight = 8
	editor.SetHeight(3)

	return &PlanHandoffInline{
		com:                    com,
		requestChangesSelected: false,
		editor:                 editor,
		keyLeftRight: key.NewBinding(
			key.WithKeys("left", "right"),
			key.WithHelp("←/→", "switch"),
		),
		keyEnter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		keyNewline: key.NewBinding(
			key.WithKeys("shift+enter", "ctrl+j"),
			key.WithHelp("ctrl+j", "newline"),
		),
		keyYes: key.NewBinding(
			key.WithKeys("y", "Y"),
			key.WithHelp("y", "start coding"),
		),
		keyNo: key.NewBinding(
			key.WithKeys("n", "N"),
			key.WithHelp("n", "revise plan"),
		),
		keyClose: CloseKey,
	}
}

// HandleKey processes a key press. Returns done=true when the user has
// made a choice. Returns a tea.Cmd to perform the mode switch when the
// user confirms.
func (p *PlanHandoffInline) HandleKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if p.editing {
		p.clearSelection()
		switch {
		case key.Matches(msg, p.keyClose):
			p.editing = false
			p.editor.Blur()
			p.heightChanged = true
			return false, nil
		case key.Matches(msg, p.keyNewline):
			previousHeight := p.editor.Height()
			p.editor.InsertRune('\n')
			var cmd tea.Cmd
			p.editor, cmd = p.editor.Update(msg)
			p.heightChanged = p.heightChanged || previousHeight != p.editor.Height()
			return false, cmd
		case key.Matches(msg, p.keyEnter):
			feedback := strings.TrimSpace(p.editor.Value())
			if feedback == "" {
				return false, nil
			}
			if p.OnRequestChanges != nil {
				return true, p.OnRequestChanges(feedback)
			}
			return true, nil
		default:
			previousHeight := p.editor.Height()
			var cmd tea.Cmd
			p.editor, cmd = p.editor.Update(msg)
			p.heightChanged = p.heightChanged || previousHeight != p.editor.Height()
			return false, cmd
		}
	}

	switch {
	case key.Matches(msg, p.keyClose):
		return false, func() tea.Msg { return CollapseInlineMsg{} }
	case key.Matches(msg, p.keyNo):
		return false, p.startEditing()
	case key.Matches(msg, p.keyLeftRight):
		p.requestChangesSelected = !p.requestChangesSelected
		return false, nil
	case key.Matches(msg, p.keyEnter):
		if p.requestChangesSelected {
			return false, p.startEditing()
		}
		return true, p.runConfirm()
	case key.Matches(msg, p.keyYes):
		return true, p.runConfirm()
	}
	return false, nil
}

func (p *PlanHandoffInline) startEditing() tea.Cmd {
	p.editing = true
	p.requestChangesSelected = true
	p.heightChanged = true
	p.clearSelection()
	if p.focused {
		return p.editor.Focus()
	}
	return nil
}

func (p *PlanHandoffInline) runConfirm() tea.Cmd {
	if p.OnConfirm != nil {
		cmd := p.OnConfirm()
		p.pendingCmd = cmd
		return cmd
	}
	return nil
}

// PendingCmd returns a cmd queued during mouse confirmation.
// The UI checks this via the CmdOnDone interface after a mouse-click dismissal.
func (p *PlanHandoffInline) PendingCmd() tea.Cmd { return p.pendingCmd }

// Height returns the number of lines needed by the current handoff state.
func (p *PlanHandoffInline) Height(width int) int {
	if !p.editing {
		return p.choiceLayout(width).height
	}
	return sectionHeight(planHandoffFeedbackPrompt, max(1, width)) +
		1 + p.editor.Height() + 1
}

func (p *PlanHandoffInline) choiceLayout(width int) planHandoffChoiceLayout {
	width = max(1, width)
	question := p.com.Styles.Editor.QuestionUnselected.Render(
		ansi.Wrap(planHandoffQuestion, width, ""),
	)

	hoveredBtn := common.HitButtonIndex(p.compositor, p.hoverX, p.hoverY)
	buttons := []common.ButtonOpts{
		{
			Text:           "Start coding",
			Selected:       !p.requestChangesSelected,
			Hovered:        hoveredBtn == 0,
			Padding:        3,
			UnderlineIndex: -1,
		},
		{
			Text:           "Revise plan",
			Selected:       p.requestChangesSelected,
			Hovered:        hoveredBtn == 1,
			Padding:        3,
			UnderlineIndex: -1,
		},
	}

	spacing := " "
	if lipgloss.Width(common.ButtonGroup(p.com.Styles, buttons, spacing)) > width {
		spacing = "\n"
	}
	buttonHeight := lipgloss.Height(common.ButtonGroup(p.com.Styles, buttons, spacing))
	return planHandoffChoiceLayout{
		question: question,
		buttons:  buttons,
		spacing:  spacing,
		height:   lipgloss.Height(question) + 1 + buttonHeight,
	}
}

// Draw renders the inline prompt at the given screen area.
func (p *PlanHandoffInline) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	if p.editing {
		return p.drawEditor(scr, area)
	}

	y := area.Min.Y
	layout := p.choiceLayout(area.Dx())
	y += drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), layout.question)
	y++ // blank

	p.compositor = common.ButtonHitCompositor(p.com.Styles, layout.buttons, layout.spacing, area.Min.X, y)
	buttons := common.ButtonGroup(p.com.Styles, layout.buttons, layout.spacing)
	drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), buttons)

	return nil
}

func (p *PlanHandoffInline) drawEditor(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	y := area.Min.Y
	questionText := p.com.Styles.Editor.QuestionUnselected.Render(
		ansi.Wrap(planHandoffFeedbackPrompt, max(1, area.Dx()), ""),
	)
	y += drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), questionText)
	y++

	promptPrefix := p.com.Styles.Editor.QuestionBody.Render("> ")
	prefixWidth := lipgloss.Width(promptPrefix)
	p.SetWidth(area.Dx())
	editorCursor := p.editor.Cursor()
	p.editorTextArea = image.Rect(
		area.Min.X+prefixWidth,
		y,
		min(area.Max.X, area.Min.X+prefixWidth+p.editor.Width()),
		min(area.Max.Y, y+p.editor.Height()),
	)
	editorView := p.editor.View()
	if start, end, ok := p.visualSelectionRange(); ok {
		editorView = list.Highlight(
			editorView,
			image.Rect(0, 0, p.editor.Width(), p.editor.Height()),
			start.row,
			start.col,
			end.row,
			end.col,
			list.ToHighlighter(p.com.Styles.TextSelection),
		)
	}
	var cursor *tea.Cursor
	for row, line := range strings.Split(editorView, "\n") {
		text := promptPrefix + line
		if row > 0 {
			text = strings.Repeat(" ", prefixWidth) + line
		}
		drawStyledText(scr, image.Rect(area.Min.X, y+row, area.Max.X, y+row+1), text)
		if editorCursor != nil && editorCursor.Y == row {
			current := *editorCursor
			current.X += prefixWidth
			current.Y += y - area.Min.Y
			cursor = &current
		}
	}
	return cursor
}

// SetWidth updates the feedback textarea width and records any resulting
// dynamic-height change for layout reconciliation.
func (p *PlanHandoffInline) SetWidth(width int) {
	promptPrefix := p.com.Styles.Editor.QuestionBody.Render("> ")
	previousHeight := p.editor.Height()
	previousWidth := p.editor.Width()
	p.editor.SetWidth(max(1, width-2-lipgloss.Width(promptPrefix)))
	if p.editor.Width() != previousWidth {
		p.clearSelection()
	}
	p.heightChanged = p.heightChanged || previousHeight != p.editor.Height()
}

// ShouldCollapse always uses the compact handoff while the chat has focus.
func (p *PlanHandoffInline) ShouldCollapse(_, _ int) bool { return true }

// CollapsedHeight returns the one-line compact handoff height.
func (p *PlanHandoffInline) CollapsedHeight() int { return 1 }

// CollapsedHelp returns the help description for restoring the handoff.
func (p *PlanHandoffInline) CollapsedHelp() string { return "review plan" }

// DrawCollapsed renders the persistent compact handoff while chat is focused.
func (p *PlanHandoffInline) DrawCollapsed(scr uv.Screen, area uv.Rectangle) {
	p.compositor = nil
	label := ansi.Truncate(planHandoffCollapsedPrompt, max(0, area.Dx()), "…")
	text := p.com.Styles.Messages.AssistantInfoModel.Render(label)
	drawStyledText(scr, area, text)
}

// HeightChanged reports whether the current state changed its layout height.
func (p *PlanHandoffInline) HeightChanged() bool {
	changed := p.heightChanged
	p.heightChanged = false
	return changed
}

// SetFocused updates the focus state (affects icon styling).
func (p *PlanHandoffInline) SetFocused(focused bool) {
	p.focused = focused
	if focused && p.editing {
		p.editor.Focus()
	} else {
		p.editor.Blur()
		p.clearSelection()
	}
}

// ShortHelp returns key bindings for the status bar.
func (p *PlanHandoffInline) ShortHelp() []key.Binding {
	if p.editing {
		return []key.Binding{p.keyEnter, p.keyNewline, p.keyClose}
	}
	return []key.Binding{p.keyLeftRight, p.keyEnter, p.keyYes, p.keyNo}
}

// SetHover implements MouseClickableEditor.
func (p *PlanHandoffInline) SetHover(x, y int) { p.hoverX = x; p.hoverY = y }

// HandleMouseClick implements MouseClickableEditor. Clicking "Start coding"
// stores the confirm cmd via PendingCmd; clicking "Revise plan" opens the
// feedback editor.
func (p *PlanHandoffInline) HandleMouseClick(x, y int) (bool, bool) {
	if p.editing {
		return false, false
	}
	switch common.HitButtonIndex(p.compositor, x, y) {
	case 0: // Start coding.
		p.requestChangesSelected = false
		p.runConfirm()
		return true, true
	case 1: // Revise plan.
		p.startEditing()
		return false, true
	}
	return false, false
}

// HandleMouseDown begins text selection inside the request-changes editor.
func (p *PlanHandoffInline) HandleMouseDown(x, y int) bool {
	point, editor, ok := p.selectionPointAt(x, y, false)
	if !ok {
		return false
	}
	p.editor = editor
	p.selectionAnchor = point
	p.selectionHead = point
	p.selectionSet = true
	p.selecting = true
	return true
}

// HandleMouseDrag updates an active request-changes text selection.
func (p *PlanHandoffInline) HandleMouseDrag(x, y int) bool {
	if !p.selecting {
		return false
	}
	point, editor, ok := p.selectionPointAt(x, y, true)
	if !ok {
		return false
	}
	p.editor = editor
	p.selectionHead = point
	return true
}

// HandleMouseRelease finishes selection and copies non-empty text.
func (p *PlanHandoffInline) HandleMouseRelease(x, y int) (bool, tea.Cmd) {
	if !p.selecting {
		return false, nil
	}
	p.HandleMouseDrag(x, y)
	p.selecting = false
	selected := p.SelectedText()
	if selected == "" {
		return true, nil
	}
	return true, common.CopyToClipboard(selected, "Selected text copied to clipboard")
}

// SelectedText returns the current request-changes selection.
func (p *PlanHandoffInline) SelectedText() string {
	if !p.selectionSet || p.selectionAnchor.offset == p.selectionHead.offset {
		return ""
	}
	start, end := p.selectionAnchor.offset, p.selectionHead.offset
	if start > end {
		start, end = end, start
	}
	value := []rune(p.editor.Value())
	start = min(max(0, start), len(value))
	end = min(max(0, end), len(value))
	return string(value[start:end])
}

func (p *PlanHandoffInline) selectionPointAt(x, y int, clampToArea bool) (planHandoffSelectionPoint, textarea.Model, bool) {
	if !p.editing || p.editorTextArea.Empty() {
		return planHandoffSelectionPoint{}, p.editor, false
	}
	if clampToArea {
		x = min(max(x, p.editorTextArea.Min.X), p.editorTextArea.Max.X)
		y = min(max(y, p.editorTextArea.Min.Y), p.editorTextArea.Max.Y-1)
	} else if !image.Pt(x, y).In(p.editorTextArea) {
		return planHandoffSelectionPoint{}, p.editor, false
	}

	editor := p.editor
	cursor := editor.Cursor()
	if cursor == nil {
		return planHandoffSelectionPoint{}, p.editor, false
	}
	targetRow := y - p.editorTextArea.Min.Y
	for cursor.Y < targetRow {
		previousLine, previousInfo := editor.Line(), editor.LineInfo()
		editor.CursorDown()
		cursor = editor.Cursor()
		if editor.Line() == previousLine && editor.LineInfo() == previousInfo {
			break
		}
	}
	for cursor.Y > targetRow {
		previousLine, previousInfo := editor.Line(), editor.LineInfo()
		editor.CursorUp()
		cursor = editor.Cursor()
		if editor.Line() == previousLine && editor.LineInfo() == previousInfo {
			break
		}
	}

	lines := strings.Split(editor.Value(), "\n")
	lineIndex := editor.Line()
	if lineIndex < 0 || lineIndex >= len(lines) {
		return planHandoffSelectionPoint{}, p.editor, false
	}
	line := []rune(lines[lineIndex])
	info := editor.LineInfo()
	segmentStart := min(info.StartColumn, len(line))
	segmentEnd := min(info.StartColumn+info.Width, len(line))
	targetCol := max(0, x-p.editorTextArea.Min.X)
	column := segmentStart + runeIndexAtCell(line[segmentStart:segmentEnd], targetCol)
	editor.SetCursorColumn(column)
	cursor = editor.Cursor()

	offset := column
	for i := 0; i < lineIndex; i++ {
		offset += utf8.RuneCountInString(lines[i]) + 1
	}
	return planHandoffSelectionPoint{
		offset: offset,
		row:    cursor.Y,
		col:    cursor.X,
	}, editor, true
}

func (p *PlanHandoffInline) visualSelectionRange() (planHandoffSelectionPoint, planHandoffSelectionPoint, bool) {
	if !p.selectionSet || p.selectionAnchor.offset == p.selectionHead.offset {
		return planHandoffSelectionPoint{}, planHandoffSelectionPoint{}, false
	}
	if p.selectionAnchor.offset < p.selectionHead.offset {
		return p.selectionAnchor, p.selectionHead, true
	}
	return p.selectionHead, p.selectionAnchor, true
}

func (p *PlanHandoffInline) clearSelection() {
	p.selectionAnchor = planHandoffSelectionPoint{}
	p.selectionHead = planHandoffSelectionPoint{}
	p.selectionSet = false
	p.selecting = false
}

func runeIndexAtCell(runes []rune, cell int) int {
	graphemes := uniseg.NewGraphemes(string(runes))
	runeIndex := 0
	width := 0
	for graphemes.Next() {
		grapheme := graphemes.Str()
		nextWidth := width + graphemes.Width()
		if cell < nextWidth {
			return runeIndex
		}
		width = nextWidth
		runeIndex += utf8.RuneCountInString(grapheme)
	}
	return runeIndex
}

// HandlePaste forwards paste events to the feedback editor.
func (p *PlanHandoffInline) HandlePaste(msg tea.PasteMsg) tea.Cmd {
	if !p.editing {
		return nil
	}
	p.clearSelection()
	previousHeight := p.editor.Height()
	var cmd tea.Cmd
	p.editor, cmd = p.editor.Update(msg)
	p.heightChanged = p.heightChanged || previousHeight != p.editor.Height()
	return cmd
}
