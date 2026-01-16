package model

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

const (
	headerDiag     = "╱"
	minHeaderDiags = 3
	leftPadding    = 1
	rightPadding   = 1
)

// renderCompactHeader renders the compact header for the given session.
func renderCompactHeader(
	com *common.Common,
	session *session.Session,
	lspClients *csync.Map[string, *lsp.Client],
	detailsOpen bool,
	width int,
) string {
	if session == nil || session.ID == "" {
		return ""
	}

	t := com.Styles

	var b strings.Builder

	b.WriteString(t.Header.Charm.Render("Charm™"))
	b.WriteString(" ")
	b.WriteString(styles.ApplyBoldForegroundGrad(t, "CRUSH", t.Secondary, t.Primary))
	b.WriteString(" ")

	availDetailWidth := width - leftPadding - rightPadding - lipgloss.Width(b.String()) - minHeaderDiags
	details := renderHeaderDetails(com, session, lspClients, detailsOpen, availDetailWidth)

	remainingWidth := width -
		lipgloss.Width(b.String()) -
		lipgloss.Width(details) -
		leftPadding -
		rightPadding

	if remainingWidth > 0 {
		b.WriteString(t.Header.Diagonals.Render(
			strings.Repeat(headerDiag, max(minHeaderDiags, remainingWidth)),
		))
		b.WriteString(" ")
	}

	b.WriteString(details)

	return t.Base.Padding(0, rightPadding, 0, leftPadding).Render(b.String())
}

// renderHeaderDetails renders the details section of the header.
func renderHeaderDetails(
	com *common.Common,
	session *session.Session,
	lspClients *csync.Map[string, *lsp.Client],
	detailsOpen bool,
	availWidth int,
) string {
	t := com.Styles

	var parts []string

	errorCount := 0
	for l := range lspClients.Seq() {
		errorCount += l.GetDiagnosticCounts().Error
	}

	if errorCount > 0 {
		parts = append(parts, t.LSP.ErrorDiagnostic.Render(fmt.Sprintf("%s%d", styles.ErrorIcon, errorCount)))
	}

	agentCfg := config.Get().Agents[config.AgentCoder]
	model := config.Get().GetModelByType(agentCfg.Model)
	percentage := (float64(session.CompletionTokens+session.PromptTokens) / float64(model.ContextWindow)) * 100
	formattedPercentage := t.Header.Percentage.Render(fmt.Sprintf("%d%%", int(percentage)))
	parts = append(parts, formattedPercentage)

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
	cfg := com.Config()
	cwd := fsext.DirTrim(fsext.PrettyPath(cfg.WorkingDir()), dirTrimLimit)
	cwd = ansi.Truncate(cwd, max(0, availWidth-lipgloss.Width(metadata)), "…")
	cwd = t.Header.WorkingDir.Render(cwd)

	return cwd + metadata
}
