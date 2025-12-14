package askuser

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/askuser"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	// AskUserDialogID is the unique identifier for this dialog.
	AskUserDialogID dialogs.DialogID = "askuser"
)

// AskUserResponseMsg is sent when user completes or cancels the dialog.
type AskUserResponseMsg struct {
	Request  askuser.AskUserRequest
	Response askuser.AskUserResponse
}

// AskUserDialogCmp interface for ask user dialog component.
type AskUserDialogCmp interface {
	dialogs.DialogModel
}

// questionState tracks the state for each question.
type questionState struct {
	selectedIndices map[int]bool // For multi-select
	cursorIndex     int          // Current cursor position
	isOther         bool
	otherText       string
}

// askUserDialogCmp is the implementation of AskUserDialogCmp.
type askUserDialogCmp struct {
	wWidth, wHeight int
	width, height   int

	request         askuser.AskUserRequest
	questionStates  []questionState
	currentQuestion int

	otherInput     textinput.Model
	showOtherInput bool

	keyMap KeyMap
	help   help.Model
}

// NewAskUserDialogCmp creates a new ask user dialog component.
func NewAskUserDialogCmp(request askuser.AskUserRequest) AskUserDialogCmp {
	// Initialize question states
	states := make([]questionState, len(request.Questions))
	for i := range states {
		states[i] = questionState{
			selectedIndices: make(map[int]bool),
			cursorIndex:     0,
		}
	}

	// Setup other input
	otherInput := textinput.New()
	otherInput.Placeholder = "Enter your response..."
	otherInput.Prompt = "> "

	helpModel := help.New()

	return &askUserDialogCmp{
		request:        request,
		questionStates: states,
		currentQuestion: 0,
		otherInput:     otherInput,
		keyMap:         DefaultKeyMap(),
		help:           helpModel,
		width:          70,
	}
}

func (d *askUserDialogCmp) Init() tea.Cmd {
	return nil
}

func (d *askUserDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.wWidth = msg.Width
		d.wHeight = msg.Height
		d.width = min(80, int(float64(d.wWidth)*0.7))
		d.height = min(30, int(float64(d.wHeight)*0.6))
		d.otherInput.SetWidth(d.width - 10)
		return d, nil

	case tea.KeyPressMsg:
		if d.showOtherInput {
			return d.handleOtherInput(msg)
		}
		return d.handleQuestionInput(msg)
	}
	return d, nil
}

func (d *askUserDialogCmp) handleOtherInput(msg tea.KeyPressMsg) (util.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, d.keyMap.Cancel):
		d.showOtherInput = false
		d.otherInput.SetValue("")
		d.otherInput.Blur()
		return d, nil

	case key.Matches(msg, d.keyMap.Select):
		// Save other text and mark as other
		state := &d.questionStates[d.currentQuestion]
		state.isOther = true
		state.otherText = d.otherInput.Value()
		d.showOtherInput = false
		d.otherInput.SetValue("")
		d.otherInput.Blur()

		// Move to next question or submit
		if d.currentQuestion < len(d.request.Questions)-1 {
			d.currentQuestion++
		} else {
			return d, d.submit()
		}
		return d, nil

	default:
		var cmd tea.Cmd
		d.otherInput, cmd = d.otherInput.Update(msg)
		return d, cmd
	}
}

func (d *askUserDialogCmp) handleQuestionInput(msg tea.KeyPressMsg) (util.Model, tea.Cmd) {
	question := d.request.Questions[d.currentQuestion]
	state := &d.questionStates[d.currentQuestion]
	optionCount := len(question.Options)

	switch {
	case key.Matches(msg, d.keyMap.Cancel):
		return d, d.cancel()

	case key.Matches(msg, d.keyMap.Up):
		if state.cursorIndex > 0 {
			state.cursorIndex--
		}
		state.isOther = false
		return d, nil

	case key.Matches(msg, d.keyMap.Down):
		if state.cursorIndex < optionCount-1 {
			state.cursorIndex++
		}
		state.isOther = false
		return d, nil

	case key.Matches(msg, d.keyMap.Toggle):
		if question.MultiSelect {
			idx := state.cursorIndex
			state.selectedIndices[idx] = !state.selectedIndices[idx]
		}
		return d, nil

	case key.Matches(msg, d.keyMap.Other):
		d.showOtherInput = true
		d.otherInput.Focus()
		return d, d.otherInput.Focus()

	case key.Matches(msg, d.keyMap.Left):
		if d.currentQuestion > 0 {
			d.currentQuestion--
		}
		return d, nil

	case key.Matches(msg, d.keyMap.Right):
		if d.currentQuestion < len(d.request.Questions)-1 {
			d.currentQuestion++
		}
		return d, nil

	case key.Matches(msg, d.keyMap.Select):
		// For single-select, mark current as selected
		if !question.MultiSelect {
			state.selectedIndices = map[int]bool{state.cursorIndex: true}
		}

		// Move to next question or submit
		if d.currentQuestion < len(d.request.Questions)-1 {
			d.currentQuestion++
		} else {
			return d, d.submit()
		}
		return d, nil
	}

	return d, nil
}

