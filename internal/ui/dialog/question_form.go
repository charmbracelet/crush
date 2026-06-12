package dialog

import (
	"fmt"
	"image"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
)

// questionResponder extends InlineEditor with access to the last
// response. Used internally by QuestionForm to collect answers
// from child components.
type questionResponder interface {
	InlineEditor
	Response() question.Answer
	SetHover(x, y int)
	HandleMouseClick(x, y int) (done bool, handled bool)
}

// QuestionForm presents multiple questions as a tabbed form.
// Tab/shift+tab switches between questions; each question keeps
// its own internal keybindings. For multi-question batches, a
// Confirm tab is appended automatically.
type QuestionForm struct {
	Styles       *styles.Styles
	BatchID      string
	questions    []questionResponder // includes ConfirmComponent as last item for batches
	labels       []string            // includes "Confirm" for batches
	requestIDs   []string
	answers      []*question.Answer // nil until answered; only covers real questions
	activeIdx    int
	focused      bool
	hasConfirm   bool              // whether a confirm tab exists
	showTabs     bool              // whether to render tab chrome
	numQuestions int               // real question count (excludes confirm tab)
	confirmComp  *ConfirmComponent // nil when no confirm tab

	keyPrevTab key.Binding
	keyNextTab key.Binding
	keyClose   key.Binding

	// Compositor for tab hit detection. Built during Draw() from
	// tab layers positioned at their screen coordinates.
	compositor *lipgloss.Compositor

	// Hover position for highlighting interactive elements.
	hoverX, hoverY int

	// OnAnswer is called when the form is submitted. The UI sets
	// this to wire up workspace submission.
	OnAnswer func(responses []question.Answer)
}

// NewQuestionForm creates a tabbed multi-question form from a
// batch request. Each question is wrapped in its existing
// component type (YesNo, SingleChoice, MultiChoice, FreeText).
// A Confirm tab is appended for multi-question batches.
func NewQuestionForm(sty *styles.Styles, batch question.Request) *QuestionForm {
	comps := make([]questionResponder, len(batch.Questions))
	labels := make([]string, len(batch.Questions))
	ids := make([]string, len(batch.Questions))
	for i, req := range batch.Questions {
		switch req.Type {
		case question.TypeYesNo:
			comps[i] = NewYesNo(sty, req)
		case question.TypeSingleChoice:
			comps[i] = NewSingleChoice(sty, req)
		case question.TypeMultiChoice:
			comps[i] = NewMultiChoice(sty, req)
		case question.TypeFreeText:
			comps[i] = NewFreeText(sty, req)
		}
		if req.Label != "" {
			labels[i] = req.Label
		} else {
			labels[i] = shortLabel(req.Text)
		}
		ids[i] = req.ID
	}

	numQuestions := len(comps)
	// Confirm tab only for multi-question batches.
	hasConfirm := numQuestions > 1
	answers := make([]*question.Answer, numQuestions)

	var confirmComp *ConfirmComponent
	allLabels := labels
	if hasConfirm {
		confirmTitle := batch.ConfirmTitle
		if confirmTitle == "" {
			confirmTitle = "Confirm"
		}
		confirmComp = NewConfirmComponent(
			sty,
			confirmTitle,
			batch.ConfirmDescription,
			labels,
			batch.Questions,
			answers,
		)
		allLabels = make([]string, len(labels)+1)
		copy(allLabels, labels)
		allLabels[len(labels)] = "Confirm"
	}
	showTabs := numQuestions > 1

	f := &QuestionForm{
		Styles:       sty,
		BatchID:      batch.ID,
		questions:    comps,
		labels:       allLabels,
		requestIDs:   ids,
		answers:      answers,
		hasConfirm:   hasConfirm,
		showTabs:     showTabs,
		numQuestions: numQuestions,
		confirmComp:  confirmComp,
		keyPrevTab: key.NewBinding(
			key.WithKeys("ctrl+left"),
			key.WithHelp("ctrl+←", "prev tab"),
		),
		keyNextTab: key.NewBinding(
			key.WithKeys("ctrl+right"),
			key.WithHelp("ctrl+→", "next tab"),
		),
		keyClose: CloseKey,
	}

	// Wire confirm callbacks.
	if confirmComp != nil {
		confirmComp.OnConfirm = f.submit
		confirmComp.OnReject = func() {
			if idx := f.firstUnanswered(); idx >= 0 {
				f.switchTab(idx)
			} else if numQuestions > 0 {
				f.switchTab(numQuestions - 1)
			}
		}
	}

	if len(comps) > 0 {
		comps[0].SetFocused(true)
	}
	return f
}

