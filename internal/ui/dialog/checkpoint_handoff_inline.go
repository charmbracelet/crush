package dialog

import (
	"image"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

const (
	choiceKeep = iota
	choiceDiscard
	choiceRedo
	checkpointChoiceCount
)

// CheckpointHandoffInline asks the user what to do with a previewed
// checkpoint, in the same inline prompt shape the plan handoff uses.
//
// A preview used to announce itself with a banner that expired, and left the
// keep and discard choices in a palette the user had to think to reopen. That
// made a staged checkpoint indistinguishable from one that had been applied -
// the decision existed but nothing waited on it. Here the decision is the
// foreground until it is answered, and a third choice lets the user send the
// checkpoint back for another pass with their own notes attached.
type CheckpointHandoffInline struct {
	com            *common.Common
	selectedChoice int
	editing        bool
	focused        bool
	editor         textarea.Model

	heightChanged bool

	// OnKeep adopts the staged checkpoint.
	OnKeep func() tea.Cmd
	// OnDiscard throws it away and changes nothing.
	OnDiscard func() tea.Cmd
	// OnRedo receives the user's notes and is expected to discard the staged
	// checkpoint before generating another, so a session does not accumulate
	// previews the user never answered.
	OnRedo func(comments string) tea.Cmd

	keyLeftRight key.Binding
	keyEnter     key.Binding
	keyNewline   key.Binding
	keyKeep      key.Binding
	keyDiscard   key.Binding
	keyRedo      key.Binding
	keyClose     key.Binding
}

var (
	_ InlineEditor          = (*CheckpointHandoffInline)(nil)
	_ ResizableInlineEditor = (*CheckpointHandoffInline)(nil)
	_ PasteableEditor       = (*CheckpointHandoffInline)(nil)
)

const (
	checkpointHandoffQuestion       = "Keep this checkpoint?"
	checkpointHandoffNotesPrompt    = "What should the next one do differently?"
	checkpointHandoffEditorHint     = "Describe what to change..."
	checkpointHandoffNotesMaxLength = 2000

	// checkpointHandoffIndent aligns the choices under the question icon, as
	// the plan handoff does.
	checkpointHandoffIndent = 2
)

type checkpointChoiceLayout struct {
	question string
	buttons  []common.ButtonOpts
	spacing  string
	height   int
}

// NewCheckpointHandoffInline builds the prompt. Wire its callbacks before
// setting it as the active inline editor.
func NewCheckpointHandoffInline(com *common.Common) *CheckpointHandoffInline {
	editor := newQuestionTextarea(com.Styles, checkpointHandoffEditorHint, checkpointHandoffNotesMaxLength)
	editor.MinHeight = 3
	editor.MaxHeight = 8
	editor.SetHeight(3)

	return &CheckpointHandoffInline{
		com:            com,
		selectedChoice: choiceKeep,
		editor:         editor,
		keyLeftRight: key.NewBinding(
			key.WithKeys("left", "right", "h", "l"),
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
		keyKeep: key.NewBinding(
			key.WithKeys("k", "K"),
			key.WithHelp("k", "keep"),
		),
		keyDiscard: key.NewBinding(
			key.WithKeys("d", "D"),
			key.WithHelp("d", "discard"),
		),
		keyRedo: key.NewBinding(
			key.WithKeys("r", "R"),
			key.WithHelp("r", "redo with notes"),
		),
		keyClose: CloseKey,
	}
}

// HandleKey processes a key press, reporting done when the user has answered.
// Dismissing is not answering: esc collapses the prompt and leaves the staged
// checkpoint alone, so the palette can still offer keep and discard.
func (c *CheckpointHandoffInline) HandleKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if c.editing {
		switch {
		case key.Matches(msg, c.keyClose):
			c.editing = false
			c.editor.Blur()
			c.heightChanged = true
			return false, nil
		case key.Matches(msg, c.keyNewline):
			previousHeight := c.editor.Height()
			c.editor.InsertRune('\n')
			var cmd tea.Cmd
			c.editor, cmd = c.editor.Update(msg)
			c.heightChanged = c.heightChanged || previousHeight != c.editor.Height()
			return false, cmd
		case key.Matches(msg, c.keyEnter):
			comments := strings.TrimSpace(c.editor.Value())
			if comments == "" {
				// An empty redo is a guess. The choices stay on screen so the
				// user can pick keep or discard instead of waiting for a
				// checkpoint that would come back unchanged.
				return false, nil
			}
			if c.OnRedo != nil {
				return true, c.OnRedo(comments)
			}
			return true, nil
		default:
			previousHeight := c.editor.Height()
			var cmd tea.Cmd
			c.editor, cmd = c.editor.Update(msg)
			c.heightChanged = c.heightChanged || previousHeight != c.editor.Height()
			return false, cmd
		}
	}

	switch {
	case key.Matches(msg, c.keyClose):
		return false, func() tea.Msg { return CollapseInlineMsg{} }
	case key.Matches(msg, c.keyKeep):
		c.selectedChoice = choiceKeep
		return true, c.run(choiceKeep)
	case key.Matches(msg, c.keyDiscard):
		c.selectedChoice = choiceDiscard
		return true, c.run(choiceDiscard)
	case key.Matches(msg, c.keyRedo):
		return false, c.startEditing()
	case key.Matches(msg, c.keyLeftRight):
		delta := 1
		if msg.String() == "left" || msg.String() == "h" {
			delta = checkpointChoiceCount - 1
		}
		c.selectedChoice = (c.selectedChoice + delta) % checkpointChoiceCount
		return false, nil
	case key.Matches(msg, c.keyEnter):
		if c.selectedChoice == choiceRedo {
			return false, c.startEditing()
		}
		return true, c.run(c.selectedChoice)
	}
	return false, nil
}

func (c *CheckpointHandoffInline) startEditing() tea.Cmd {
	c.editing = true
	c.selectedChoice = choiceRedo
	c.heightChanged = true
	if c.focused {
		return c.editor.Focus()
	}
	return nil
}

func (c *CheckpointHandoffInline) run(choice int) tea.Cmd {
	switch choice {
	case choiceKeep:
		if c.OnKeep != nil {
			return c.OnKeep()
		}
	case choiceDiscard:
		if c.OnDiscard != nil {
			return c.OnDiscard()
		}
	}
	return nil
}

// Height returns the lines the prompt needs at the given content width.
func (c *CheckpointHandoffInline) Height(width int) int {
	if !c.editing {
		return c.choiceLayout(width).height
	}
	iconPrompt := questionIconPrompt(c.com.Styles, c.focused)
	return sectionHeight(checkpointHandoffNotesPrompt, max(1, width-checkpointHandoffIndent-lipgloss.Width(iconPrompt))) +
		1 + c.editor.Height() + 1
}

func (c *CheckpointHandoffInline) choiceLayout(width int) checkpointChoiceLayout {
	width = max(1, width)
	iconPrompt := questionIconPrompt(c.com.Styles, c.focused)
	iconWidth := lipgloss.Width(iconPrompt)
	question := iconPrompt + c.com.Styles.Editor.QuestionUnselected.Render(
		ansi.Wrap(checkpointHandoffQuestion, max(1, width-checkpointHandoffIndent-iconWidth), ""),
	)

	// Outside the editor pane the choices stay legible but inactive, matching
	// how the plan handoff renders when the chat holds focus.
	inactive := !c.focused
	selectedChoice := c.selectedChoice
	if !c.focused {
		selectedChoice = -1
	}

	buttons := []common.ButtonOpts{
		{
			Text:           "Keep",
			Selected:       selectedChoice == choiceKeep,
			Inactive:       inactive,
			Padding:        3,
			UnderlineIndex: 1,
		},
		{
			Text:           "Discard",
			Selected:       selectedChoice == choiceDiscard,
			Inactive:       inactive,
			Padding:        3,
			UnderlineIndex: 2,
		},
		{
			Text:           "Redo with notes",
			Selected:       selectedChoice == choiceRedo,
			Inactive:       inactive,
			Padding:        3,
			UnderlineIndex: 5,
		},
	}

	spacing := " "
	if lipgloss.Width(common.ButtonGroup(c.com.Styles, buttons, spacing)) > width-checkpointHandoffIndent {
		spacing = "\n"
	}
	buttonHeight := lipgloss.Height(common.ButtonGroup(c.com.Styles, buttons, spacing))
	return checkpointChoiceLayout{
		question: question,
		buttons:  buttons,
		spacing:  spacing,
		height:   lipgloss.Height(question) + 1 + buttonHeight + 2,
	}
}

// Draw renders the prompt within the given area.
func (c *CheckpointHandoffInline) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	if c.editing {
		return c.drawEditor(scr, area)
	}

	y := area.Min.Y
	layout := c.choiceLayout(area.Dx())
	y += drawStyledText(scr, image.Rect(area.Min.X+checkpointHandoffIndent, y, area.Max.X, area.Max.Y), layout.question)
	y++ // blank

	buttonsX := area.Min.X + checkpointHandoffIndent
	buttons := common.ButtonGroup(c.com.Styles, layout.buttons, layout.spacing)
	drawStyledText(scr, image.Rect(buttonsX, y, area.Max.X, area.Max.Y), buttons)

	return nil
}

