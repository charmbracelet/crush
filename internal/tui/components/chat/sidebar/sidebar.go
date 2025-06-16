package sidebar

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
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
	"github.com/charmbracelet/x/ansi"
)

const (
	logoBreakpoint = 65
)

type SessionFile struct {
	FilePath  string
	Additions int
	Deletions int
}
type SessionFilesMsg struct {
	Files []SessionFile
}

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
	history       history.Service
	files         []SessionFile
}

func NewSidebarCmp(history history.Service, lspClients map[string]*lsp.Client) Sidebar {
	return &sidebarCmp{
		lspClients: lspClients,
		history:    history,
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
		return m, m.loadSessionFiles
	case SessionFilesMsg:
		m.files = msg.Files
		logging.Info("Loaded session files", "count", len(m.files))
		return m, nil

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
		m.filesBlock(),
		"",
		m.lspBlock(),
		"",
		m.mcpBlock(),
	)

	return tea.NewView(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)
}

func (m *sidebarCmp) loadSessionFiles() tea.Msg {
	files, err := m.history.ListBySession(context.Background(), m.session.ID)
	if err != nil {
		return util.InfoMsg{
			Type: util.InfoTypeError,
			Msg:  err.Error(),
		}
	}

	type fileHistory struct {
		initialVersion history.File
		latestVersion  history.File
	}

	fileMap := make(map[string]fileHistory)

	for _, file := range files {
		if existing, ok := fileMap[file.Path]; ok {
			// Update the latest version
			if existing.latestVersion.CreatedAt < file.CreatedAt {
				existing.latestVersion = file
			}
			if file.Version == history.InitialVersion {
				existing.initialVersion = file
			}
			fileMap[file.Path] = existing
		} else {
			// Add the initial version
			fileMap[file.Path] = fileHistory{
				initialVersion: file,
				latestVersion:  file,
			}
		}
	}

	sessionFiles := make([]SessionFile, 0, len(fileMap))
	for path, fh := range fileMap {
		if fh.initialVersion.Version == history.InitialVersion {
			_, additions, deletions := diff.GenerateDiff(fh.initialVersion.Content, fh.latestVersion.Content, fh.initialVersion.Path)
			sessionFiles = append(sessionFiles, SessionFile{
				FilePath:  path,
				Additions: additions,
				Deletions: deletions,
			})
		}
	}

	return SessionFilesMsg{
		Files: sessionFiles,
	}
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

func (m *sidebarCmp) filesBlock() string {
	maxWidth := min(m.width, 58)
	t := styles.CurrentTheme()

	section := t.S().Subtle.Render(
		core.Section("Modified Files", maxWidth),
	)

	if len(m.files) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			section,
			"",
			t.S().Base.Foreground(t.Border).Render("None"),
		)
	}

	fileList := []string{section, ""}

	for _, file := range m.files {
		// Extract just the filename from the path

		// Create status indicators for additions/deletions
		var statusParts []string
		if file.Additions > 0 {
			statusParts = append(statusParts, t.S().Base.Foreground(t.Success).Render(fmt.Sprintf("+%d", file.Additions)))
		}
		if file.Deletions > 0 {
			statusParts = append(statusParts, t.S().Base.Foreground(t.Error).Render(fmt.Sprintf("-%d", file.Deletions)))
		}

		extraContent := strings.Join(statusParts, " ")
		filePath := fsext.DirTrim(fsext.PrettyPath(file.FilePath), 2)
		filePath = ansi.Truncate(filePath, maxWidth-lipgloss.Width(extraContent)-2, "…")
		fileList = append(fileList,
			core.Status(
				core.StatusOpts{
					IconColor:    t.FgMuted,
					NoIcon:       true,
					Title:        filePath,
					ExtraContent: extraContent,
				},
				m.width,
			),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		fileList...,
	)
}

func (m *sidebarCmp) lspBlock() string {
	maxWidth := min(m.width, 58)
	t := styles.CurrentTheme()

	section := t.S().Subtle.Render(
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

	section := t.S().Subtle.Render(
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

	formattedTokens = baseStyle.Foreground(t.FgSubtle).Render(fmt.Sprintf("(%s)", formattedTokens))
	formattedPercentage := baseStyle.Foreground(t.FgMuted).Render(fmt.Sprintf("%d%%", int(percentage)))
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
