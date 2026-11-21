package dialog

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	// OnboardingDemoID is the identifier for the final onboarding screen,
	// which offers a live first turn instead of a shortcut list.
	OnboardingDemoID = "onboarding_demo"

	// Wide enough for the longest sentence of copy plus its padding, so
	// the frame does not have to stretch to fit it.
	onboardingDemoDialogMaxWidth = 66
)

// OnboardingDemo closes out setup by offering to run the first turn live:
// reading the project and summarizing it is the tutorial.
type OnboardingDemo struct {
	com      *common.Common
	help     help.Model
	selected int

	keyMap struct {
		Go     key.Binding
		Toggle key.Binding
		Skip   key.Binding
	}
}

var _ Dialog = (*OnboardingDemo)(nil)

// NewOnboardingDemo creates the demo screen.
func NewOnboardingDemo(com *common.Common) *OnboardingDemo {
	m := &OnboardingDemo{com: com}

	m.help = help.New()
	m.help.Styles = com.Styles.DialogHelpStyles()

	m.keyMap.Go = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "go"),
	)
	m.keyMap.Toggle = key.NewBinding(
		key.WithKeys("tab", "shift+tab", "left", "right"),
		key.WithHelp("tab", "choose"),
	)
	m.keyMap.Skip = key.NewBinding(
		key.WithKeys("esc", "alt+esc"),
		key.WithHelp("esc", "skip"),
	)

	return m
}

// ID implements Dialog.
func (m *OnboardingDemo) ID() string {
	return OnboardingDemoID
}

// HandleMsg implements Dialog.
func (m *OnboardingDemo) HandleMsg(msg tea.Msg) Action {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	switch {
	case key.Matches(keyMsg, m.keyMap.Go):
		return ActionOnboardingDone{RunDemo: m.selected == 0}
	case key.Matches(keyMsg, m.keyMap.Skip):
		return ActionOnboardingDone{}
	case key.Matches(keyMsg, m.keyMap.Toggle):
		m.selected = 1 - m.selected
	}
	return nil
}

// ShortHelp returns the short help view.
func (m *OnboardingDemo) ShortHelp() []key.Binding {
	return []key.Binding{m.keyMap.Toggle, m.keyMap.Go, m.keyMap.Skip}
}

// FullHelp returns the full help view.
func (m *OnboardingDemo) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

// Draw implements Dialog.
func (m *OnboardingDemo) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(onboardingDemoDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	rc := NewRenderContext(t, width)
	rc.Title = "Welcome"
	rc.TitleInfo = onboardingProgress(t, OnboardingStepDemo)
	rc.AddPart("\n" + t.Dialog.Onboarding.Lead.Render("Let me show you what this looks like."))
	rc.AddPart(t.Dialog.SecondaryText.Render("I'll read through your project and summarize what it does."))
	rc.AddPart(t.Dialog.SecondaryText.Render("It's the first thing Crush runs for you."))

	buttons := common.ButtonGroup(t, []common.ButtonOpts{
		{Text: "Go ahead", Selected: m.selected == 0},
		{Text: "Not now", Selected: m.selected == 1},
	}, "  ")
	rc.AddPart("\n" + buttons)

	rc.Help = renderDialogHelp(t, &m.help, m, innerWidth)

	DrawCenter(scr, area, rc.Render())
	return nil
}
