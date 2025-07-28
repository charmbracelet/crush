package sidebar

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
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
	compactMode   bool
	files         *csync.Map[string, SessionFile]
	app           *app.App
}

func New(app *app.App, compact bool) Sidebar {
	return &sidebarCmp{
		app:         app,
		compactMode: compact,
		files:       csync.NewMap[string, SessionFile](),
	}
}

func (m *sidebarCmp) Init() tea.Cmd {
	return nil
}

func (m *sidebarCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			before := existing.History.initialVersion.Content
			after := existing.History.latestVersion.Content
			path := existing.History.initialVersion.Path
			cwd := m.app.Config().WorkingDir()
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
	files, err := m.app.History.ListBySession(context.Background(), m.session.ID)
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
		cwd := m.app.Config().WorkingDir()
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
	m.cwd = cwd(m.app.Config().WorkingDir())
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
	t := styles.CurrentTheme()

	section := t.S().Subtle.Render("Modified Files")

	files := slices.Collect(m.files.Seq())

	if len(files) == 0 {
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			section,
			"",
			t.S().Base.Foreground(t.Border).Render("None"),
		)
		return lipgloss.NewStyle().Width(maxWidth).Render(content)
	}

	fileList := []string{section, ""}
	sort.Slice(files, func(i, j int) bool {
		return files[i].History.latestVersion.CreatedAt > files[j].History.latestVersion.CreatedAt
	})

	// Limit items for horizontal layout - use less space
	maxItems := min(5, len(files))
	availableHeight := m.height - 8 // Reserve space for header and other content
	if availableHeight > 0 {
		maxItems = min(maxItems, availableHeight)
	}

	filesShown := 0
	for _, file := range files {
		if file.Additions == 0 && file.Deletions == 0 {
			continue
		}
		if filesShown >= maxItems {
			break
		}

		var statusParts []string
		if file.Additions > 0 {
			statusParts = append(statusParts, t.S().Base.Foreground(t.Success).Render(fmt.Sprintf("+%d", file.Additions)))
		}
		if file.Deletions > 0 {
			statusParts = append(statusParts, t.S().Base.Foreground(t.Error).Render(fmt.Sprintf("-%d", file.Deletions)))
		}

		extraContent := strings.Join(statusParts, " ")
		cwd := m.app.Config().WorkingDir() + string(os.PathSeparator)
		filePath := file.FilePath
		filePath = strings.TrimPrefix(filePath, cwd)
		filePath = fsext.DirTrim(fsext.PrettyPath(filePath), 2)
		filePath = ansi.Truncate(filePath, maxWidth-lipgloss.Width(extraContent)-2, "…")

		fileList = append(fileList,
			core.Status(
				core.StatusOpts{
					IconColor:    t.FgMuted,
					NoIcon:       true,
					Title:        filePath,
					ExtraContent: extraContent,
				},
				maxWidth,
			),
		)
		filesShown++
	}

	// Add "..." indicator if there are more files
	totalFilesWithChanges := 0
	for _, file := range files {
		if file.Additions > 0 || file.Deletions > 0 {
			totalFilesWithChanges++
		}
	}
	if totalFilesWithChanges > maxItems {
		fileList = append(fileList, t.S().Base.Foreground(t.FgMuted).Render("…"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, fileList...)
	return lipgloss.NewStyle().Width(maxWidth).Render(content)
}

// lspBlockCompact renders the LSP block with limited width and height for horizontal layout
func (m *sidebarCmp) lspBlockCompact(maxWidth int) string {
	t := styles.CurrentTheme()

	section := t.S().Subtle.Render("LSPs")

	lspList := []string{section, ""}

	lsp := m.app.Config().LSP.Sorted()
	if len(lsp) == 0 {
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			section,
			"",
			t.S().Base.Foreground(t.Border).Render("None"),
		)
		return lipgloss.NewStyle().Width(maxWidth).Render(content)
	}

	// Limit items for horizontal layout
	maxItems := min(5, len(lsp))
	availableHeight := m.height - 8
	if availableHeight > 0 {
		maxItems = min(maxItems, availableHeight)
	}

	for i, l := range lsp {
		if i >= maxItems {
			break
		}

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
		if client, ok := m.app.LSPClients[l.Name]; ok {
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
				maxWidth,
			),
		)
	}

	// Add "..." indicator if there are more LSPs
	if len(lsp) > maxItems {
		lspList = append(lspList, t.S().Base.Foreground(t.FgMuted).Render("…"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lspList...)
	return lipgloss.NewStyle().Width(maxWidth).Render(content)
}

// mcpBlockCompact renders the MCP block with limited width and height for horizontal layout
func (m *sidebarCmp) mcpBlockCompact(maxWidth int) string {
	t := styles.CurrentTheme()

	section := t.S().Subtle.Render("MCPs")

	mcpList := []string{section, ""}

	mcps := m.app.Config().MCP.Sorted()
	if len(mcps) == 0 {
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			section,
			"",
			t.S().Base.Foreground(t.Border).Render("None"),
		)
		return lipgloss.NewStyle().Width(maxWidth).Render(content)
	}

	// Limit items for horizontal layout
	maxItems := min(5, len(mcps))
	availableHeight := m.height - 8
	if availableHeight > 0 {
		maxItems = min(maxItems, availableHeight)
	}

	for i, l := range mcps {
		if i >= maxItems {
			break
		}

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
				maxWidth,
			),
		)
	}

	// Add "..." indicator if there are more MCPs
	if len(mcps) > maxItems {
		mcpList = append(mcpList, t.S().Base.Foreground(t.FgMuted).Render("…"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, mcpList...)
	return lipgloss.NewStyle().Width(maxWidth).Render(content)
}

func (m *sidebarCmp) filesBlock() string {
	t := styles.CurrentTheme()

	section := t.S().Subtle.Render(
		core.Section("Modified Files", m.getMaxWidth()),
	)

	files := slices.Collect(m.files.Seq())
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

	// Limit the number of files shown
	maxFiles, _, _ := m.getDynamicLimits()
	maxFiles = min(len(files), maxFiles)
	filesShown := 0

	for _, file := range files {
		if file.Additions == 0 && file.Deletions == 0 {
			continue // skip files with no changes
		}
		if filesShown >= maxFiles {
			break
		}

		var statusParts []string
		if file.Additions > 0 {
			statusParts = append(statusParts, t.S().Base.Foreground(t.Success).Render(fmt.Sprintf("+%d", file.Additions)))
		}
		if file.Deletions > 0 {
			statusParts = append(statusParts, t.S().Base.Foreground(t.Error).Render(fmt.Sprintf("-%d", file.Deletions)))
		}

		extraContent := strings.Join(statusParts, " ")
		cwd := m.app.Config().WorkingDir() + string(os.PathSeparator)
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
		filesShown++
	}

	// Add indicator if there are more files
	totalFilesWithChanges := 0
	for _, file := range files {
		if file.Additions > 0 || file.Deletions > 0 {
			totalFilesWithChanges++
		}
	}
	if totalFilesWithChanges > maxFiles {
		remaining := totalFilesWithChanges - maxFiles
		fileList = append(fileList,
			t.S().Base.Foreground(t.FgSubtle).Render(fmt.Sprintf("…and %d more", remaining)),
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

	lsp := m.app.Config().LSP.Sorted()
	if len(lsp) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			section,
			"",
			t.S().Base.Foreground(t.Border).Render("None"),
		)
	}

	// Limit the number of LSPs shown
	_, maxLSPs, _ := m.getDynamicLimits()
	maxLSPs = min(len(lsp), maxLSPs)
	for i, l := range lsp {
		if i >= maxLSPs {
			break
		}

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
		if client, ok := m.app.LSPClients[l.Name]; ok {
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

	// Add indicator if there are more LSPs
	if len(lsp) > maxLSPs {
		remaining := len(lsp) - maxLSPs
		lspList = append(lspList,
			t.S().Base.Foreground(t.FgSubtle).Render(fmt.Sprintf("…and %d more", remaining)),
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

	mcps := m.app.Config().MCP.Sorted()
	if len(mcps) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			section,
			"",
			t.S().Base.Foreground(t.Border).Render("None"),
		)
	}

	// Limit the number of MCPs shown
	_, _, maxMCPs := m.getDynamicLimits()
	maxMCPs = min(len(mcps), maxMCPs)
	for i, l := range mcps {
		if i >= maxMCPs {
			break
		}

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

	// Add indicator if there are more MCPs
	if len(mcps) > maxMCPs {
		remaining := len(mcps) - maxMCPs
		mcpList = append(mcpList,
			t.S().Base.Foreground(t.FgSubtle).Render(fmt.Sprintf("…and %d more", remaining)),
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
	model := s.app.CoderAgent.Model()
	selectedModel := s.app.CoderAgent.ModelConfig()
	modelProvider := s.app.CoderAgent.Provider()

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
		case catwalk.TypeOpenAI:
			reasoningEffort := model.DefaultReasoningEffort
			if selectedModel.ReasoningEffort != "" {
				reasoningEffort = selectedModel.ReasoningEffort
			}
			formatter := cases.Title(language.English, cases.NoLower)
			parts = append(parts, reasoningInfoStyle.Render(formatter.String(fmt.Sprintf("Reasoning %s", reasoningEffort))))
		case catwalk.TypeAnthropic:
			formatter := cases.Title(language.English, cases.NoLower)
			if selectedModel.Think {
				parts = append(parts, reasoningInfoStyle.Render(formatter.String("Thinking on")))
			} else {
				parts = append(parts, reasoningInfoStyle.Render(formatter.String("Thinking off")))
			}
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

func cwd(cwd string) string {
	t := styles.CurrentTheme()
	// Replace home directory with ~, unless we're at the top level of the
	// home directory).
	homeDir, err := os.UserHomeDir()
	if err == nil && cwd != homeDir {
		cwd = strings.ReplaceAll(cwd, homeDir, "~")
	}
	return t.S().Muted.Render(cwd)
}
