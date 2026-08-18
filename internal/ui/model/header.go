package model

import (
	"cmp"
	"fmt"
	"net"
	"os"
	"os/user"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

const (
	headerDiag           = "╱"
	minHeaderDiags       = 3
	leftPadding          = 1
	rightPadding         = 1
	diagToDetailsSpacing = 1 // space between diagonal pattern and details section
)

// defaultWorkingDirFormat is used when options.tui.working_dir_format is
// unset. It mirrors the familiar user@host:cwd shell prompt so the header
// stays unambiguous when hopping between hosts.
const defaultWorkingDirFormat = "{user}@{host}:{cwd}"

// currentUserHost resolves the current username and hostname once per
// process. The hostname is shortened to its first label so long FQDNs do
// not crowd the header.
var currentUserHost = sync.OnceValue(func() userHost {
	host, err := os.Hostname()
	if err != nil {
		host = ""
	}
	name := ""
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	if name == "" {
		name = cmp.Or(os.Getenv("USER"), os.Getenv("USERNAME"), os.Getenv("LOGNAME"))
	}
	return userHost{name: shortUsername(name), host: shortHostname(host)}
})

type userHost struct {
	name string
	host string
}

// shortHostname reduces a hostname to its first DNS label so long FQDNs do
// not crowd the header. IP literals are left intact and an empty hostname
// falls back to "localhost".
func shortHostname(host string) string {
	if host == "" {
		return "localhost"
	}
	if net.ParseIP(host) != nil {
		return host
	}
	if label, _, ok := strings.Cut(host, "."); ok && label != "" {
		return label
	}
	return host
}

// shortUsername strips a Windows-style DOMAIN\ prefix from a username.
func shortUsername(name string) string {
	if _, short, ok := strings.Cut(name, `\`); ok && short != "" {
		return short
	}
	return name
}

// formatWorkingDir expands {cwd}, {user} and {host} placeholders in a
// working directory format string. Unknown placeholders are left verbatim.
// A blank format falls back to defaultWorkingDirFormat.
func formatWorkingDir(format, cwd, user, host string) string {
	if strings.TrimSpace(format) == "" {
		format = defaultWorkingDirFormat
	}
	// Sequential ReplaceAll avoids building a Replacer trie on every frame.
	out := strings.ReplaceAll(format, "{host}", host)
	out = strings.ReplaceAll(out, "{user}", user)
	return strings.ReplaceAll(out, "{cwd}", cwd)
}

type header struct {
	// cached logo and compact logo
	logo        string
	compactLogo string

	com     *common.Common
	width   int
	compact bool
}

// newHeader creates a new header model.
func newHeader(com *common.Common) *header {
	h := &header{
		com: com,
	}
	h.refresh()
	return h
}

// refresh rebuilds cached logo strings using the current styles. Call
// after the theme changes.
func (h *header) refresh() {
	t := h.com.Styles
	isHyper := h.com.IsHyper()
	charm := "Charm™"
	if !isHyper {
		charm = " " + charm
	}
	name := "CRUSH"
	if isHyper {
		name = "HYPERCRUSH"
	}
	h.compactLogo = t.Header.Charm.Render(charm) + " " +
		styles.ApplyBoldForegroundGrad(t.Header.LogoGradCanvas, name, t.Header.LogoGradFromColor, t.Header.LogoGradToColor) + " "
	// Force drawHeader to re-render the wide logo on the next frame.
	h.width = 0
	h.logo = ""
}

// drawHeader draws the header for the given session. lspErrorCount comes
// from the UI's memoized LSP state: drawing runs on every frame and must not
// probe the workspace (a synchronous HTTP round-trip in client/server mode).
func (h *header) drawHeader(
	scr uv.Screen,
	area uv.Rectangle,
	session *session.Session,
	compact bool,
	detailsOpen bool,
	width int,
	lspErrorCount int,
	hyperCredits *int,
) {
	t := h.com.Styles
	if width != h.width || compact != h.compact {
		h.logo = renderLogo(h.com.Styles, compact, h.com.IsHyper(), width)
	}

	h.width = width
	h.compact = compact

	if !compact || session == nil {
		uv.NewStyledString(h.logo).Draw(scr, area)
		return
	}

	if session.ID == "" {
		return
	}

	var b strings.Builder
	b.WriteString(h.compactLogo)

	availDetailWidth := width - leftPadding - rightPadding - lipgloss.Width(b.String()) - minHeaderDiags - diagToDetailsSpacing
	details := renderHeaderDetails(
		h.com,
		session,
		lspErrorCount,
		detailsOpen,
		availDetailWidth,
		hyperCredits,
	)

	remainingWidth := width -
		lipgloss.Width(b.String()) -
		lipgloss.Width(details) -
		leftPadding -
		rightPadding -
		diagToDetailsSpacing

	if remainingWidth > 0 {
		b.WriteString(t.Header.Diagonals.Render(
			strings.Repeat(headerDiag, max(minHeaderDiags, remainingWidth)),
		))
		b.WriteString(" ")
	}

	b.WriteString(details)

	view := uv.NewStyledString(
		t.Header.Wrapper.Padding(0, rightPadding, 0, leftPadding).Render(b.String()),
	)
	view.Draw(scr, area)
}

// renderHeaderDetails renders the details section of the header.
func renderHeaderDetails(
	com *common.Common,
	session *session.Session,
	lspErrorCount int,
	detailsOpen bool,
	availWidth int,
	hyperCredits *int,
) string {
	t := com.Styles

	var parts []string

	if lspErrorCount > 0 {
		parts = append(parts, t.LSP.ErrorDiagnostic.Render(fmt.Sprintf("%s%d", styles.LSPErrorIcon, lspErrorCount)))
	}

	agentCfg := com.Config().Agents[config.AgentCoder]
	model := com.Config().GetModelByType(agentCfg.Model)
	if model != nil && model.ContextWindow > 0 {
		percentage := (float64(session.CompletionTokens+session.PromptTokens) / float64(model.ContextWindow)) * 100
		percentageText := fmt.Sprintf("%d%%", int(percentage))
		if session.EstimatedUsage {
			percentageText = "~" + percentageText
		}
		formattedPercentage := t.Header.Percentage.Render(percentageText)
		parts = append(parts, formattedPercentage)
	}

	if com.IsHyper() && hyperCredits != nil {
		hc := t.Header.HypercreditIcon.Render(styles.HypercreditIcon) + " " + t.Header.Percentage.Render(common.FormatCredits(*hyperCredits))
		parts = append(parts, hc)
	}

	const keystroke = "ctrl+d"
	if detailsOpen {
		parts = append(parts, t.Header.Keystroke.Render(keystroke)+t.Header.KeystrokeTip.Render(" close"))
	} else {
		parts = append(parts, t.Header.Keystroke.Render(keystroke)+t.Header.KeystrokeTip.Render(" open "))
	}

	dot := t.Header.Separator.Render(" • ")
	metadata := strings.Join(parts, dot)
	metadata = dot + metadata

	const dirTrimLimit = 4
	cwd := fsext.DirTrim(fsext.PrettyPath(com.Workspace.WorkingDir()), dirTrimLimit)

	format := ""
	if cfg := com.Config().Options.TUI; cfg != nil {
		format = cfg.WorkingDirFormat
	}
	uh := currentUserHost()
	dir := formatWorkingDir(format, cwd, uh.name, uh.host)
	// Truncation drops from the right, which is where the metadata lives.
	// When the formatted directory would crowd out the metadata, fall back
	// to the bare path rather than losing the token usage and keystroke hint.
	if lipgloss.Width(dir+metadata) > availWidth && lipgloss.Width(cwd+metadata) <= availWidth {
		dir = cwd
	}
	result := t.Header.WorkingDir.Render(dir) + metadata
	return ansi.Truncate(result, max(0, availWidth), "…")
}
