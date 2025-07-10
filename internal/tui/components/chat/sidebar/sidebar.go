package sidebar

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
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

type FileHistory struct {
	initialVersion history.File
	latestVersion  history.File
}

type SessionFile struct {
	History   FileHistory
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
	SetSession(session session.Session) tea.Cmd
	SetCompactMode(bool)
}

type sidebarCmp struct {
	width, height int
	session       session.Session
	logo          string
	cwd           string
	lspClients    map[string]*lsp.Client
	compactMode   bool
	history       history.Service
	// Using a sync map here because we might receive file history events concurrently
	files sync.Map
}

func New(history history.Service, lspClients map[string]*lsp.Client, compact bool) Sidebar {
	return &sidebarCmp{
		lspClients:  lspClients,
		history:     history,
		compactMode: compact,
	}
}

func (m *sidebarCmp) Init() tea.Cmd {
	return nil
}

func (m *sidebarCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SessionFilesMsg:
		m.files = sync.Map{}
		for _, file := range msg.Files {
			m.files.Store(file.FilePath, file)
		}
		return m, nil

	case chat.SessionClearedMsg:
		m.session = session.Session{}
	case pubsub.Event[history.File]:
		return m, m.handleFileHistoryEvent(msg)
	case pubsub.Event[session.Session]:
		if msg.Type == pubsub.UpdatedEvent {
			if m.session.ID == msg.Payload.ID {
				m.session = msg.Payload
			}
		}
	}
	return m, nil
}

func (m *sidebarCmp) View() string {
	t := styles.CurrentTheme()
	parts := []string{}
	if !m.compactMode {
		parts = append(parts, m.logo)
	}

	if !m.compactMode && m.session.ID != "" {
		parts = append(parts, t.S().Muted.Render(m.session.Title), "")
	} else if m.session.ID != "" {
		parts = append(parts, t.S().Text.Render(m.session.Title), "")
	}

	if !m.compactMode {
		parts = append(parts,
			m.cwd,
			"",
		)
	}
	parts = append(parts,
		m.currentModelBlock(),
	)
	if m.session.ID != "" {
		parts = append(parts, "", m.filesBlock())
	}
	parts = append(parts,
		"",
		m.lspBlock(),
		"",
		m.mcpBlock(),
	)

	style := t.S().Base.
		Width(m.width).
		Height(m.height).
		Padding(1)
	if m.compactMode {
		style = style.PaddingTop(0)
	}
	return style.Render(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)
}

func (m *sidebarCmp) handleFileHistoryEvent(event pubsub.Event[history.File]) tea.Cmd {
	return func() tea.Msg {
		file := event.Payload
		found := false
		m.files.Range(func(key, value any) bool {
			existing := value.(SessionFile)
			if existing.FilePath == file.Path {
				if existing.History.latestVersion.Version < file.Version {
					existing.History.latestVersion = file
				} else if file.Version == 0 {
					existing.History.initialVersion = file
				} else {
					// If the version is not greater than the latest, we ignore it
					return true
				}
				before := existing.History.initialVersion.Content
				after := existing.History.latestVersion.Content
				path := existing.History.initialVersion.Path
				cwd := config.Get().WorkingDir()
				path = strings.TrimPrefix(path, cwd)
				_, additions, deletions := diff.GenerateDiff(before, after, path)
				existing.Additions = additions
				existing.Deletions = deletions
				m.files.Store(file.Path, existing)
				found = true
				return false
			}
			return true
		})
		if found {
			return nil
		}
		sf := SessionFile{
			History: FileHistory{
				initialVersion: file,
				latestVersion:  file,
			},
			FilePath:  file.Path,
			Additions: 0,
			Deletions: 0,
		}
		m.files.Store(file.Path, sf)
		return nil
	}
}