// shortLabel truncates a question to at most three words for use
// as a tab header.
func shortLabel(q string) string {
	q = strings.ReplaceAll(q, "\n", " ")
	words := strings.Fields(q)
	if len(words) > 3 {
		words = words[:3]
	}
	return strings.Join(words, " ")
}

// isConfirmTab reports whether the active tab is the confirm tab.
func (f *QuestionForm) isConfirmTab() bool {
	return f.hasConfirm && f.activeIdx == f.numQuestions
}

// isAnswered reports whether a question has a meaningful answer.
func (f *QuestionForm) isAnswered(idx int) bool {
	if idx >= len(f.answers) || f.answers[idx] == nil {
		return false
	}
	resp := f.answers[idx]
	return len(resp.SelectedIDs) > 0 || resp.FillInText != "" || resp.Yes != nil
}

// firstUnanswered returns the index of the first unanswered
// question, or -1 if all are answered.
func (f *QuestionForm) firstUnanswered() int {
	for i, ans := range f.answers {
		if ans == nil {
			return i
		}
		if len(ans.SelectedIDs) == 0 && ans.FillInText == "" && ans.Yes == nil {
			return i
		}
	}
	return -1
}

// HandleKey routes keys to the active tab. Returns true when the
// entire batch is submitted.
func (f *QuestionForm) HandleKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	// Tab navigation works on all tabs including confirm.
	switch {
	case key.Matches(msg, f.keyNextTab):
		f.switchTab(f.activeIdx + 1)
		return false, nil
	case key.Matches(msg, f.keyPrevTab):
		f.switchTab(f.activeIdx - 1)
		return false, nil
	}

	// Confirm tab delegates to ConfirmComponent.
	if f.isConfirmTab() {
		done, cmd := f.confirmComp.HandleKey(msg)
		if done {
			return true, cmd
		}
		return false, cmd
	}

	// Global keys for question tabs.
	if key.Matches(msg, f.keyClose) {
		f.submit()
		return true, nil
	}

	// Route to active question.
	if f.activeIdx < f.numQuestions {
		done, cmd := f.questions[f.activeIdx].HandleKey(msg)
		if done {
			resp := f.questions[f.activeIdx].Response()
			f.answers[f.activeIdx] = &resp
			f.syncConfirmAnswers()
			if f.activeIdx < len(f.labels)-1 {
				f.switchTab(f.activeIdx + 1)
			} else if !f.hasConfirm {
				f.submit()
				return true, cmd
			}
			return false, cmd
		}
		return false, cmd
	}
	return false, nil
}

// switchTab moves focus to the given tab index, wrapping around.
// Snapshots the current question's response before leaving.
func (f *QuestionForm) switchTab(idx int) {
	totalTabs := len(f.labels)
	if totalTabs == 0 {
		return
	}
	// Snapshot and unfocus current.
	if !f.isConfirmTab() && f.activeIdx < f.numQuestions {
		resp := f.questions[f.activeIdx].Response()
		f.answers[f.activeIdx] = &resp
		f.questions[f.activeIdx].SetFocused(false)
	} else if f.isConfirmTab() {
		f.confirmComp.SetFocused(false)
	}
	// Wrap.
	if idx < 0 {
		idx = totalTabs - 1
	} else if idx >= totalTabs {
		idx = 0
	}
	f.activeIdx = idx
	// Focus new.
	if f.isConfirmTab() {
		f.syncConfirmAnswers()
		f.confirmComp.SetFocused(f.focused)
	} else if f.activeIdx < f.numQuestions {
		f.questions[f.activeIdx].SetFocused(f.focused)
	}
}

