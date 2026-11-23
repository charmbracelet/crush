package dialog

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// providerInlineMaxWidth caps the card so rows stay easy to scan.
const providerInlineMaxWidth = 72

// providerInlineIndent shifts the card right so it lines up with the text on
// the landing page.
const providerInlineIndent = 1

// ChoiceMoreProviders is the ID of the choice that opens the full model list.
const ChoiceMoreProviders = "more-providers"

// ProviderChoice is one row of the provider card.
type ProviderChoice struct {
	// ID is the provider ID, or [ChoiceMoreProviders].
	ID string
	// Name is the row's label.
	Name string
	// Detail is a short hint on how to authenticate.
	Detail string
	// NeedsKey marks a provider that only needs an API key. The card asks for
	// it in place instead of handing off to a sign-in dialog.
	NeedsKey bool
}

// providerInlineStep is what the card is currently asking.
type providerInlineStep uint8

const (
	// providerStepList asks which provider to use.
	providerStepList providerInlineStep = iota
	// providerStepKey asks for the chosen provider's API key.
	providerStepKey
)

// ProviderInline is the inline card shown in place of the editor when a
// message is sent and no provider is set up. Like the question form it takes
// over the editor area while it needs an answer, so there is never a text
// input competing with it. It lists a few providers and, for those that only
// need an API key, asks for the key without leaving the card.
type ProviderInline struct {
	com      *common.Common
	title    string
	choices  []ProviderChoice
	selected int
	step     providerInlineStep
	focused  bool

	// Key entry state.
	keyFor    ProviderChoice
	input     textinput.Model
	verifying bool
	rejected  bool

	// lastHeight backs HeightChanged.
	lastHeight int

	// OnSelect runs when a row that is not key-only is chosen. The UI starts
	// that provider's sign-in or opens the full model list.
	OnSelect func(ProviderChoice) tea.Cmd
	// OnSubmitKey runs when a key is submitted. The UI verifies and saves it
	// off the Update loop, then reports back through [ProviderInline.KeyResult].
	OnSubmitKey func(choice ProviderChoice, apiKey string) tea.Cmd
	// OnCancel runs when the user backs out of the card. It returns the
	// held message to the editor.
	OnCancel func() tea.Cmd

	keyUpDown key.Binding
	keyEnter  key.Binding
	keySave   key.Binding
	keyBack   key.Binding
	keyClose  key.Binding
}

var (
	_ InlineEditor    = (*ProviderInline)(nil)
	_ PasteableEditor = (*ProviderInline)(nil)
	_ StyleRefresher  = (*ProviderInline)(nil)
)

// NewProviderInline creates the card with the given title and rows. Wire its
// callbacks before making it the active inline editor.
func NewProviderInline(com *common.Common, title string, choices []ProviderChoice) *ProviderInline {
	input := textinput.New()
	input.SetVirtualCursor(false)
	input.EchoMode = textinput.EchoPassword
	input.Placeholder = "Paste your API key…"
	input.SetStyles(com.Styles.TextInput)

	c := &ProviderInline{
		com:     com,
		title:   title,
		choices: choices,
		input:   input,
		keyUpDown: key.NewBinding(
			key.WithKeys("up", "down", "ctrl+p", "ctrl+n", "k", "j"),
			key.WithHelp("↑↓", "choose"),
		),
		keyEnter: key.NewBinding(
			key.WithKeys("enter", "ctrl+y"),
			key.WithHelp("enter", "continue"),
		),
		keySave: key.NewBinding(
			key.WithKeys("enter", "ctrl+y"),
			key.WithHelp("enter", "save and send"),
		),
		keyBack: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "back"),
		),
		keyClose: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "keep editing"),
		),
	}
	c.lastHeight = c.height()
	return c
}

// Selected returns the highlighted row.
func (c *ProviderInline) Selected() (ProviderChoice, bool) {
	if c.selected < 0 || c.selected >= len(c.choices) {
		return ProviderChoice{}, false
	}
	return c.choices[c.selected], true
}

// AskingForKey reports whether the card is on its key-entry step.
func (c *ProviderInline) AskingForKey() bool { return c.step == providerStepKey }

// HandleKey implements [InlineEditor]. The card never reports itself done:
// the UI removes it once a provider is set up or the user backs out.
func (c *ProviderInline) HandleKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if c.step == providerStepKey {
		return c.handleKeyEntry(msg)
	}

	switch {
	case key.Matches(msg, c.keyClose):
		if c.OnCancel != nil {
			return true, c.OnCancel()
		}
		return true, nil
	case key.Matches(msg, c.keyEnter):
		choice, ok := c.Selected()
		if !ok {
			return false, nil
		}
		if choice.NeedsKey {
			c.beginKeyEntry(choice)
			return false, textinput.Blink
		}
		if c.OnSelect != nil {
			return false, c.OnSelect(choice)
		}
	case key.Matches(msg, c.keyUpDown):
		switch msg.String() {
		case "up", "ctrl+p", "k":
			c.move(-1)
		default:
			c.move(1)
		}
	}
	return false, nil
}

func (c *ProviderInline) handleKeyEntry(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if c.verifying {
		// Nothing to change while the key is being checked.
		return false, nil
	}
	switch {
	case key.Matches(msg, c.keyBack):
		c.step = providerStepList
		c.rejected = false
		c.input.SetValue("")
		c.input.Blur()
		return false, nil
	case key.Matches(msg, c.keySave):
		apiKey := strings.TrimSpace(c.input.Value())
		if apiKey == "" || c.OnSubmitKey == nil {
			return false, nil
		}
		c.verifying = true
		c.rejected = false
		c.input.Blur()
		return false, c.OnSubmitKey(c.keyFor, apiKey)
	}

	c.rejected = false
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return false, cmd
}