func (m *sidebarCmp) loadSessionFiles() tea.Msg {
	files, err := m.history.ListBySession(context.Background(), m.session.ID)
	if err != nil {
		return util.InfoMsg{
			Type: util.InfoTypeError,
			Msg:  err.Error(),
		}
	}

	fileMap := make(map[string]FileHistory)

	for _, file := range files {
		if existing, ok := fileMap[file.Path]; ok {
			// Update the latest version
			existing.latestVersion = file
			fileMap[file.Path] = existing
		} else {
			// Add the initial version
			fileMap[file.Path] = FileHistory{
				initialVersion: file,
				latestVersion:  file,
			}
		}
	}

	sessionFiles := make([]SessionFile, 0, len(fileMap))
	for path, fh := range fileMap {
		cwd := config.Get().WorkingDir()
		path = strings.TrimPrefix(path, cwd)
		_, additions, deletions := diff.GenerateDiff(fh.initialVersion.Content, fh.latestVersion.Content, path)
		sessionFiles = append(sessionFiles, SessionFile{
			History:   fh,
			FilePath:  path,
			Additions: additions,
			Deletions: deletions,
		})
	}

	return SessionFilesMsg{
		Files: sessionFiles,
	}
}

func (m *sidebarCmp) SetSize(width, height int) tea.Cmd {
	m.logo = m.logoBlock()
	m.cwd = cwd()
	m.width = width
	m.height = height
	return nil
}

func (m *sidebarCmp) GetSize() (int, int) {
	return m.width, m.height
}

func (m *sidebarCmp) logoBlock() string {
	t := styles.CurrentTheme()
	return logo.Render(version.Version, true, logo.Opts{
		FieldColor:   t.Primary,
		TitleColorA:  t.Secondary,
		TitleColorB:  t.Primary,
		CharmColor:   t.Secondary,
		VersionColor: t.Primary,
		Width:        m.width - 2,
	})
}

func (m *sidebarCmp) getMaxWidth() int {
	return min(m.width-2, 58) // -2 for padding
}

func (m *sidebarCmp) filesBlock() string {
	t := styles.CurrentTheme()

	section := t.S().Subtle.Render(
		core.Section("Modified Files", m.getMaxWidth()),
	)

	files := make([]SessionFile, 0)
	m.files.Range(func(key, value any) bool {
		file := value.(SessionFile)
		files = append(files, file)
		return true // continue iterating
	})
	if len(files) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			section,
			"",
			t.S().Base.Foreground(t.Border).Render("None"),
		)
	}

	fileList := []string{section, ""}
	// order files by the latest version's created time
	sort.Slice(files, func(i, j int) bool {
		return files[i].History.latestVersion.CreatedAt > files[j].History.latestVersion.CreatedAt
	})

	for _, file := range files {
		if file.Additions == 0 && file.Deletions == 0 {
			continue // skip files with no changes
		}
		var statusParts []string
		if file.Additions > 0 {
			statusParts = append(statusParts, t.S().Base.Foreground(t.Success).Render(fmt.Sprintf("+%d", file.Additions)))
		}
		if file.Deletions > 0 {
			statusParts = append(statusParts, t.S().Base.Foreground(t.Error).Render(fmt.Sprintf("-%d", file.Deletions)))
		}

		extraContent := strings.Join(statusParts, " ")
		cwd := config.Get().WorkingDir() + string(os.PathSeparator)
		filePath := file.FilePath
		filePath = strings.TrimPrefix(filePath, cwd)
		filePath = fsext.DirTrim(fsext.PrettyPath(filePath), 2)
		filePath = ansi.Truncate(filePath, m.getMaxWidth()-lipgloss.Width(extraContent)-2, "…")
		fileList = append(fileList,
			core.Status(
				core.StatusOpts{
					IconColor:    t.FgMuted,
					NoIcon:       true,
					Title:        filePath,
					ExtraContent: extraContent,
				},
				m.getMaxWidth(),
			),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		fileList...,
	)
}

