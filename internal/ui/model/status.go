package model

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// DefaultStatusTTL is the default time-to-live for status messages.
const DefaultStatusTTL = 5 * time.Second

// Status is the status bar and help model.
type Status struct {
	com           *common.Common
	hideHelp      bool
	help          help.Model
	helpKm        help.KeyMap
	msg           util.InfoMsg
	persistentMsg util.InfoMsg
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

// SetPersistentMsg sets a persistent status message that is always
// shown in the status bar (below transient messages). It can only be
// replaced or cleared — it does not auto-expire.
func (s *Status) SetPersistentMsg(msg util.InfoMsg) {
	s.persistentMsg = msg
}

// ClearPersistentMsg clears the persistent status message.
func (s *Status) ClearPersistentMsg() {
	s.persistentMsg = util.InfoMsg{}
}

// SetWidth sets the width of the status bar and help view.
func (s *Status) SetWidth(width int) {
	helpStyle := s.com.Styles.Status.Help
	horizontalPadding := helpStyle.GetPaddingLeft() + helpStyle.GetPaddingRight()
	s.help.SetWidth(width - horizontalPadding)
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
	if !s.hideHelp {
		helpView := s.com.Styles.Status.Help.Render(s.help.View(s.helpKm))
		uv.NewStyledString(helpView).Draw(scr, area)
	}

	// Render notifications
	if !s.msg.IsEmpty() {
		drawInfoMsg(scr, area, s.msg, s.com)
	} else if !s.persistentMsg.IsEmpty() {
		drawInfoMsg(scr, area, s.persistentMsg, s.com)
	}
}

func drawInfoMsg(scr uv.Screen, area uv.Rectangle, msg util.InfoMsg, com *common.Common) {
	var indStyle lipgloss.Style
	var msgStyle lipgloss.Style
	switch msg.Type {
	case util.InfoTypeError:
		indStyle = com.Styles.Status.ErrorIndicator
		msgStyle = com.Styles.Status.ErrorMessage
	case util.InfoTypeWarn:
		indStyle = com.Styles.Status.WarnIndicator
		msgStyle = com.Styles.Status.WarnMessage
	case util.InfoTypeUpdate:
		indStyle = com.Styles.Status.UpdateIndicator
		msgStyle = com.Styles.Status.UpdateMessage
	case util.InfoTypeInfo:
		indStyle = com.Styles.Status.InfoIndicator
		msgStyle = com.Styles.Status.InfoMessage
	case util.InfoTypeSuccess:
		indStyle = com.Styles.Status.SuccessIndicator
		msgStyle = com.Styles.Status.SuccessMessage
	}

	ind := indStyle.String()
	indWidth := lipgloss.Width(ind)
	msgPad := msgStyle.GetPaddingLeft() + msgStyle.GetPaddingRight()
	avail := max(0, area.Dx()-indWidth-msgPad)
	text := strings.Join(strings.Split(msg.Msg, "\n"), " ")
	text = ansi.Truncate(text, avail, "…")
	if w := lipgloss.Width(text); w < avail {
		text += strings.Repeat(" ", avail-w)
	}
	info := msgStyle.Render(text)

	// Draw the info message over the help view
	uv.NewStyledString(ind+info).Draw(scr, area)
}

// clearInfoMsgCmd returns a command that clears the info message after the
// given TTL.
func clearInfoMsgCmd(ttl time.Duration) tea.Cmd {
	return tea.Tick(ttl, func(time.Time) tea.Msg {
		return util.ClearStatusMsg{}
	})
}
