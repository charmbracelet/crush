package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	// OnboardingHelpID is the identifier for the final onboarding screen
	// that teaches the keys worth knowing.
	OnboardingHelpID = "onboarding_help"

	// Wide enough for the longest sentence of copy plus its padding, so
	// the frame does not have to stretch to fit it.
	onboardingHelpDialogMaxWidth = 58
	// onboardingHelpKeyWidth caps the key column so a long binding (e.g.
	// "ctrl+shift+a") cannot push the descriptions out of the dialog.
	onboardingHelpKeyWidth = 14
)

// KeyHint is one key/description pair. The UI builds these from its real
// key bindings so the hints cannot drift from what the keys do.
type KeyHint struct {
	Key  string
	Desc string
}

// OnboardingHelp closes out first-run setup by showing how to use Crush.
type OnboardingHelp struct {
	com   *common.Common
	hints []KeyHint
	help  help.Model

	keyMap struct {
		Start key.Binding
	}
}

var _ Dialog = (*OnboardingHelp)(nil)

// NewOnboardingHelp creates the usage screen.
func NewOnboardingHelp(com *common.Common, hints []KeyHint) *OnboardingHelp {
	m := &OnboardingHelp{com: com, hints: hints}

	m.help = help.New()
	m.help.Styles = com.Styles.DialogHelpStyles()

	m.keyMap.Start = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "start"),
	)

	return m
}

// ID implements Dialog.
func (m *OnboardingHelp) ID() string {
	return OnboardingHelpID
}

// HandleMsg implements Dialog.
func (m *OnboardingHelp) HandleMsg(msg tea.Msg) Action {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	if key.Matches(keyMsg, m.keyMap.Start) {
		return ActionOnboardingDone{}
	}
	return nil
}

// ShortHelp returns the short help view.
func (m *OnboardingHelp) ShortHelp() []key.Binding {
	return []key.Binding{m.keyMap.Start}
}

// FullHelp returns the full help view.
func (m *OnboardingHelp) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

// Draw implements Dialog.
func (m *OnboardingHelp) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(onboardingHelpDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	rc := NewRenderContext(t, width)
	rc.Title = "Welcome"
	rc.TitleInfo = onboardingProgress(t, OnboardingStepHelp)
	rc.AddPart("\n" + t.Dialog.Onboarding.Lead.Render("You're set. Now just ask for things."))
	rc.AddPart(t.Dialog.SecondaryText.Render("Crush reads, writes, and runs things in this project."))
	rc.AddPart("\n" + m.hintsView())
	rc.AddPart(t.Dialog.Onboarding.Tip.PaddingLeft(1).Render(`Try: "what does this project do?"`))
	rc.Help = renderDialogHelp(t, &m.help, m, innerWidth)

	DrawCenter(scr, area, rc.Render())
	return nil
}

// hintsView renders the key column and its descriptions. It is shared with
// the landing cheat sheet.
func (m *OnboardingHelp) hintsView() string {
	return RenderKeyHints(m.com.Styles, m.hints, onboardingHelpKeyWidth)
}

// RenderKeyHints lays out key hints as a two-column block, padding the key
// column to a uniform width.
func RenderKeyHints(t *styles.Styles, hints []KeyHint, maxKeyWidth int) string {
	keyWidth := 0
	for _, hint := range hints {
		keyWidth = max(keyWidth, lipgloss.Width(hint.Key))
	}
	keyWidth = min(keyWidth, maxKeyWidth)

	lines := make([]string, 0, len(hints))
	for _, hint := range hints {
		keyCell := t.Dialog.Onboarding.Key.Width(keyWidth).Render(hint.Key)
		lines = append(lines, " "+keyCell+"  "+t.Dialog.Onboarding.Desc.Render(hint.Desc))
	}
	return strings.Join(lines, "\n")
}
