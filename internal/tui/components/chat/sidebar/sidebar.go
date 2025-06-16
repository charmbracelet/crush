package sidebar

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/llm/models"
	"github.com/charmbracelet/crush/internal/logging"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsp/protocol"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/components/logo"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/charmbracelet/lipgloss/v2"
)

const (
	logoBreakpoint = 65
)

type Sidebar interface {
	util.Model
	layout.Sizeable
}

type sidebarCmp struct {
	width, height int
	session       session.Session
	logo          string
	cwd           string
	lspClients    map[string]*lsp.Client
}

func NewSidebarCmp(lspClients map[string]*lsp.Client) Sidebar {
	return &sidebarCmp{
		lspClients: lspClients,
	}
}

func (m *sidebarCmp) Init() tea.Cmd {
	m.logo = m.logoBlock(false)
	m.cwd = cwd()
	return nil
}

func (m *sidebarCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case chat.SessionSelectedMsg:
		if msg.ID != m.session.ID {
			m.session = msg
		}
	case chat.SessionClearedMsg:
		m.session = session.Session{}
	case pubsub.Event[session.Session]:
		if msg.Type == pubsub.UpdatedEvent {
			if m.session.ID == msg.Payload.ID {
				m.session = msg.Payload
			}
		}
	}
	return m, nil
}

func (m *sidebarCmp) View() tea.View {
	t := styles.CurrentTheme()
	parts := []string{
		m.logo,
	}

	if m.session.ID != "" {
		parts = append(parts, t.S().Muted.Render(m.session.Title), "")
	}

	parts = append(parts,
		m.cwd,
		"",
		m.currentModelBlock(),
		"",
		m.lspBlock(),
		"",
		m.mcpBlock(),
	)

	return tea.NewView(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)
}

func (m *sidebarCmp) SetSize(width, height int) tea.Cmd {
	if width < logoBreakpoint && m.width >= logoBreakpoint {
		m.logo = m.logoBlock(true)
	} else if width >= logoBreakpoint && m.width < logoBreakpoint {
		m.logo = m.logoBlock(false)
	}

	m.width = width
	m.height = height
	return nil
}

func (m *sidebarCmp) GetSize() (int, int) {
	return m.width, m.height
}

func (m *sidebarCmp) logoBlock(compact bool) string {
	t := styles.CurrentTheme()
	return logo.Render(version.Version, compact, logo.Opts{
		FieldColor:   t.Primary,
		TitleColorA:  t.Secondary,
		TitleColorB:  t.Primary,
		CharmColor:   t.Secondary,
		VersionColor: t.Primary,
	})
}

func (m *sidebarCmp) lspBlock() string {
	maxWidth := min(m.width, 58)
	t := styles.CurrentTheme()

	section := t.S().Muted.Render(
		core.Section("LSPs", maxWidth),
	)

	lspList := []string{section, ""}

	lsp := config.Get().LSP
	if len(lsp) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			section,
			"",
			t.S().Base.Foreground(t.Border).Render("None"),
		)
	}

	for n, l := range lsp {
		iconColor := t.Success
		if l.Disabled {
			iconColor = t.FgMuted
		}
		lspErrs := map[protocol.DiagnosticSeverity]int{
			protocol.SeverityError:       0,
			protocol.SeverityWarning:     0,
			protocol.SeverityHint:        0,
			protocol.SeverityInformation: 0,
		}
		if client, ok := m.lspClients[n]; ok {
			for _, diagnostics := range client.GetDiagnostics() {
				for _, diagnostic := range diagnostics {
					if severity, ok := lspErrs[diagnostic.Severity]; ok {
						lspErrs[diagnostic.Severity] = severity + 1
					}
				}
			}
		}

		errs := []string{}
		if lspErrs[protocol.SeverityError] > 0 {
			errs = append(errs, t.S().Base.Foreground(t.Error).Render(fmt.Sprintf("%s%d", styles.ErrorIcon, lspErrs[protocol.SeverityError])))
		}
		if lspErrs[protocol.SeverityWarning] > 0 {
			errs = append(errs, t.S().Base.Foreground(t.Warning).Render(fmt.Sprintf("%s%d", styles.WarningIcon, lspErrs[protocol.SeverityWarning])))
		}
		if lspErrs[protocol.SeverityHint] > 0 {
			errs = append(errs, t.S().Base.Foreground(t.FgHalfMuted).Render(fmt.Sprintf("%s%d", styles.HintIcon, lspErrs[protocol.SeverityHint])))
		}
		if lspErrs[protocol.SeverityInformation] > 0 {
			errs = append(errs, t.S().Base.Foreground(t.FgHalfMuted).Render(fmt.Sprintf("%s%d", styles.InfoIcon, lspErrs[protocol.SeverityInformation])))
		}

		logging.Info("LSP Errors", "errors", errs)
		lspList = append(lspList,
			core.Status(
				core.StatusOpts{
					IconColor:    iconColor,
					Title:        n,
					Description:  l.Command,
					ExtraContent: strings.Join(errs, " "),
				},
				m.width,
			),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		lspList...,
	)
}

