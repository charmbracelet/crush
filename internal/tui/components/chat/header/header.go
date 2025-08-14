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
		gap          = " "
		diag         = "╱"
		minDiags     = 1
		leftPadding  = 1
		rightPadding = 1
	)

	t := styles.CurrentTheme()

	var b strings.Builder

	b.WriteString(t.S().Base.Foreground(t.Secondary).Render("Charm™"))
	b.WriteString(gap)
	b.WriteString(styles.ApplyBoldForegroundGrad("CRUSH", t.Secondary, t.Primary))
	b.WriteString(gap)

	details := h.details()

	availWidth := h.width -
		lipgloss.Width(b.String()) -
		lipgloss.Width(details) -
		leftPadding -
		rightPadding

	if availWidth > 0 {
		b.WriteString(t.S().Base.Foreground(t.Primary).Render(
			strings.Repeat(diag, max(minDiags, availWidth)),
		))
		b.WriteString(gap)
	}

	b.WriteString(details)

	return t.S().Base.Padding(0, rightPadding, 0, leftPadding).Render(b.String())
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