// syncConfirmAnswers pushes the latest answers to the confirm
// component so its summary stays current.
func (f *QuestionForm) syncConfirmAnswers() {
	if f.confirmComp != nil {
		f.confirmComp.UpdateAnswers(f.answers)
	}
}

// submit collects stored responses and calls OnAnswer.
func (f *QuestionForm) submit() {
	responses := make([]question.Answer, f.numQuestions)
	for i, ans := range f.answers {
		if ans != nil {
			responses[i] = *ans
		} else {
			responses[i] = question.Answer{
				QuestionID: f.requestIDs[i],
			}
		}
	}
	if f.OnAnswer != nil {
		f.OnAnswer(responses)
	}
}

// ShortHelp returns key bindings for the status bar.
func (f *QuestionForm) ShortHelp() []key.Binding {
	if f.isConfirmTab() {
		return f.confirmComp.ShortHelp()
	}
	bindings := []key.Binding{f.keyPrevTab, f.keyNextTab}
	if f.activeIdx < f.numQuestions {
		bindings = append(bindings, f.questions[f.activeIdx].ShortHelp()...)
	}
	return bindings
}

// Height returns the total height using the max tab height so
// switching tabs doesn't cause layout jumps.
func (f *QuestionForm) Height() int {
	h := 0
	if f.showTabs {
		h = 4 // bordered tab row (top + label + bottom) + blank line
	}
	maxQ := 0
	for _, q := range f.questions {
		if qh := q.Height(); qh > maxQ {
			maxQ = qh
		}
	}
	if f.confirmComp != nil {
		if ch := f.confirmComp.Height(); ch > maxQ {
			maxQ = ch
		}
	}
	h += maxQ
	return h
}

// CollapsedHeight returns the height of the collapsed summary
// line shown when the editor area is not focused.
func (f *QuestionForm) CollapsedHeight() int { return 1 }

// DrawCollapsed renders a compact one-line summary of the form
// when the user has tabbed away to the chat. For multi-question
// batches it shows the active question text and answered count;
// for single questions it shows just the question text.
func (f *QuestionForm) DrawCollapsed(scr uv.Screen, area uv.Rectangle) {
	icon := f.Styles.Editor.PromptQuestionIconBlurred.Render()
	iconWidth := lipgloss.Width(icon)
	textStyle := f.Styles.Messages.AssistantInfoModel
	countStyle := f.Styles.Messages.AssistantInfoProvider
	lineStyle := f.Styles.Section.Line

	var plainText string
	var confirmRendered string
	if f.numQuestions > 1 {
		answered := 0
		for i := 0; i < f.numQuestions; i++ {
			if f.isAnswered(i) {
				answered++
			}
		}
		if f.isConfirmTab() && f.confirmComp != nil {
			plainText = f.confirmComp.Title
			confirmRendered = f.Styles.Editor.QuestionConfirm.Render(f.confirmComp.Title)
		} else if f.activeIdx < len(f.questions) {
			plainText = f.getQuestionText(f.activeIdx)
		}
		count := fmt.Sprintf("(%d/%d answered)", answered, f.numQuestions)
		plainLabel := plainText + " " + count
		textWidth := iconWidth + 1 + lipgloss.Width(plainLabel)
		remaining := area.Dx() - textWidth - 1

		var rendered string
		if confirmRendered != "" {
			rendered = fmt.Sprintf("%s%s %s", icon, confirmRendered, countStyle.Render(count))
		} else {
			rendered = fmt.Sprintf("%s%s %s", icon, textStyle.Render(plainText), countStyle.Render(count))
		}
		if remaining > 0 {
			rendered = rendered + " " + lineStyle.Render(strings.Repeat(styles.SectionSeparator, remaining))
		}
		drawStyledText(scr, area, rendered)
	} else if f.numQuestions == 1 {
		plainText = f.getQuestionText(0)
		textWidth := iconWidth + 1 + lipgloss.Width(plainText)
		remaining := area.Dx() - textWidth - 1
		rendered := fmt.Sprintf("%s%s", icon, textStyle.Render(plainText))
		if remaining > 0 {
			rendered = rendered + " " + lineStyle.Render(strings.Repeat(styles.SectionSeparator, remaining))
		}
		drawStyledText(scr, area, rendered)
	}
}

