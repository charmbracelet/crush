package header

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsp/protocol"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
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

func (p *header) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case pubsub.Event[session.Session]:
		if msg.Type == pubsub.UpdatedEvent {
			if p.session.ID == msg.Payload.ID {
				p.session = msg.Payload
			}
		}
	}
	return p, nil
}

func (p *header) View() string {
	if p.session.ID == "" {
		return ""
	}

	t := styles.CurrentTheme()
	details := p.details()
	parts := []string{
		t.S().Base.Foreground(t.Secondary).Render("Charm™"),
		" ",
		styles.ApplyBoldForegroundGrad("CRUSH", t.Secondary, t.Primary),
		" ",
	}

	remainingWidth := p.width - lipgloss.Width(strings.Join(parts, "")) - lipgloss.Width(details) - 2
	if remainingWidth > 0 {
		char := "╱"
		lines := strings.Repeat(char, remainingWidth)
		parts = append(parts, t.S().Base.Foreground(t.Primary).Render(lines), " ")
	}

	parts = append(parts, details)

	content := t.S().Base.Padding(0, 1).Render(
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			parts...,
		),
	)
	return content
}

func (h *header) details() string {
	t := styles.CurrentTheme()
	cwd := fsext.DirTrim(fsext.PrettyPath(config.Get().WorkingDir()), 4)
	parts := []string{
		t.S().Muted.Render(cwd),
	}

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
		parts = append(parts, t.S().Error.Render(fmt.Sprintf("%s%d", styles.ErrorIcon, errorCount)))
	}

	agentCfg := config.Get().Agents["coder"]
	model := config.Get().GetModelByType(agentCfg.Model)
	percentage := (float64(h.session.CompletionTokens+h.session.PromptTokens) / float64(model.ContextWindow)) * 100
	formattedPercentage := t.S().Muted.Render(fmt.Sprintf("%d%%", int(percentage)))
	parts = append(parts, formattedPercentage)

	if h.detailsOpen {
		parts = append(parts, t.S().Muted.Render("ctrl+d")+t.S().Subtle.Render(" close"))
	} else {
		parts = append(parts, t.S().Muted.Render("ctrl+d")+t.S().Subtle.Render(" open "))
	}
	dot := t.S().Subtle.Render(" • ")
	return strings.Join(parts, dot)
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
