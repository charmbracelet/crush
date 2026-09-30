package model

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/router"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// DefaultStatusTTL is the default time-to-live for status messages.
const DefaultStatusTTL = 5 * time.Second

// badgeLeftInset is the number of cells between the status bar's left edge
// and the mode badge.
const badgeLeftInset = 1

// Status is the status bar and help model.
type Status struct {
	com      *common.Common
	hideHelp bool
	help     help.Model
	helpKm   help.KeyMap
	msg      util.InfoMsg

	// inputMode and yolo drive the mode badge shown before the help hints.
	inputMode uiInputMode
	yolo      bool

	// routerDecision and hasRouterDecision drive the router badge shown
	// after the help hints: the reasoning effort applied to the last
	// dispatched message, flagged when the router's confidence was low.
	routerDecision    router.Decision
	hasRouterDecision bool
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

// SetMode sets the input mode and YOLO state used for the mode badge.
func (s *Status) SetMode(mode uiInputMode, yolo bool) {
	s.inputMode = mode
	s.yolo = yolo
}

// modeBadge renders the badge for the current mode, or an empty string in
// the default coding mode.
func (s *Status) modeBadge() string {
	t := s.com.Styles
	// Mirror the editor prompt precedence: planning wins over YOLO, which
	// can be carried into plan mode.
	if s.inputMode == uiInputModePlan {
		return t.Status.ModeBadgePlan.String()
	}
	if s.yolo {
		return t.Status.ModeBadgeYolo.String()
	}
	return ""
}

// SetRouterDecision sets the most recent model-router decision shown as a
// compact badge in the status bar. ok mirrors the router's own fail-open
// semantics: false means no trustworthy decision exists yet (router
// disabled, or every call so far failed open), and the badge is hidden.
func (s *Status) SetRouterDecision(d router.Decision, ok bool) {
	s.routerDecision = d
	s.hasRouterDecision = ok
}

// routerBadge renders the compact "router: <effort>" badge, styled with
// the warn token when the router's confidence was low, or an empty string
// when no decision has been made yet.
func (s *Status) routerBadge() string {
	if !s.hasRouterDecision {
		return ""
	}
	t := s.com.Styles
	label := "router: "
	if s.routerDecision.ModelID != "" {
		label += shortModelName(s.routerDecision.ModelID)
		if s.routerDecision.ReasoningEffort != "" {
			label += " · "
		}
	}
	label += s.routerDecision.ReasoningEffort
	if s.routerDecision.LowConfidence {
		// WarnIndicator has a baked-in SetString("WARNING") label (used
		// elsewhere as a standalone indicator icon); clear it so
		// Render doesn't prepend "WARNING " ahead of our own text.
		return t.Status.WarnIndicator.SetString("").Render(label)
	}
	// InfoIndicator similarly carries a baked-in "OKAY!" label.
	return t.Status.InfoIndicator.SetString("").Render(label)
}

// shortModelName returns the last path segment of an OpenRouter model
// id (e.g. "anthropic/claude-haiku-4" -> "claude-haiku-4"), so the badge
// stays compact. Ids with no "/" are returned unchanged.
func shortModelName(id string) string {
	if i := strings.LastIndex(id, "/"); i >= 0 {
		return id[i+1:]
	}
	return id
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
		helpStyle := s.com.Styles.Status.Help
		helpWidth := area.Dx() - helpStyle.GetPaddingLeft() - helpStyle.GetPaddingRight()
		badge := s.modeBadge()
		routerBadge := s.routerBadge()
		if badge != "" {
			// Shrink the hints so the badge does not push them past the
			// status area.
			helpWidth -= lipgloss.Width(badge) + 1 + badgeLeftInset
		}
		if routerBadge != "" {
			helpWidth -= lipgloss.Width(routerBadge) + 1
		}
		s.help.SetWidth(max(0, helpWidth))
		helpView := helpStyle.Render(s.help.View(s.helpKm))
		if routerBadge != "" {
			helpView += " " + routerBadge
		}
		if badge != "" {
			// Indent the rows after the first so the expanded help lines up
			// with the hints on the badge row.
			indent := strings.Repeat(" ", badgeLeftInset+lipgloss.Width(badge)+1)
			helpView = strings.ReplaceAll(helpView, "\n", "\n"+indent)
			helpView = strings.Repeat(" ", badgeLeftInset) + badge + " " + helpView
		}
		uv.NewStyledString(helpView).Draw(scr, area)
	}

	// Render notifications
	if s.msg.IsEmpty() {
		return
	}

	var indStyle lipgloss.Style
	var msgStyle lipgloss.Style
	// Mode banners show the same badge that sits next to the help hints, so
	// they honor the same left inset to keep the indicator from jumping when
	// the banner appears or expires.
	indInset := 0
	switch s.msg.Type {
	case util.InfoTypePlan:
		indStyle = s.com.Styles.Status.ModeBannerPlanBadge
		msgStyle = s.com.Styles.Status.ModeBannerPlan
		indInset = badgeLeftInset
	case util.InfoTypeYolo:
		indStyle = s.com.Styles.Status.ModeBannerYoloBadge
		msgStyle = s.com.Styles.Status.ModeBannerYolo
		indInset = badgeLeftInset
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
	avail := max(0, area.Dx()-indWidth-msgPad-indInset)
	msg := strings.Join(strings.Split(s.msg.Msg, "\n"), " ")
	msg = ansi.Truncate(msg, avail, "…")
	if w := lipgloss.Width(msg); w < avail {
		msg += strings.Repeat(" ", avail-w)
	}
	info := msgStyle.Render(msg)

	// Draw the info message over the help view
	uv.NewStyledString(strings.Repeat(" ", indInset)+ind+info).Draw(scr, area)
}

// clearInfoMsgCmd returns a command that clears the info message after the
// given TTL.
func clearInfoMsgCmd(ttl time.Duration) tea.Cmd {
	return tea.Tick(ttl, func(time.Time) tea.Msg {
		return util.ClearStatusMsg{}
	})
}
