package model

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// DefaultStatusTTL is the default time-to-live for status messages.
const DefaultStatusTTL = 5 * time.Second

// statusTickInterval is how often the timer in the status bar refreshes.
const statusTickInterval = 100 * time.Millisecond

// tokenSpeedInterval is how often the tokens-per-second value is
// recomputed. The tick still fires at statusTickInterval for the elapsed
// timer, but the speed display only changes this often so it stays
// readable.
const tokenSpeedInterval = 800 * time.Millisecond

// Status is the status bar and help model.
type Status struct {
	com      *common.Common
	hideHelp bool
	help     help.Model
	helpKm   help.KeyMap
	msg      util.InfoMsg

	// Timer fields — track how long the current request has been running.
	timerActive bool
	startTime   time.Time

	// Token-speed fields — estimated tokens/sec during streaming.
	estimatedTokens       int       // running estimate of tokens generated so far
	lastTickTokens        int       // token count at the previous speed sample
	lastTickTime          time.Time // when the last speed sample was taken
	tokensPerSec          float64   // displayed tokens/sec value
	nextSpeedTime         time.Time // earliest time to recompute tokensPerSec
	baselineOutputTokens  int64     // session.CompletionTokens at request start
	confirmedOutputTokens int64     // real output tokens from completed steps
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

// StartTimer begins tracking elapsed time for the current request. The
// baselineOutputTokens is the session's CompletionTokens at the start of
// the request — used to compute real token deltas when steps complete. It
// is safe to call when the timer is already running — the start time is
// only set once per run.
func (s *Status) StartTimer(baselineOutputTokens int64) {
	if s.timerActive {
		return
	}
	s.timerActive = true
	s.startTime = time.Now()
	s.lastTickTime = s.startTime
	s.lastTickTokens = 0
	s.estimatedTokens = 0
	s.tokensPerSec = 0
	s.nextSpeedTime = s.startTime.Add(tokenSpeedInterval)
	s.baselineOutputTokens = baselineOutputTokens
	s.confirmedOutputTokens = 0
}

// StopTimer stops the timer and resets token-speed tracking.
func (s *Status) StopTimer() {
	s.timerActive = false
	s.estimatedTokens = 0
	s.lastTickTokens = 0
	s.tokensPerSec = 0
	s.baselineOutputTokens = 0
	s.confirmedOutputTokens = 0
}

// IsTimerActive returns whether the request timer is currently running.
func (s *Status) IsTimerActive() bool {
	return s.timerActive
}

// UpdateConfirmedTokens updates the real output token count from the
// session's CompletionTokens. The delta above baseline replaces the
// heuristic estimate for completed steps, so the tokens-per-second value
// converges on the real count as steps finish.
func (s *Status) UpdateConfirmedTokens(completionTokens int64) {
	if !s.timerActive {
		return
	}
	delta := completionTokens - s.baselineOutputTokens
	if delta < 0 {
		delta = 0
	}
	s.confirmedOutputTokens = delta
}

// UpdateEstimatedTokens sets the estimated total tokens generated so far
// for the current in-flight step (from a text-length heuristic). This is
// called from the UI as streaming text arrives. The total displayed token
// count is the sum of confirmed tokens from completed steps plus this
// estimate for the current step.
func (s *Status) UpdateEstimatedTokens(tokens int) {
	s.estimatedTokens = int(s.confirmedOutputTokens) + tokens
}

// Tick advances the timer by one interval. The elapsed timer always
// updates, but tokens-per-second is only recomputed once per
// tokenSpeedInterval so the value stays readable.
func (s *Status) Tick() {
	if !s.timerActive {
		return
	}
	now := time.Now()
	if now.After(s.nextSpeedTime) {
		elapsed := now.Sub(s.lastTickTime).Seconds()
		if elapsed > 0 {
			tokenDelta := s.estimatedTokens - s.lastTickTokens
			s.tokensPerSec = float64(tokenDelta) / elapsed
		}
		s.lastTickTokens = s.estimatedTokens
		s.lastTickTime = now
		s.nextSpeedTime = now.Add(tokenSpeedInterval)
	}
}

// Elapsed returns the elapsed time since the timer started.
func (s *Status) Elapsed() time.Duration {
	if !s.timerActive {
		return 0
	}
	return time.Since(s.startTime)
}

// TokensPerSec returns the current estimated token generation speed.
func (s *Status) TokensPerSec() float64 {
	return s.tokensPerSec
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
	msg := strings.Join(strings.Split(s.msg.Msg, "\n"), " ")
	msg = ansi.Truncate(msg, avail, "…")
	if w := lipgloss.Width(msg); w < avail {
		msg += strings.Repeat(" ", avail-w)
	}
	info := msgStyle.Render(msg)

	// Draw the info message over the help view
	uv.NewStyledString(ind+info).Draw(scr, area)
}

// RenderTimerLine returns a styled string showing the elapsed time and
// token generation speed, suitable for display above the text editor.
// Returns an empty string when the timer is not active.
func (s *Status) RenderTimerLine() string {
	if !s.timerActive {
		return ""
	}
	elapsed := s.Elapsed()
	minutes := int(elapsed.Minutes())
	seconds := int(elapsed.Seconds()) % 60
	timerStr := fmt.Sprintf("%s %d:%02d", styles.SpinnerIcon, minutes, seconds)
	speedStr := fmt.Sprintf("%.1f tok/s", s.tokensPerSec)
	combined := fmt.Sprintf("%s  %s", timerStr, speedStr)
	return s.com.Styles.Status.Timer.Render(combined)
}

// clearInfoMsgCmd returns a command that clears the info message after the
// given TTL.
func clearInfoMsgCmd(ttl time.Duration) tea.Cmd {
	return tea.Tick(ttl, func(time.Time) tea.Msg {
		return util.ClearStatusMsg{}
	})
}

// statusTickCmd returns a command that fires a statusTickMsg after the
// configured tick interval.
func statusTickCmd() tea.Cmd {
	return tea.Tick(statusTickInterval, func(time.Time) tea.Msg {
		return statusTickMsg{}
	})
}
