package model

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// DefaultStatusTTL is the default time-to-live for status messages.
const DefaultStatusTTL = 5 * time.Second

// Status is the status bar and help model.
type Status struct {
	com      *common.Common
	hideHelp bool
	help     help.Model
	helpKm   help.KeyMap
	msg      util.InfoMsg
	width    int
}

// NewStatus creates a new status bar and help model.
func NewStatus(com *common.Common, km help.KeyMap) *Status {
	s := new(Status)
	s.com = com
	s.help = help.New()
	s.help.Styles = com.Styles.Help
	s.helpKm = km
	return s
}

// SetInfoMsg sets the status info message.
func (s *Status) SetInfoMsg(msg util.InfoMsg) {
	s.msg = msg
}

// ClearInfoMsg clears the status info message.
func (s *Status) ClearInfoMsg() {
	s.msg = util.InfoMsg{}
}

// SetWidth sets the width of the status bar and help view.
func (s *Status) SetWidth(width int) {
	s.width = width
}

func (s *Status) setHelpWidth(mode string) {
	helpStyle := s.com.Styles.Status.Help
	horizontalPadding := helpStyle.GetPaddingLeft() + helpStyle.GetPaddingRight()
	prefixWidth := lipgloss.Width(mode + " | ")
	s.help.SetWidth(max(0, s.width-horizontalPadding-prefixWidth))
}

// ShowingAll returns whether the full help view is shown.
func (s *Status) ShowingAll() bool {
	return s.help.ShowAll
}

// ToggleHelp toggles the full help view.
func (s *Status) ToggleHelp() {
	s.help.ShowAll = !s.help.ShowAll
}

// SetHideHelp sets whether the app is on the onboarding flow.
func (s *Status) SetHideHelp(hideHelp bool) {
	s.hideHelp = hideHelp
}

// Draw draws the status bar onto the screen.
func (s *Status) Draw(scr uv.Screen, area uv.Rectangle) {
	mode := statusModeLabel(s.com.Workspace.AgentMode())
	s.setHelpWidth(mode)
	if !s.hideHelp {
		helpView := s.com.Styles.Status.Help.Render(helpWithModePrefix(mode, s.help.View(s.helpKm)))
		uv.NewStyledString(helpView).Draw(scr, area)
	}

	// Render notifications
	if s.msg.IsEmpty() {
		return
	}

	var indStyle lipgloss.Style
	var msgStyle lipgloss.Style
	switch s.msg.Type {
	case util.InfoTypeError:
		indStyle = s.com.Styles.Status.ErrorIndicator
		msgStyle = s.com.Styles.Status.ErrorMessage
	case util.InfoTypeWarn:
		indStyle = s.com.Styles.Status.WarnIndicator
		msgStyle = s.com.Styles.Status.WarnMessage
	case util.InfoTypeUpdate:
		indStyle = s.com.Styles.Status.UpdateIndicator
		msgStyle = s.com.Styles.Status.UpdateMessage
	case util.InfoTypeInfo:
		indStyle = s.com.Styles.Status.InfoIndicator
		msgStyle = s.com.Styles.Status.InfoMessage
	case util.InfoTypeSuccess:
		indStyle = s.com.Styles.Status.SuccessIndicator
		msgStyle = s.com.Styles.Status.SuccessMessage
	}

	ind := indStyle.String()
	indWidth := lipgloss.Width(ind)
	msgPad := msgStyle.GetPaddingLeft() + msgStyle.GetPaddingRight()
	avail := max(0, area.Dx()-indWidth-msgPad)
	msg := mode + " | " + strings.Join(strings.Split(s.msg.Msg, "\n"), " ")
	msg = ansi.Truncate(msg, avail, "…")
	if w := lipgloss.Width(msg); w < avail {
		msg += strings.Repeat(" ", avail-w)
	}
	info := msgStyle.Render(msg)

	// Draw the info message over the help view
	uv.NewStyledString(ind+info).Draw(scr, area)
}

func helpWithModePrefix(mode, helpView string) string {
	prefix := mode + " | "
	lines := strings.Split(helpView, "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = strings.Repeat(" ", lipgloss.Width(prefix)) + lines[i]
	}
	return prefix + strings.Join(lines, "\n")
}

func statusModeLabel(agentID string) string {
	if agentID == config.AgentGoal {
		return "MODE: GOAL"
	}
	if agentID == config.AgentPlan || agentID == config.AgentReview {
		return "MODE: REVIEW READ ONLY"
	}
	return "MODE: TASK"
}

// clearInfoMsgCmd returns a command that clears the info message after the
// given TTL.
func clearInfoMsgCmd(ttl time.Duration) tea.Cmd {
	return tea.Tick(ttl, func(time.Time) tea.Msg {
		return util.ClearStatusMsg{}
	})
}