func (c *CheckpointHandoffInline) drawEditor(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	y := area.Min.Y
	iconPrompt := questionIconPrompt(c.com.Styles, c.focused)
	iconWidth := lipgloss.Width(iconPrompt)
	blockX := area.Min.X + checkpointHandoffIndent
	questionText := c.com.Styles.Editor.QuestionUnselected.Render(
		ansi.Wrap(checkpointHandoffNotesPrompt, max(1, area.Dx()-checkpointHandoffIndent-iconWidth), ""),
	)
	y += drawStyledText(scr, image.Rect(blockX, y, area.Max.X, area.Max.Y), iconPrompt+questionText)
	y++

	promptPrefix := c.com.Styles.Editor.QuestionBody.Render("> ")
	prefixWidth := lipgloss.Width(promptPrefix)
	c.SetWidth(area.Dx())
	editorCursor := c.editor.Cursor()
	editorView := c.editor.View()

	var cursor *tea.Cursor
	for row, line := range strings.Split(editorView, "\n") {
		text := promptPrefix + line
		if row > 0 {
			text = strings.Repeat(" ", prefixWidth) + line
		}
		drawStyledText(scr, image.Rect(blockX, y+row, area.Max.X, y+row+1), text)
		if editorCursor != nil && editorCursor.Y == row {
			current := *editorCursor
			current.X += checkpointHandoffIndent + prefixWidth
			current.Y += y - area.Min.Y
			cursor = &current
		}
	}
	return cursor
}

