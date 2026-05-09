package dialog

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// SSHAnimationsID is the identifier for the SSH animations dialog.
const SSHAnimationsID = "ssh_animations"

// SSHAAnimationAction represents the user's response to an SSH animation dialog.
type SSHAAnimationAction string

const (
	SSHAAnimationReduce       SSHAAnimationAction = "reduce"
	SSHAAnimationKeep         SSHAAnimationAction = "keep"
	SSHAAnimationPersistAuto  SSHAAnimationAction = "persist_auto"
	SSHAAnimationPersistNever SSHAAnimationAction = "persist_never"
)

// SSHAnimations represents a dialog for prompting about SSH animation preferences.
type SSHAnimations struct {
	com            *common.Common
	selectedOption int // 0: Yes, 1: No, 2: On all SSH, 3: Never
}

var _ Dialog = (*SSHAnimations)(nil)

// NewSSHAnimations creates a new SSH animations dialog.
func NewSSHAnimations(com *common.Common) *SSHAnimations {
	return &SSHAnimations{
		com:            com,
		selectedOption: 1, // Default to "No"
	}
}

// ID implements [Dialog].
func (*SSHAnimations) ID() string {
	return SSHAnimationsID
}

// HandleMsg implements [Dialog].
func (s *SSHAnimations) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, s.keyMap().Right), key.Matches(msg, s.keyMap().Tab):
			s.selectedOption = (s.selectedOption + 1) % 4
		case key.Matches(msg, s.keyMap().Left):
			s.selectedOption = (s.selectedOption + 3) % 4
		case key.Matches(msg, s.keyMap().Select):
			return s.selectCurrentOption()
		case key.Matches(msg, s.keyMap().Yes):
			return s.respond(SSHAAnimationReduce)
		case key.Matches(msg, s.keyMap().No):
			return s.respond(SSHAAnimationKeep)
		case key.Matches(msg, s.keyMap().PersistAuto):
			return s.respond(SSHAAnimationPersistAuto)
		case key.Matches(msg, s.keyMap().PersistNever):
			return s.respond(SSHAAnimationPersistNever)
		case key.Matches(msg, s.keyMap().Close):
			return ActionClose{}
		}
	}
	return nil
}

func (s *SSHAnimations) selectCurrentOption() Action {
	switch s.selectedOption {
	case 0:
		return s.respond(SSHAAnimationReduce)
	case 1:
		return s.respond(SSHAAnimationKeep)
	case 2:
		return s.respond(SSHAAnimationPersistAuto)
	default:
		return s.respond(SSHAAnimationPersistNever)
	}
}

func (s *SSHAnimations) respond(action SSHAAnimationAction) Action {
	switch action {
	case SSHAAnimationReduce:
		return ActionReduceSSHAAnimations{}
	case SSHAAnimationKeep:
		return ActionKeepSSHAAnimations{}
	case SSHAAnimationPersistAuto:
		return ActionPersistSSHAutoReduce{}
	case SSHAAnimationPersistNever:
		return ActionPersistSSHNever{}
	}
	return ActionClose{}
}

// Draw implements [Dialog].
func (s *SSHAnimations) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := s.com.Styles

	// Calculate dialog width.
	width := min(area.Dx()-4, defaultDialogMaxWidth)

	dialogStyle := t.Dialog.View.Width(width).Padding(0, 1)
	contentWidth := width - dialogStyle.GetHorizontalFrameSize() - 2

	// Render components to compute needed height.
	header := s.renderHeader(contentWidth)
	buttons := s.renderButtons(contentWidth)
	helpView := s.renderHelp(contentWidth)
	body := s.renderBody(contentWidth)

	parts := []string{header, "", body, "", buttons, "", helpView}
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	dialogHeight := lipgloss.Height(content) + dialogStyle.GetVerticalFrameSize()
	height := min(area.Dy()-4, dialogHeight)

	// Re-render with corrected height style (for vertical centering within the dialog frame).
	dialogStyle = t.Dialog.View.Width(width).Height(height).Padding(0, 1)
	DrawCenterCursor(scr, area, dialogStyle.Render(content), nil)
	return nil
}

func (s *SSHAnimations) renderHeader(contentWidth int) string {
	t := s.com.Styles
	title := common.DialogTitle(t, "Reduce animations over SSH?", contentWidth-t.Dialog.Title.GetHorizontalFrameSize(), t.Dialog.TitleGradFromColor, t.Dialog.TitleGradToColor)
	return t.Dialog.Title.Render(title)
}

func (s *SSHAnimations) renderBody(contentWidth int) string {
	return lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Center).
		Render("Crush detected that you are connected over SSH. \nAnimated spinners can look choppy or\nconsume extra bandwidth in some terminal sessions.\n\nWould you like to switch to a simpler animation mode?")
}

func (s *SSHAnimations) renderButtons(contentWidth int) string {
	buttons := []common.ButtonOpts{
		{Text: "Yes", UnderlineIndex: 0, Selected: s.selectedOption == 0},
		{Text: "No", UnderlineIndex: 0, Selected: s.selectedOption == 1},
		{Text: "For all SSH connections", UnderlineIndex: 2, Selected: s.selectedOption == 2},
		{Text: "Never", UnderlineIndex: 0, Selected: s.selectedOption == 3},
	}

	content := common.ButtonGroup(s.com.Styles, buttons, "  ")
	if lipgloss.Width(content) > contentWidth {
		content = common.ButtonGroup(s.com.Styles, buttons, "\n")
		return lipgloss.NewStyle().
			Width(contentWidth).
			Align(lipgloss.Center).
			Render(content)
	}
	return lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Right).
		Render(content)
}

func (s *SSHAnimations) renderHelp(contentWidth int) string {
	return "←/→ or tab to navigate · enter to confirm · esc to dismiss"
}

type sshAnimationsKeyMap struct {
	Left         key.Binding
	Right        key.Binding
	Tab          key.Binding
	Select       key.Binding
	Yes          key.Binding
	No           key.Binding
	PersistAuto  key.Binding
	PersistNever key.Binding
	Close        key.Binding
}

func (s *SSHAnimations) keyMap() sshAnimationsKeyMap {
	return sshAnimationsKeyMap{
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←", "previous"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→", "next"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next option"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", "ctrl+y"),
			key.WithHelp("enter", "confirm"),
		),
		Yes: key.NewBinding(
			key.WithKeys("y", "Y"),
			key.WithHelp("y", "yes"),
		),
		No: key.NewBinding(
			key.WithKeys("n", "N"),
			key.WithHelp("n", "no"),
		),
		PersistAuto: key.NewBinding(
			key.WithKeys("a", "A"),
			key.WithHelp("a", "all SSH"),
		),
		PersistNever: key.NewBinding(
			key.WithKeys("v", "V"),
			key.WithHelp("v", "never"),
		),
		Close: CloseKey,
	}
}

// ShortHelp implements [help.KeyMap].
func (s *SSHAnimations) ShortHelp() []key.Binding {
	km := s.keyMap()
	return []key.Binding{km.Left, km.Right, km.Select, km.Close}
}

// FullHelp implements [help.KeyMap].
func (s *SSHAnimations) FullHelp() [][]key.Binding {
	return [][]key.Binding{s.ShortHelp()}
}
