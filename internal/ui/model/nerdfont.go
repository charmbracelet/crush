package model

import (
	"log/slog"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/x/nerdfont"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// nerdFontsMsg reports whether the terminal should render Nerd Font glyphs.
type nerdFontsMsg bool

// detectNerdFonts resolves Nerd Font support off the render path, since
// probing may shell out to fc-list or scan font directories.
func detectNerdFonts(tui *config.TUIOptions) tea.Cmd {
	return func() tea.Msg {
		env, _ := os.LookupEnv(nerdfont.EnvVar)
		result := resolveNerdFonts(tui, env, nerdfont.Detect)
		slog.Debug("Nerd font support", "supported", result.Supported, "reason", result.Reason)
		return nerdFontsMsg(result.Supported)
	}
}

// resolveNerdFonts applies the precedence NERDFONT env var, then the
// tui.nerd_fonts option, then detection.
func resolveNerdFonts(tui *config.TUIOptions, env string, detect func() nerdfont.Result) nerdfont.Result {
	envSet := env != "" && !strings.EqualFold(strings.TrimSpace(env), "auto")
	if !envSet && tui != nil && tui.NerdFonts != nil {
		return nerdfont.Result{Supported: *tui.NerdFonts, Reason: "tui.nerd_fonts option"}
	}
	return detect()
}

// gitBranchLabel is the polled branch, prefixed with the branch glyph when
// Nerd Fonts are available.
func (m *UI) gitBranchLabel() string {
	if m.gitBranch == "" || !m.nerdFonts {
		return m.gitBranch
	}
	return styles.GitBranchIcon + " " + m.gitBranch
}