func (m *sidebarCmp) mcpBlock() string {
	maxWidth := min(m.width, 58)
	t := styles.CurrentTheme()

	section := t.S().Muted.Render(
		core.Section("MCPs", maxWidth),
	)

	mcpList := []string{section, ""}

	mcp := config.Get().MCPServers
	if len(mcp) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			section,
			"",
			t.S().Base.Foreground(t.Border).Render("None"),
		)
	}

	for n, l := range mcp {
		iconColor := t.Success
		mcpList = append(mcpList,
			core.Status(
				core.StatusOpts{
					IconColor:   iconColor,
					Title:       n,
					Description: l.Command,
				},
				m.width,
			),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		mcpList...,
	)
}

func formatTokensAndCost(tokens, contextWindow int64, cost float64) string {
	t := styles.CurrentTheme()
	// Format tokens in human-readable format (e.g., 110K, 1.2M)
	var formattedTokens string
	switch {
	case tokens >= 1_000_000:
		formattedTokens = fmt.Sprintf("%.1fM", float64(tokens)/1_000_000)
	case tokens >= 1_000:
		formattedTokens = fmt.Sprintf("%.1fK", float64(tokens)/1_000)
	default:
		formattedTokens = fmt.Sprintf("%d", tokens)
	}

	// Remove .0 suffix if present
	if strings.HasSuffix(formattedTokens, ".0K") {
		formattedTokens = strings.Replace(formattedTokens, ".0K", "K", 1)
	}
	if strings.HasSuffix(formattedTokens, ".0M") {
		formattedTokens = strings.Replace(formattedTokens, ".0M", "M", 1)
	}

	percentage := (float64(tokens) / float64(contextWindow)) * 100

	baseStyle := t.S().Base

	formattedCost := baseStyle.Foreground(t.FgMuted).Render(fmt.Sprintf("$%.2f", cost))

	formattedTokens = baseStyle.Foreground(t.FgMuted).Render(fmt.Sprintf("(%s)", formattedTokens))
	formattedPercentage := baseStyle.Foreground(t.FgSubtle).Render(fmt.Sprintf("%d%%", int(percentage)))
	formattedTokens = fmt.Sprintf("%s %s", formattedPercentage, formattedTokens)
	if percentage > 80 {
		// add the warning icon
		formattedTokens = fmt.Sprintf("%s %s", styles.WarningIcon, formattedTokens)
	}

	return fmt.Sprintf("%s %s", formattedTokens, formattedCost)
}

func (s *sidebarCmp) currentModelBlock() string {
	cfg := config.Get()
	agentCfg := cfg.Agents[config.AgentCoder]
	selectedModelID := agentCfg.Model
	model := models.SupportedModels[selectedModelID]

	t := styles.CurrentTheme()

	modelIcon := t.S().Base.Foreground(t.FgSubtle).Render(styles.ModelIcon)
	modelName := t.S().Text.Render(model.Name)
	modelInfo := fmt.Sprintf("%s %s", modelIcon, modelName)
	parts := []string{
		// section,
		// "",
		modelInfo,
	}
	if s.session.ID != "" {
		parts = append(
			parts,
			"  "+formatTokensAndCost(
				s.session.CompletionTokens+s.session.PromptTokens,
				model.ContextWindow,
				s.session.Cost,
			),
		)
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		parts...,
	)
}

func cwd() string {
	cwd := config.WorkingDirectory()
	t := styles.CurrentTheme()
	// replace home directory with ~
	homeDir, err := os.UserHomeDir()
	if err == nil {
		cwd = strings.ReplaceAll(cwd, homeDir, "~")
	}
	return t.S().Muted.Render(cwd)
}