func (d *askUserDialogCmp) submit() tea.Cmd {
	answers := make([]askuser.Answer, len(d.request.Questions))

	for i, state := range d.questionStates {
		answer := askuser.Answer{
			QuestionIndex: i,
			IsOther:       state.isOther,
			OtherText:     state.otherText,
		}

		if !state.isOther {
			for idx, selected := range state.selectedIndices {
				if selected {
					answer.SelectedIndices = append(answer.SelectedIndices, idx)
				}
			}
			if len(answer.SelectedIndices) == 0 {
				// Default to current selection for single-select
				answer.SelectedIndices = []int{state.cursorIndex}
			}
			if len(answer.SelectedIndices) > 0 {
				answer.SelectedIndex = answer.SelectedIndices[0]
			}
		}

		answers[i] = answer
	}

	return tea.Batch(
		util.CmdHandler(dialogs.CloseDialogMsg{}),
		util.CmdHandler(AskUserResponseMsg{
			Request: d.request,
			Response: askuser.AskUserResponse{
				RequestID: d.request.ID,
				Answers:   answers,
				Cancelled: false,
			},
		}),
	)
}

func (d *askUserDialogCmp) cancel() tea.Cmd {
	return tea.Batch(
		util.CmdHandler(dialogs.CloseDialogMsg{}),
		util.CmdHandler(AskUserResponseMsg{
			Request: d.request,
			Response: askuser.AskUserResponse{
				RequestID: d.request.ID,
				Cancelled: true,
			},
		}),
	)
}

func (d *askUserDialogCmp) View() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base

	// Title with question progress
	progress := fmt.Sprintf("(%d/%d)", d.currentQuestion+1, len(d.request.Questions))
	title := core.Title("Question "+progress, d.width-4)

	question := d.request.Questions[d.currentQuestion]
	state := d.questionStates[d.currentQuestion]

	// Header
	header := t.S().Text.Bold(true).Render(question.Header)

	// Question text
	questionText := t.S().Text.Width(d.width - 8).Render(question.Question)

	// Options or other input
	var optionsView string
	if d.showOtherInput {
		optionsView = d.renderOtherInput()
	} else {
		optionsView = d.renderOptions(question, state)
	}

	// Navigation dots for multiple questions
	var navDots string
	if len(d.request.Questions) > 1 {
		var dots []string
		for i := range d.request.Questions {
			if i == d.currentQuestion {
				dots = append(dots, "●")
			} else {
				dots = append(dots, "○")
			}
		}
		navDots = t.S().Muted.Render(strings.Join(dots, " "))
	}

	// Help
	helpText := d.help.View(d.keyMap)

	// Build content
	var contentParts []string
	contentParts = append(contentParts, title)
	contentParts = append(contentParts, "")
	contentParts = append(contentParts, header)
	contentParts = append(contentParts, questionText)
	contentParts = append(contentParts, "")
	contentParts = append(contentParts, optionsView)

	if navDots != "" {
		contentParts = append(contentParts, "")
		contentParts = append(contentParts, navDots)
	}

	contentParts = append(contentParts, "")
	contentParts = append(contentParts, helpText)

	content := lipgloss.JoinVertical(lipgloss.Left, contentParts...)

	return baseStyle.
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(d.width).
		Render(content)
}

func (d *askUserDialogCmp) renderOptions(question askuser.Question, state questionState) string {
	t := styles.CurrentTheme()
	var lines []string

	for i, opt := range question.Options {
		var prefix string
		isSelected := state.selectedIndices[i]
		isCurrent := i == state.cursorIndex

		if question.MultiSelect {
			if isSelected {
				prefix = "[x] "
			} else {
				prefix = "[ ] "
			}
		} else {
			if isCurrent {
				prefix = "● "
			} else {
				prefix = "○ "
			}
		}

		labelStyle := t.S().Text
		if isCurrent && !d.showOtherInput {
			labelStyle = labelStyle.Bold(true).Foreground(t.Primary)
		}

		line := labelStyle.Render(prefix + opt.Label)
		if opt.Description != "" {
			desc := t.S().Muted.Render(" - " + opt.Description)
			line += desc
		}
		lines = append(lines, line)
	}

	// Add "Other" option
	otherStyle := t.S().Muted
	if state.isOther {
		otherStyle = t.S().Text.Bold(true).Foreground(t.Primary)
	}
	otherPrefix := "○ "
	if state.isOther {
		otherPrefix = "● "
	}
	lines = append(lines, otherStyle.Render(otherPrefix+"Other (press 'o')"))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (d *askUserDialogCmp) renderOtherInput() string {
	t := styles.CurrentTheme()
	label := t.S().Text.Bold(true).Render("Enter your response:")
	input := d.otherInput.View()
	hint := t.S().Muted.Render("Press Enter to confirm, Esc to cancel")
	return lipgloss.JoinVertical(lipgloss.Left, label, "", input, "", hint)
}

// ID implements dialogs.DialogModel.
func (d *askUserDialogCmp) ID() dialogs.DialogID {
	return AskUserDialogID
}

// Position implements dialogs.DialogModel.
func (d *askUserDialogCmp) Position() (int, int) {
	row := d.wHeight/2 - d.height/2
	col := d.wWidth/2 - d.width/2

	// Ensure position is not negative
	if row < 0 {
		row = 0
	}
	if col < 0 {
		col = 0
	}

	return row, col
}