// SetWidth resizes the notes textarea and records any height change.
func (c *CheckpointHandoffInline) SetWidth(width int) {
	promptPrefix := c.com.Styles.Editor.QuestionBody.Render("> ")
	previousHeight := c.editor.Height()
	c.editor.SetWidth(max(1, width-checkpointHandoffIndent-2-lipgloss.Width(promptPrefix)))
	c.heightChanged = c.heightChanged || previousHeight != c.editor.Height()
}

// HeightChanged reports and clears the layout-changed flag.
func (c *CheckpointHandoffInline) HeightChanged() bool {
	changed := c.heightChanged
	c.heightChanged = false
	return changed
}

// SetFocused tracks focus, which selects the icon style and whether the notes
// editor is live.
func (c *CheckpointHandoffInline) SetFocused(focused bool) {
	c.focused = focused
	if focused && c.editing {
		c.editor.Focus()
	} else {
		c.editor.Blur()
	}
}

// ShortHelp returns the status bar bindings for the current state.
func (c *CheckpointHandoffInline) ShortHelp() []key.Binding {
	if c.editing {
		return []key.Binding{c.keyEnter, c.keyNewline, c.keyClose}
	}
	return []key.Binding{c.keyLeftRight, c.keyEnter, c.keyKeep, c.keyDiscard, c.keyRedo}
}

// HandlePaste implements PasteableEditor, so a set of notes copied from the
// transcript or a file can be dropped straight into the redo prompt.
func (c *CheckpointHandoffInline) HandlePaste(msg tea.PasteMsg) tea.Cmd {
	if !c.editing {
		return nil
	}
	previousHeight := c.editor.Height()
	editor, cmd := c.editor.Update(msg)
	c.editor = editor
	c.heightChanged = c.heightChanged || previousHeight != c.editor.Height()
	return cmd
}