func (m *sidebarCmp) lspBlock() string {
	t := styles.CurrentTheme()

	section := t.S().Subtle.Render(
		core.Section("LSPs", m.getMaxWidth()),
	)

	lspList := []string{section, ""}

	lsp := config.Get().LSP.Sorted()
	if len(lsp) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			section,
			"",
			t.S().Base.Foreground(t.Border).Render("None"),
		)
	}

	for _, l := range lsp {
		iconColor := t.Success
		if l.LSP.Disabled {
			iconColor = t.FgMuted
		}
		lspErrs := map[protocol.DiagnosticSeverity]int{
			protocol.SeverityError:       0,
			protocol.SeverityWarning:     0,
			protocol.SeverityHint:        0,
			protocol.SeverityInformation: 0,
		}
		if client, ok := m.lspClients[l.Name]; ok {
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
			errs = append(errs, t.S().Base.Foreground(t.Error).Render(fmt.Sprintf("%s %d", styles.ErrorIcon, lspErrs[protocol.SeverityError])))
		}
		if lspErrs[protocol.SeverityWarning] > 0 {
			errs = append(errs, t.S().Base.Foreground(t.Warning).Render(fmt.Sprintf("%s %d", styles.WarningIcon, lspErrs[protocol.SeverityWarning])))
		}
		if lspErrs[protocol.SeverityHint] > 0 {
			errs = append(errs, t.S().Base.Foreground(t.FgHalfMuted).Render(fmt.Sprintf("%s %d", styles.HintIcon, lspErrs[protocol.SeverityHint])))
		}
		if lspErrs[protocol.SeverityInformation] > 0 {
			errs = append(errs, t.S().Base.Foreground(t.FgHalfMuted).Render(fmt.Sprintf("%s %d", styles.InfoIcon, lspErrs[protocol.SeverityInformation])))
		}

		lspList = append(lspList,
			core.Status(
				core.StatusOpts{
					IconColor:    iconColor,
					Title:        l.Name,
					Description:  l.LSP.Command,
					ExtraContent: strings.Join(errs, " "),
				},
				m.getMaxWidth(),
			),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		lspList...,
	)
}

func (m *sidebarCmp) mcpBlock() string {
	t := styles.CurrentTheme()

	section := t.S().Subtle.Render(
		core.Section("MCPs", m.getMaxWidth()),
	)

	mcpList := []string{section, ""}

	mcps := config.Get().MCP.Sorted()
	if len(mcps) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			section,
			"",
			t.S().Base.Foreground(t.Border).Render("None"),
		)
	}

	for _, l := range mcps {
		iconColor := t.Success
		if l.MCP.Disabled {
			iconColor = t.FgMuted
		}
		mcpList = append(mcpList,
			core.Status(
				core.StatusOpts{
					IconColor:   iconColor,
					Title:       l.Name,
					Description: l.MCP.Command,
				},
				m.getMaxWidth(),
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
	agentCfg := config.Get().Agents["coder"]
	model := config.Get().GetModelByType(agentCfg.Model)

	t := styles.CurrentTheme()

	modelIcon := t.S().Base.Foreground(t.FgSubtle).Render(styles.ModelIcon)
	modelName := t.S().Text.Render(model.Model)
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

// SetSession implements Sidebar.
func (m *sidebarCmp) SetSession(session session.Session) tea.Cmd {
	m.session = session
	return m.loadSessionFiles
}

// SetCompactMode sets the compact mode for the sidebar.
func (m *sidebarCmp) SetCompactMode(compact bool) {
	m.compactMode = compact
}

func cwd() string {
	cwd := config.Get().WorkingDir()
	t := styles.CurrentTheme()
	// Replace home directory with ~, unless we're at the top level of the
	// home directory).
	homeDir, err := os.UserHomeDir()
	if err == nil && cwd != homeDir {
		cwd = strings.ReplaceAll(cwd, homeDir, "~")
	}
	return t.S().Muted.Render(cwd)
}
