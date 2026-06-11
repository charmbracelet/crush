package dialog

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/questions"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// questionOptionsList is a list of options for a question.
type questionOptionsList struct {
	*list.List

	t *styles.Styles
}

// newQuestionOptionsList creates a new list of options for a question.
func newQuestionOptionsList(sty *styles.Styles) *questionOptionsList {
	l := &questionOptionsList{
		List: list.NewList(),
		t:    sty,
	}
	l.RegisterRenderCallback(list.FocusedRenderCallback(l.List))
	return l
}

// SetQuestion sets the question's options in the list,
// and a map of which option is selected.
func (l *questionOptionsList) SetQuestion(q questions.Question, selOpts map[int]bool) {
	var items []list.Item
	for i, opt := range q.Options {
		items = append(items, &questionOptionsListItem{
			parent:   l,
			opt:      opt,
			selected: selOpts[i],
			index:    i,
		})
	}
	l.SetItems(items...)
}

// questionOptionsListItem is a list item for a question's option.
type questionOptionsListItem struct {
	parent   *questionOptionsList
	opt      questions.Option
	selected bool
	focused  bool
	index    int
}

func (i *questionOptionsListItem) Height() int {
	return 1
}

func (i *questionOptionsListItem) String() string {
	return i.opt.Label
}

// SetFocused implements ListItem.
func (i *questionOptionsListItem) SetFocused(focused bool) {
	i.focused = focused
}

func (i *questionOptionsListItem) Render(width int) string {
	t := i.parent.t

	// Setup styles
	radioStyle := t.RadioOff.Bold(true)
	if i.selected {
		radioStyle = t.RadioOn.Foreground(t.Green).Bold(true)
	}
	labelStyle := t.Dialog.NormalItem
	if i.focused {
		labelStyle = t.Dialog.SelectedItem
	}
	descStyle := labelStyle.Italic(true).Foreground(t.FgHalfMuted)
	gapStyle := labelStyle.Padding(0)

	// Render each part
	radioRender := radioStyle.Render()
	labelRender := labelStyle.Render(i.opt.Label)
	// NOTE: Only render the portion of description that fits on one line
	descRender := ""
	if len(i.opt.Description) > 0 {
		// NOTE: `-2` is for the padding of the style
		descAvailWidth := width - lipgloss.Width(radioRender) - lipgloss.Width(labelRender) - 2
		optDesc := ansi.Truncate(i.opt.Description, max(0, descAvailWidth), "…")
		descRender = descStyle.Render(optDesc)
	}
	gapRender := gapStyle.Render(strings.Repeat(" ", max(0, width-lipgloss.Width(radioRender)-lipgloss.Width(labelRender)-lipgloss.Width(descRender))))

	return labelStyle.Render(radioRender + labelRender + gapRender + descRender)
}