// HandlePaste implements [PasteableEditor].
func (c *ProviderInline) HandlePaste(msg tea.PasteMsg) tea.Cmd {
	if c.step != providerStepKey || c.verifying {
		return nil
	}
	c.rejected = false
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return cmd
}

// KeyResult reports the outcome of checking a submitted key. A rejected key
// puts the card back to asking so the user can try again; an accepted one
// leaves it as is, since the UI is about to replace it.
func (c *ProviderInline) KeyResult(accepted bool) {
	if accepted {
		return
	}
	c.verifying = false
	c.rejected = true
	c.input.SetValue("")
	c.input.Focus()
}

func (c *ProviderInline) beginKeyEntry(choice ProviderChoice) {
	c.step = providerStepKey
	c.keyFor = choice
	c.verifying = false
	c.rejected = false
	c.input.SetValue("")
	c.input.Focus()
}

func (c *ProviderInline) move(delta int) {
	if len(c.choices) == 0 {
		return
	}
	c.selected = (c.selected + delta + len(c.choices)) % len(c.choices)
}

// SetFocused implements [InlineEditor].
func (c *ProviderInline) SetFocused(focused bool) {
	c.focused = focused
	if c.step == providerStepKey && !c.verifying {
		if focused {
			c.input.Focus()
		} else {
			c.input.Blur()
		}
	}
}

// RefreshStyles implements [StyleRefresher].
func (c *ProviderInline) RefreshStyles() {
	c.input.SetStyles(c.com.Styles.TextInput)
}

// height is the number of lines the current step needs.
func (c *ProviderInline) height() int {
	if c.step == providerStepKey {
		// Question, blank line, input and a status line.
		return 4
	}
	// Question, blank line and one row per choice.
	return 2 + len(c.choices)
}

// Height implements [InlineEditor].
func (c *ProviderInline) Height(int) int { return c.height() }

// HeightChanged implements [InlineEditor].
func (c *ProviderInline) HeightChanged() bool {
	h := c.height()
	changed := h != c.lastHeight
	c.lastHeight = h
	return changed
}

// ShortHelp implements [InlineEditor].
func (c *ProviderInline) ShortHelp() []key.Binding {
	if c.step == providerStepKey {
		if c.verifying {
			return nil
		}
		return []key.Binding{c.keySave, c.keyBack}
	}
	return []key.Binding{c.keyUpDown, c.keyEnter, c.keyClose}
}

// Draw implements [InlineEditor].
func (c *ProviderInline) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	width := max(1, min(providerInlineMaxWidth, area.Dx()-providerInlineIndent))
	view := c.render(width)

	origin := uv.Rect(area.Min.X+providerInlineIndent, area.Min.Y, width, area.Dy())
	uv.NewStyledString(view).Draw(scr, origin)

	if c.step != providerStepKey || c.verifying || !c.focused {
		return nil
	}
	cur := c.input.Cursor()
	if cur == nil {
		return nil
	}
	// The UI wants the cursor relative to the editor area's top-left. The
	// input sits on the third row, after the gutter bar.
	cur.X += providerInlineIndent + lipgloss.Width(c.gutter(true))
	cur.Y = 2
	return cur
}

// The card borrows the question tool's styling: a heading, then rows with a
// bar in the gutter on the selected one.

// gutter returns the bar column in front of a row.
func (c *ProviderInline) gutter(active bool) string {
	if active {
		return c.com.Styles.Editor.QuestionCursorBar.Render("┃ ")
	}
	return "  "
}

// header returns the question that opens every step.
func (c *ProviderInline) header(question string, width int) string {
	text := ansi.Truncate(question, max(1, width), "…")
	return c.com.Styles.Editor.QuestionUnselected.Render(text)
}

// render builds the card for the given content width.
func (c *ProviderInline) render(width int) string {
	if c.step == providerStepKey {
		return c.renderKeyEntry(width)
	}

	nameWidth := 0
	for _, ch := range c.choices {
		nameWidth = max(nameWidth, lipgloss.Width(ch.Name))
	}

	rows := make([]string, 0, c.height())
	rows = append(rows, c.header(c.title, width), "")
	for i, ch := range c.choices {
		rows = append(rows, c.renderRow(ch, i == c.selected, nameWidth, width))
	}
	return strings.Join(rows, "\n")
}

func (c *ProviderInline) renderRow(ch ProviderChoice, selected bool, nameWidth, width int) string {
	t := c.com.Styles
	gutter := c.gutter(selected)

	nameStyle := t.Editor.QuestionUnselected
	if selected {
		nameStyle = t.Editor.QuestionSelected
	}
	name := nameStyle.Render(ch.Name) + strings.Repeat(" ", nameWidth-lipgloss.Width(ch.Name))

	detail := ""
	if ch.Detail != "" {
		avail := max(0, width-lipgloss.Width(gutter)-nameWidth-2)
		detail = t.Editor.QuestionBody.Render("  " + ansi.Truncate(ch.Detail, avail, "…"))
	}
	return gutter + name + detail
}

func (c *ProviderInline) renderKeyEntry(width int) string {
	t := c.com.Styles

	c.input.SetWidth(max(1, width-lipgloss.Width(c.gutter(true))-2))
	c.input.Prompt = "› "
	input := c.gutter(true) + c.input.View()

	var status string
	switch {
	case c.verifying:
		status = c.gutter(false) + t.Editor.QuestionBody.Render("Checking your key…")
	case c.rejected:
		status = c.gutter(false) + t.Dialog.TitleError.Render("That key didn't work. Paste it again?")
	}

	return strings.Join([]string{c.header(c.keyFor.Name+" API key", width), "", input, status}, "\n")
}
