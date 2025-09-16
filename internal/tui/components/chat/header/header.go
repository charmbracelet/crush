package header

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nom-nom-hub/blush/internal/config"
	"github.com/nom-nom-hub/blush/internal/fsext"
	"github.com/nom-nom-hub/blush/internal/lsp"
	"github.com/nom-nom-hub/blush/internal/lsp/protocol"
	"github.com/nom-nom-hub/blush/internal/pubsub"
	"github.com/nom-nom-hub/blush/internal/session"
	"github.com/nom-nom-hub/blush/internal/tui/styles"
	"github.com/nom-nom-hub/blush/internal/tui/util"
)

type Header interface {
	util.Model
	SetSession(session session.Session) tea.Cmd
	SetWidth(width int) tea.Cmd
	SetDetailsOpen(open bool)
	ShowingDetails() bool
}

type header struct {
	width       int
	session     session.Session
	lspClients  map[string]*lsp.Client
	detailsOpen bool
}

func New(lspClients map[string]*lsp.Client) Header {
	return &header{
		lspClients: lspClients,
		width:      0,
	}
}

func (h *header) Init() tea.Cmd {
	return nil
}

func (h *header) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case pubsub.Event[session.Session]:
		if msg.Type == pubsub.UpdatedEvent {
			if h.session.ID == msg.Payload.ID {
				h.session = msg.Payload
			}
		}
	}
	return h, nil
}

func (h *header) View() string {
	if h.session.ID == "" {
		return ""
	}

	const (
		gap          = "  " // Increased gap
		leftPadding  = 1
		rightPadding = 1
	)

	t := styles.CurrentTheme()

	var b strings.Builder

	// Create a larger, more prominent rainbow-colored "BLUSH" header logo
	// Using bigger, bolder letters with more spacing
	bb := t.S().Base.Foreground(t.BlueLight).Bold(true).Render("B")
	ll := t.S().Base.Foreground(t.Citron).Bold(true).Render("L")
	uu := t.S().Base.Foreground(t.Green).Bold(true).Render("U")
	ss := t.S().Base.Foreground(t.Yellow).Bold(true).Render("S")
	hh := t.S().Base.Foreground(t.RedLight).Bold(true).Render("H")
	
	// Add more spacing between letters for better visibility
	blushLogo := fmt.Sprintf("%s %s %s %s %s", bb, ll, uu, ss, hh)
	b.WriteString(blushLogo)
	b.WriteString(gap)

	availDetailWidth := h.width - leftPadding - rightPadding - lipgloss.Width(b.String())
	details := h.details(availDetailWidth)

	remainingWidth := h.width -
		lipgloss.Width(b.String()) -
		lipgloss.Width(details) -
		leftPadding -
		rightPadding

	// Use a more prominent separator
	if remainingWidth > 0 {
		b.WriteString(t.S().Base.Foreground(t.Border).Render(
			strings.Repeat("─", max(5, remainingWidth)),
		))
		b.WriteString(gap)
	}

	b.WriteString(details)

	return t.S().Base.Padding(0, rightPadding, 0, leftPadding).Render(b.String())
}

func (h *header) details(availWidth int) string {
	s := styles.CurrentTheme().S()

	var parts []string

	errorCount := 0
	for _, l := range h.lspClients {
		for _, diagnostics := range l.GetDiagnostics() {
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == protocol.SeverityError {
					errorCount++
				}
			}
		}
	}

	if errorCount > 0 {
		parts = append(parts, s.Error.Render(fmt.Sprintf("%s%d", styles.ErrorIcon, errorCount)))
	}

	agentCfg := config.Get().Agents["coder"]
	model := config.Get().GetModelByType(agentCfg.Model)
	percentage := (float64(h.session.CompletionTokens+h.session.PromptTokens) / float64(model.ContextWindow)) * 100
	formattedPercentage := s.Muted.Render(fmt.Sprintf("%d%%", int(percentage)))
	parts = append(parts, formattedPercentage)

	const keystroke = "ctrl+d"
	if h.detailsOpen {
		parts = append(parts, s.Muted.Render(keystroke)+s.Subtle.Render(" close"))
	} else {
		parts = append(parts, s.Muted.Render(keystroke)+s.Subtle.Render(" open "))
	}

	dot := s.Subtle.Render(" • ")
	metadata := strings.Join(parts, dot)
	metadata = dot + metadata

	// Truncate cwd if necessary, and insert it at the beginning.
	const dirTrimLimit = 4
	cwd := fsext.DirTrim(fsext.PrettyPath(config.Get().WorkingDir()), dirTrimLimit)
	cwd = ansi.Truncate(cwd, max(0, availWidth-lipgloss.Width(metadata)), "…")
	cwd = s.Muted.Render(cwd)

	return cwd + metadata
}

func (h *header) SetDetailsOpen(open bool) {
	h.detailsOpen = open
}

// SetSession implements Header.
func (h *header) SetSession(session session.Session) tea.Cmd {
	h.session = session
	return nil
}

// SetWidth implements Header.
func (h *header) SetWidth(width int) tea.Cmd {
	h.width = width
	return nil
}

// ShowingDetails implements Header.
func (h *header) ShowingDetails() bool {
	return h.detailsOpen
}
