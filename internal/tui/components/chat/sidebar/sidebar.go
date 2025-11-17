package sidebar

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/components/files"
	"github.com/charmbracelet/crush/internal/tui/components/logo"
	lspcomponent "github.com/charmbracelet/crush/internal/tui/components/lsp"
	"github.com/charmbracelet/crush/internal/tui/components/mcp"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/crush/internal/version"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type FileHistory struct {
	initialVersion history.File
	latestVersion  history.File
}

const LogoHeightBreakpoint = 30

// Default maximum number of items to show in each section
const (
	DefaultMaxFilesShown = 10
	DefaultMaxLSPsShown  = 8
	DefaultMaxMCPsShown  = 8
	MinItemsPerSection   = 2 // Minimum items to show per section
)

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
	lspClients    *csync.Map[string, *lsp.Client]
	compactMode   bool
	history       history.Service
	files         *csync.Map[string, SessionFile]
}

func New(history history.Service, lspClients *csync.Map[string, *lsp.Client], compact bool) Sidebar {
	return &sidebarCmp{
		lspClients:  lspClients,
		history:     history,
		compactMode: compact,
		files:       csync.NewMap[string, SessionFile](),
	}
}

func (m *sidebarCmp) Init() tea.Cmd {
	return nil
}

func (m *sidebarCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SessionFilesMsg:
		m.files = csync.NewMap[string, SessionFile]()
		for _, file := range msg.Files {
			m.files.Set(file.FilePath, file)
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

	style := t.S().Base.
		Width(m.width).
		Height(m.height).
		Padding(1)
	if m.compactMode {
		style = style.PaddingTop(0)
	}

	if !m.compactMode {
		if m.height > LogoHeightBreakpoint {
			parts = append(parts, m.logo)
		} else {
			// Use a smaller logo for smaller screens
			parts = append(parts,
				logo.SmallRender(m.width-style.GetHorizontalFrameSize()),
				"")
		}
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

	// Check if we should use horizontal layout for sections
	if m.compactMode && m.width > m.height {
		// Horizontal layout for compact mode when width > height
		sectionsContent := m.renderSectionsHorizontal()
		if sectionsContent != "" {
			parts = append(parts, "", sectionsContent)
		}
	} else {
		// Vertical layout (default)
		if m.session.ID != "" {
			parts = append(parts, "", m.filesBlock())
		}
		parts = append(parts,
			"",
			m.lspBlock(),
			"",
			m.mcpBlock(),
		)
	}

	return style.Render(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)
}

func (m *sidebarCmp) handleFileHistoryEvent(event pubsub.Event[history.File]) tea.Cmd {
	return func() tea.Msg {
		file := event.Payload
		found := false
		for existing := range m.files.Seq() {
			if existing.FilePath != file.Path {
				continue
			}
			if existing.History.latestVersion.Version < file.Version {
				existing.History.latestVersion = file
			} else if file.Version == 0 {
				existing.History.initialVersion = file
			} else {
				// If the version is not greater than the latest, we ignore it
				continue
			}
			before, _ := fsext.ToUnixLineEndings(existing.History.initialVersion.Content)
			after, _ := fsext.ToUnixLineEndings(existing.History.latestVersion.Content)
			path := existing.History.initialVersion.Path
			cwd := config.Get().WorkingDir()
			path = strings.TrimPrefix(path, cwd)
			_, additions, deletions := diff.GenerateDiff(before, after, path)
			existing.Additions = additions
			existing.Deletions = deletions
			m.files.Set(file.Path, existing)
			found = true
			break
		}
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
		m.files.Set(file.Path, sf)
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
		before, _ := fsext.ToUnixLineEndings(fh.initialVersion.Content)
		after, _ := fsext.ToUnixLineEndings(fh.latestVersion.Content)
		_, additions, deletions := diff.GenerateDiff(before, after, path)
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

// calculateAvailableHeight estimates how much height is available for dynamic content
func (m *sidebarCmp) calculateAvailableHeight() int {
	usedHeight := 0

	if !m.compactMode {
		if m.height > LogoHeightBreakpoint {
			usedHeight += 7 // Approximate logo height
		} else {
			usedHeight += 2 // Smaller logo height
		}
		usedHeight += 1 // Empty line after logo
	}

	if m.session.ID != "" {
		usedHeight += 1 // Title line
		usedHeight += 1 // Empty line after title
	}

	if !m.compactMode {
		usedHeight += 1 // CWD line
		usedHeight += 1 // Empty line after CWD
	}

	usedHeight += 2 // Model info

	usedHeight += 6 // 3 sections × 2 lines each (header + empty line)

	// Base padding
	usedHeight += 2 // Top and bottom padding

	return max(0, m.height-usedHeight)
}

// getDynamicLimits calculates how many items to show in each section based on available height
func (m *sidebarCmp) getDynamicLimits() (maxFiles, maxLSPs, maxMCPs int) {
	availableHeight := m.calculateAvailableHeight()

	// If we have very little space, use minimum values
	if availableHeight < 10 {
		return MinItemsPerSection, MinItemsPerSection, MinItemsPerSection
	}

	// Distribute available height among the three sections
	// Give priority to files, then LSPs, then MCPs
	totalSections := 3
	heightPerSection := availableHeight / totalSections

	// Calculate limits for each section, ensuring minimums
	maxFiles = max(MinItemsPerSection, min(DefaultMaxFilesShown, heightPerSection))
	maxLSPs = max(MinItemsPerSection, min(DefaultMaxLSPsShown, heightPerSection))
	maxMCPs = max(MinItemsPerSection, min(DefaultMaxMCPsShown, heightPerSection))

	// If we have extra space, give it to files first
	remainingHeight := availableHeight - (maxFiles + maxLSPs + maxMCPs)
	if remainingHeight > 0 {
		extraForFiles := min(remainingHeight, DefaultMaxFilesShown-maxFiles)
		maxFiles += extraForFiles
		remainingHeight -= extraForFiles

		if remainingHeight > 0 {
			extraForLSPs := min(remainingHeight, DefaultMaxLSPsShown-maxLSPs)
			maxLSPs += extraForLSPs
			remainingHeight -= extraForLSPs

			if remainingHeight > 0 {
				maxMCPs += min(remainingHeight, DefaultMaxMCPsShown-maxMCPs)
			}
		}
	}

	return maxFiles, maxLSPs, maxMCPs
}

// renderSectionsHorizontal renders the files, LSPs, and MCPs sections horizontally
func (m *sidebarCmp) renderSectionsHorizontal() string {
	// Calculate available width for each section
	totalWidth := m.width - 4 // Account for padding and spacing
	sectionWidth := min(50, totalWidth/3)

	// Get the sections content with limited height
	var filesContent, lspContent, mcpContent string

	filesContent = m.filesBlockCompact(sectionWidth)
	lspContent = m.lspBlockCompact(sectionWidth)
	mcpContent = m.mcpBlockCompact(sectionWidth)

	return lipgloss.JoinHorizontal(lipgloss.Top, filesContent, " ", lspContent, " ", mcpContent)
}

// filesBlockCompact renders the files block with limited width and height for horizontal layout
func (m *sidebarCmp) filesBlockCompact(maxWidth int) string {
	// Convert map to slice and handle type conversion
	sessionFiles := slices.Collect(m.files.Seq())
	fileSlice := make([]files.SessionFile, len(sessionFiles))
	for i, sf := range sessionFiles {
		fileSlice[i] = files.SessionFile{
			History: files.FileHistory{
				InitialVersion: sf.History.initialVersion,
				LatestVersion:  sf.History.latestVersion,
			},
			FilePath:  sf.FilePath,
			Additions: sf.Additions,
			Deletions: sf.Deletions,
		}
	}

	// Limit items for horizontal layout
	maxItems := min(5, len(fileSlice))
	availableHeight := m.height - 8 // Reserve space for header and other content
	if availableHeight > 0 {
		maxItems = min(maxItems, availableHeight)
	}

	return files.RenderFileBlock(fileSlice, files.RenderOptions{
		MaxWidth:    maxWidth,
		MaxItems:    maxItems,
		ShowSection: true,
		SectionName: "Modified Files",
	}, true)
}

// lspBlockCompact renders the LSP block with limited width and height for horizontal layout
func (m *sidebarCmp) lspBlockCompact(maxWidth int) string {
	// Limit items for horizontal layout
	lspConfigs := config.Get().LSP.Sorted()
	maxItems := min(5, len(lspConfigs))
	availableHeight := m.height - 8
	if availableHeight > 0 {
		maxItems = min(maxItems, availableHeight)
	}

	return lspcomponent.RenderLSPBlock(m.lspClients, lspcomponent.RenderOptions{
		MaxWidth:    maxWidth,
		MaxItems:    maxItems,
		ShowSection: true,
		SectionName: "LSPs",
	}, true)
}

// mcpBlockCompact renders the MCP block with limited width and height for horizontal layout
func (m *sidebarCmp) mcpBlockCompact(maxWidth int) string {
	// Limit items for horizontal layout
	maxItems := min(5, len(config.Get().MCP.Sorted()))
	availableHeight := m.height - 8
	if availableHeight > 0 {
		maxItems = min(maxItems, availableHeight)
	}

	return mcp.RenderMCPBlock(mcp.RenderOptions{
		MaxWidth:    maxWidth,
		MaxItems:    maxItems,
		ShowSection: true,
		SectionName: "MCPs",
	}, true)
}

func (m *sidebarCmp) filesBlock() string {
	// Convert map to slice and handle type conversion
	sessionFiles := slices.Collect(m.files.Seq())
	fileSlice := make([]files.SessionFile, len(sessionFiles))
	for i, sf := range sessionFiles {
		fileSlice[i] = files.SessionFile{
			History: files.FileHistory{
				InitialVersion: sf.History.initialVersion,
				LatestVersion:  sf.History.latestVersion,
			},
			FilePath:  sf.FilePath,
			Additions: sf.Additions,
			Deletions: sf.Deletions,
		}
	}

	// Limit the number of files shown
	maxFiles, _, _ := m.getDynamicLimits()
	maxFiles = min(len(fileSlice), maxFiles)

	return files.RenderFileBlock(fileSlice, files.RenderOptions{
		MaxWidth:    m.getMaxWidth(),
		MaxItems:    maxFiles,
		ShowSection: true,
		SectionName: core.Section("Modified Files", m.getMaxWidth()),
	}, true)
}

func (m *sidebarCmp) lspBlock() string {
	// Limit the number of LSPs shown
	_, maxLSPs, _ := m.getDynamicLimits()
	lspConfigs := config.Get().LSP.Sorted()
	maxLSPs = min(len(lspConfigs), maxLSPs)

	return lspcomponent.RenderLSPBlock(m.lspClients, lspcomponent.RenderOptions{
		MaxWidth:    m.getMaxWidth(),
		MaxItems:    maxLSPs,
		ShowSection: true,
		SectionName: core.Section("LSPs", m.getMaxWidth()),
	}, true)
}

func (m *sidebarCmp) mcpBlock() string {
	// Limit the number of MCPs shown
	_, _, maxMCPs := m.getDynamicLimits()
	mcps := config.Get().MCP.Sorted()
	maxMCPs = min(len(mcps), maxMCPs)

	return mcp.RenderMCPBlock(mcp.RenderOptions{
		MaxWidth:    m.getMaxWidth(),
		MaxItems:    maxMCPs,
		ShowSection: true,
		SectionName: core.Section("MCPs", m.getMaxWidth()),
	}, true)
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

	selectedModel := cfg.Models[agentCfg.Model]

	model := config.Get().GetModelByType(agentCfg.Model)
	modelProvider := config.Get().GetProviderForModel(agentCfg.Model)

	t := styles.CurrentTheme()

	modelIcon := t.S().Base.Foreground(t.FgSubtle).Render(styles.ModelIcon)
	modelName := t.S().Text.Render(model.Name)
	modelInfo := fmt.Sprintf("%s %s", modelIcon, modelName)
	parts := []string{
		modelInfo,
	}
	if model.CanReason {
		reasoningInfoStyle := t.S().Subtle.PaddingLeft(2)
		switch modelProvider.Type {
		case catwalk.TypeAnthropic:
			formatter := cases.Title(language.English, cases.NoLower)
			if selectedModel.Think {
				parts = append(parts, reasoningInfoStyle.Render(formatter.String("Thinking on")))
			} else {
				parts = append(parts, reasoningInfoStyle.Render(formatter.String("Thinking off")))
			}
		default:
			reasoningEffort := model.DefaultReasoningEffort
			if selectedModel.ReasoningEffort != "" {
				reasoningEffort = selectedModel.ReasoningEffort
			}
			formatter := cases.Title(language.English, cases.NoLower)
			parts = append(parts, reasoningInfoStyle.Render(formatter.String(fmt.Sprintf("Reasoning %s", reasoningEffort))))
		}
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
	return t.S().Muted.Render(home.Short(cwd))
}
