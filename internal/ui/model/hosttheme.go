package model

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
)

// hostThemeState tracks the theme a host application (an editor running
// Crush in one of its panes, for example) pushes through the server. A host
// theme is applied in memory only: it never touches the config.
//
// The host and the user take turns: the host's latest change applies, and
// any theme the user applies inside Crush afterwards wins until the host
// changes its theme again.
type hostThemeState struct {
	// last is the most recent host theme received, nil when there is none
	// or the host cleared it. The server replays the current theme every
	// time the event stream attaches, and the client re-reads it after a
	// reconnect, so a theme equal to last is a replay and is ignored;
	// otherwise a reconnect would undo a theme the user picked after the
	// host's.
	last *proto.HostTheme
	// seq counts the host themes received and the user's theme changes.
	// Styles built for an older seq are stale and dropped.
	seq uint64
	// active reports that the styles in use (or the ones a theme preview
	// reverts to) came from the host.
	active bool
	// transparent reports that m.isTransparent follows the host theme's
	// override instead of the config.
	transparent bool
}

// transparencyToggledMsg reports that the user's transparent background
// toggle reached the config.
type transparencyToggledMsg struct {
	transparent bool
}

// hostThemeStylesMsg carries styles built off the update loop for a host
// theme, or, when theme is nil, Crush's own theme to revert to.
type hostThemeStylesMsg struct {
	seq    uint64
	theme  *proto.HostTheme
	styles styles.Styles
	err    error
}

// handleHostThemeEvent reacts to the host setting or clearing its theme.
// Building styles may read theme files, so it happens in a command.
func (m *UI) handleHostThemeEvent(msg workspace.HostThemeEvent) tea.Cmd {
	if msg.Theme.Equal(m.hostTheme.last) {
		return nil
	}
	m.hostTheme.last = msg.Theme
	m.hostTheme.seq++

	if theme := msg.Theme; theme != nil {
		return m.buildHostThemeStyles(m.hostTheme.seq, theme)
	}
	if !m.hostTheme.active {
		// The user's own theme is on screen already; only a transparency
		// override can be left to undo.
		m.restoreConfigTransparency()
		return nil
	}
	return m.buildOwnThemeStyles(m.hostTheme.seq)
}

// buildHostThemeStyles builds the styles for a host theme.
func (m *UI) buildHostThemeStyles(seq uint64, theme *proto.HostTheme) tea.Cmd {
	return func() tea.Msg {
		s, err := styles.HostThemeStyles(theme.Base, theme.Palette, theme.Syntax)
		return hostThemeStylesMsg{seq: seq, theme: theme, styles: s, err: err}
	}
}

// buildOwnThemeStyles builds the theme Crush shows without a host theme:
// the configured one, else the one for the current provider.
func (m *UI) buildOwnThemeStyles(seq uint64) tea.Cmd {
	name := common.ThemeNameFromConfig(m.com.Config())
	key := m.themeKey
	return func() tea.Msg {
		s := styles.ThemeForKey(key)
		if name != "" {
			s = styles.ThemeFromConfig(name)
		}
		return hostThemeStylesMsg{seq: seq, styles: s}
	}
}

// handleHostThemeStyles applies styles built by [UI.handleHostThemeEvent]
// unless a newer host theme, or a theme the user applied, superseded them
// in the meantime.
func (m *UI) handleHostThemeStyles(msg hostThemeStylesMsg) tea.Cmd {
	if msg.seq != m.hostTheme.seq {
		return nil
	}
	if msg.err != nil {
		return util.ReportError(fmt.Errorf("apply host theme %q: %w", msg.theme.Name, msg.err))
	}

	if m.preThemeStyles != nil {
		// A theme preview is open: make the host's styles the ones it
		// reverts to, so esc lands on them rather than on stale ones.
		s := msg.styles
		m.preThemeStyles = &s
	} else {
		m.applyTheme(msg.styles)
	}
	m.hostTheme.active = msg.theme != nil

	if msg.theme != nil && msg.theme.Transparent != nil {
		m.isTransparent = *msg.theme.Transparent
		m.hostTheme.transparent = true
	} else {
		m.restoreConfigTransparency()
	}
	return nil
}

// userAppliedTheme records that the user applied a theme inside Crush. It
// wins over the host theme, including the host's transparency override,
// until the host changes its theme again; host styles still being built
// are dropped.
func (m *UI) userAppliedTheme() {
	m.hostTheme.active = false
	m.hostTheme.seq++
	m.restoreConfigTransparency()
}

// restoreConfigTransparency hands the background transparency back to the
// config when it follows a host override.
func (m *UI) restoreConfigTransparency() {
	if !m.hostTheme.transparent {
		return
	}
	m.hostTheme.transparent = false
	cfg := m.com.Config()
	m.isTransparent = cfg != nil && cfg.Options != nil && cfg.Options.TUI.IsTransparent()
}