// getQuestionText returns the question text for the given index.
func (f *QuestionForm) getQuestionText(idx int) string {
	type hasRequest interface {
		GetRequest() question.Question
	}
	if idx < len(f.questions) {
		if hr, ok := f.questions[idx].(hasRequest); ok {
			return hr.GetRequest().Text
		}
	}
	if idx < len(f.labels) {
		return f.labels[idx]
	}
	return ""
}

// Draw renders the tab bar and the active tab content. When
// showTabs is false (single question), renders content directly
// without tab chrome.
func (f *QuestionForm) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	contentY := area.Min.Y

	if f.showTabs {
		const tabPadX = 1
		tabHeight := 3

		// Compute display labels.
		labels := make([]string, len(f.labels))
		copy(labels, f.labels)

		// Truncate if tabs exceed width.
		totalWidth := 0
		for _, l := range labels {
			totalWidth += len(l) + tabPadX*2 + 2
		}
		if totalWidth > area.Dx() && len(labels) > 0 {
			availPerTab := max((area.Dx()/len(labels))-tabPadX*2-2, 1)
			for i, l := range labels {
				if len(l) > availPerTab {
					if availPerTab > 1 {
						labels[i] = l[:availPerTab-1] + "…"
					} else {
						labels[i] = l[:availPerTab]
					}
				}
			}
		}

		// Build tab layers for click hit detection.
		var layers []*lipgloss.Layer
		x := area.Min.X

		// Determine hovered tab via simple bounds check.
		hoveredTab := -1
		if f.hoverY >= area.Min.Y && f.hoverY < area.Min.Y+tabHeight {
			tx := area.Min.X
			for i, label := range labels {
				tw := len(label) + tabPadX*2 + 2
				if f.hoverX >= tx && f.hoverX < tx+tw {
					hoveredTab = i
					break
				}
				tx += tw
			}
		}

		for i, label := range labels {
			isActive := i == f.activeIdx
			isHovered := i == hoveredTab && !isActive
			labelWidth := len(label)
			tabWidth := labelWidth + tabPadX*2 + 2

			tabArea := image.Rect(x, area.Min.Y, x+tabWidth, area.Min.Y+tabHeight)

			border := f.Styles.Tab.InactiveBorder
			textStyle := f.Styles.Tab.InactiveStyle
			if !f.focused {
				border = f.Styles.Tab.InactiveBorderBlurred
			}
			if isActive {
				border = f.Styles.Tab.ActiveBorder
				textStyle = f.Styles.Tab.ActiveStyle
				if !f.focused {
					border = f.Styles.Tab.ActiveBorderBlurred
				}
			} else if i < f.numQuestions && f.isAnswered(i) {
				textStyle = f.Styles.Tab.ActiveStyle
			}
			if isHovered {
				hovered := textStyle
				hovered.Attrs |= uv.AttrBold
				textStyle = hovered
			}

			if i == 0 {
				if isActive {
					border.BottomLeft = uv.Side{Content: "┘", Style: border.BottomLeft.Style}
				} else {
					border.BottomLeft = uv.Side{Content: "┴", Style: border.BottomLeft.Style}
				}
			}

			border.Draw(scr, tabArea)

			innerWidth := tabWidth - 2
			xOff := (innerWidth - labelWidth) / 2
			innerArea := image.Rect(
				tabArea.Min.X+1+xOff, tabArea.Min.Y+1,
				tabArea.Max.X-1, tabArea.Max.Y-1,
			)
			uv.NewStyledString(textStyle.Styled(label)).Draw(scr, innerArea)

			// Create an invisible hit layer for this tab.
			hitStr := strings.Repeat(strings.Repeat(" ", tabWidth)+"\n", tabHeight-1) + strings.Repeat(" ", tabWidth)
			layers = append(layers, lipgloss.NewLayer(hitStr).X(x).Y(area.Min.Y).ID(fmt.Sprintf("tab_%d", i)))

			x += tabWidth
		}

		f.compositor = lipgloss.NewCompositor(layers...)

		lineY := area.Min.Y + tabHeight - 1
		lineSide := f.Styles.Tab.InactiveBorder.Bottom
		if !f.focused {
			lineSide = f.Styles.Tab.InactiveBorderBlurred.Bottom
		}
		for lx := x; lx < area.Max.X; lx++ {
			c := uv.NewCell(scr.WidthMethod(), lineSide.Content)
			if c != nil {
				c.Style = lineSide.Style
			}
			scr.SetCell(lx, lineY, c)
		}

		contentY = area.Min.Y + tabHeight + 1
	} else {
		f.compositor = nil
	}

	contentArea := image.Rect(area.Min.X, contentY, area.Max.X, area.Max.Y)

	if f.isConfirmTab() {
		return f.confirmComp.Draw(scr, contentArea)
	}
	if f.activeIdx < f.numQuestions {
		cur := f.questions[f.activeIdx].Draw(scr, contentArea)
		if cur != nil {
			cur.Y += contentY - area.Min.Y
		}
		return cur
	}
	return nil
}

// HeightChanged reports whether any component's height changed.
func (f *QuestionForm) HeightChanged() bool {
	for _, q := range f.questions {
		if q.HeightChanged() {
			return true
		}
	}
	if f.confirmComp != nil && f.confirmComp.HeightChanged() {
		return true
	}
	return false
}

// SetFocused updates focus state for the active tab.
func (f *QuestionForm) SetFocused(focused bool) {
	f.focused = focused
	if f.isConfirmTab() {
		f.confirmComp.SetFocused(focused)
	} else if f.activeIdx < f.numQuestions {
		f.questions[f.activeIdx].SetFocused(focused)
	}
}

// SetHover implements MouseClickableEditor. Stores the hover
// position and propagates it to the active component.
func (f *QuestionForm) SetHover(x, y int) {
	f.hoverX = x
	f.hoverY = y
	if f.isConfirmTab() && f.confirmComp != nil {
		f.confirmComp.SetHover(x, y)
	} else if f.activeIdx < len(f.questions) {
		f.questions[f.activeIdx].SetHover(x, y)
	}
}

// HandlePaste implements PasteableEditor. Forwards paste events
// to the active question component if it supports pasting.
func (f *QuestionForm) HandlePaste(msg tea.PasteMsg) tea.Cmd {
	if f.isConfirmTab() {
		return nil
	}
	if f.activeIdx < f.numQuestions {
		if p, ok := f.questions[f.activeIdx].(PasteableEditor); ok {
			return p.HandlePaste(msg)
		}
	}
	return nil
}

// HandleMouseClick implements MouseClickableEditor. It checks if
// the click landed on a tab and switches to it, or delegates to
// the active component for content-area clicks.
func (f *QuestionForm) HandleMouseClick(x, y int) (bool, bool) {
	// Check tabs first.
	if f.showTabs && f.compositor != nil {
		hit := f.compositor.Hit(x, y)
		if !hit.Empty() {
			var idx int
			if _, err := fmt.Sscanf(hit.ID(), "tab_%d", &idx); err == nil {
				if idx >= 0 && idx < len(f.labels) && idx != f.activeIdx {
					f.switchTab(idx)
				}
				return false, true
			}
		}
	}

	// Delegate to active component.
	if f.isConfirmTab() && f.confirmComp != nil {
		return f.confirmComp.HandleMouseClick(x, y)
	}
	if f.activeIdx < len(f.questions) {
		done, handled := f.questions[f.activeIdx].HandleMouseClick(x, y)
		if handled {
			resp := f.questions[f.activeIdx].Response()
			f.answers[f.activeIdx] = &resp
			f.syncConfirmAnswers()
			if done {
				if f.activeIdx < len(f.labels)-1 {
					f.switchTab(f.activeIdx + 1)
					return false, true
				} else if !f.hasConfirm {
					f.submit()
					return true, true
				}
			}
			return false, true
		}
	}
	return false, false
}
