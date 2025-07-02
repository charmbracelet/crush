package permissions

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/exp/diffview"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// diffCacheKey represents the parameters that affect diff computation
type diffCacheKey struct {
	filePath   string
	oldContent string
	newContent string
	splitMode  bool
}

// Equals checks if two cache keys are identical
func (k diffCacheKey) Equals(other diffCacheKey) bool {
	return k.filePath == other.filePath &&
		k.oldContent == other.oldContent &&
		k.newContent == other.newContent &&
		k.splitMode == other.splitMode
}

// diffComputeMsg is sent when diff computation starts
type diffComputeMsg struct {
	key diffCacheKey
}

// diffResultMsg is sent when diff computation completes
type diffResultMsg struct {
	key    diffCacheKey
	result *diffview.DiffView
	err    error
}

type PermissionAction string

// Permission responses
const (
	PermissionAllow           PermissionAction = "allow"
	PermissionAllowForSession PermissionAction = "allow_session"
	PermissionDeny            PermissionAction = "deny"

	PermissionsDialogID dialogs.DialogID = "permissions"
)

// PermissionResponseMsg represents the user's response to a permission request
type PermissionResponseMsg struct {
	Permission permission.PermissionRequest
	Action     PermissionAction
}

// PermissionDialogCmp interface for permission dialog component
type PermissionDialogCmp interface {
	dialogs.DialogModel
}

// permissionDialogCmp is the implementation of PermissionDialog
type permissionDialogCmp struct {
	wWidth          int
	wHeight         int
	width           int
	height          int
	permission      permission.PermissionRequest
	contentViewPort viewport.Model
	selectedOption  int // 0: Allow, 1: Allow for session, 2: Deny

	// Diff view state
	diffSplitMode bool // true for split, false for unified
	diffXOffset   int  // horizontal scroll offset
	diffYOffset   int  // vertical scroll offset

	// Caching
	cachedContent string
	contentDirty  bool

	// Diff caching - cache the computed diff to avoid recomputing on scroll/resize
	cachedDiff     *diffview.DiffView
	lastDiffParams diffCacheKey

	// Async diff computation
	isComputingDiff bool
	pendingDiffKey  diffCacheKey

	// Scroll optimization - only regenerate content when viewport changes significantly
	lastViewportHeight int
	lastViewportWidth  int

	keyMap KeyMap
}

func NewPermissionDialogCmp(permission permission.PermissionRequest) PermissionDialogCmp {
	// Create viewport for content
	contentViewport := viewport.New()
	return &permissionDialogCmp{
		contentViewPort: contentViewport,
		selectedOption:  0, // Default to "Allow"
		permission:      permission,
		keyMap:          DefaultKeyMap(),
		contentDirty:    true, // Mark as dirty initially
	}
}

func (p *permissionDialogCmp) Init() tea.Cmd {
	return p.contentViewPort.Init()
}

func (p *permissionDialogCmp) supportsDiffView() bool {
	return p.permission.ToolName == tools.EditToolName || p.permission.ToolName == tools.WriteToolName
}

// invalidateDiffCache clears the cached diff when parameters change
func (p *permissionDialogCmp) invalidateDiffCache() {
	p.cachedDiff = nil
	p.lastDiffParams = diffCacheKey{}
	p.contentDirty = true
	// Also cancel any pending async computation
	p.isComputingDiff = false
	p.pendingDiffKey = diffCacheKey{}
}

// shouldRegenerateContent checks if content needs regeneration based on viewport changes
func (p *permissionDialogCmp) shouldRegenerateContent() bool {
	if p.contentDirty {
		return true
	}

	// Only regenerate if viewport size changed significantly (more than 10% or 5 lines/cols)
	heightDiff := abs(p.contentViewPort.Height() - p.lastViewportHeight)
	widthDiff := abs(p.contentViewPort.Width() - p.lastViewportWidth)

	return heightDiff > max(5, p.lastViewportHeight/10) ||
		widthDiff > max(5, p.lastViewportWidth/10)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (p *permissionDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case diffResultMsg:
		// Handle async diff computation result
		if msg.key.Equals(p.pendingDiffKey) {
			p.isComputingDiff = false
			p.pendingDiffKey = diffCacheKey{} // Clear pending key
			if msg.err == nil {
				p.cachedDiff = msg.result
				p.lastDiffParams = msg.key
				p.contentDirty = true // Mark content dirty to trigger re-render
			}
		}
		return p, nil
	case tea.WindowSizeMsg:
		p.wWidth = msg.Width
		p.wHeight = msg.Height
		// Only mark content dirty if viewport size changed significantly
		if p.shouldRegenerateContent() {
			p.contentDirty = true
			// Cancel any pending async computation since viewport changed significantly
			if p.isComputingDiff {
				p.isComputingDiff = false
				p.pendingDiffKey = diffCacheKey{}
			}
		}
		cmd := p.SetSize()
		cmds = append(cmds, cmd)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, p.keyMap.Right) || key.Matches(msg, p.keyMap.Tab):
			p.selectedOption = (p.selectedOption + 1) % 3
			return p, nil
		case key.Matches(msg, p.keyMap.Left):
			p.selectedOption = (p.selectedOption + 2) % 3
		case key.Matches(msg, p.keyMap.Select):
			return p, p.selectCurrentOption()
		case key.Matches(msg, p.keyMap.Allow):
			return p, tea.Batch(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(PermissionResponseMsg{Action: PermissionAllow, Permission: p.permission}),
			)
		case key.Matches(msg, p.keyMap.AllowSession):
			return p, tea.Batch(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(PermissionResponseMsg{Action: PermissionAllowForSession, Permission: p.permission}),
			)
		case key.Matches(msg, p.keyMap.Deny):
			return p, tea.Batch(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(PermissionResponseMsg{Action: PermissionDeny, Permission: p.permission}),
			)
		case key.Matches(msg, p.keyMap.ToggleDiffMode):
			if p.supportsDiffView() {
				p.diffSplitMode = !p.diffSplitMode
				p.invalidateDiffCache() // Invalidate diff cache when mode changes
				return p, nil
			}
		case key.Matches(msg, p.keyMap.ScrollDown):
			if p.supportsDiffView() {
				p.diffYOffset += 1
				p.contentDirty = true
				return p, nil
			}
		case key.Matches(msg, p.keyMap.ScrollUp):
			if p.supportsDiffView() {
				p.diffYOffset = max(0, p.diffYOffset-1)
				p.contentDirty = true
				return p, nil
			}
		case key.Matches(msg, p.keyMap.ScrollLeft):
			if p.supportsDiffView() {
				p.diffXOffset = max(0, p.diffXOffset-5)
				p.contentDirty = true
				return p, nil
			}
		case key.Matches(msg, p.keyMap.ScrollRight):
			if p.supportsDiffView() {
				p.diffXOffset += 5
				p.contentDirty = true
				return p, nil
			}
		default:
			// Pass other keys to viewport
			viewPort, cmd := p.contentViewPort.Update(msg)
			p.contentViewPort = viewPort
			cmds = append(cmds, cmd)
		}
	}

	// Check if we need to start async diff computation
	if needsAsync, key := p.needsAsyncDiffComputation(); needsAsync {
		cmds = append(cmds, p.computeDiffAsync(key))
	}

	return p, tea.Batch(cmds...)
}

func (p *permissionDialogCmp) selectCurrentOption() tea.Cmd {
	var action PermissionAction

	switch p.selectedOption {
	case 0:
		action = PermissionAllow
	case 1:
		action = PermissionAllowForSession
	case 2:
		action = PermissionDeny
	}

	return tea.Batch(
		util.CmdHandler(PermissionResponseMsg{Action: action, Permission: p.permission}),
		util.CmdHandler(dialogs.CloseDialogMsg{}),
	)
}

func (p *permissionDialogCmp) renderButtons() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base

	buttons := []core.ButtonOpts{
		{
			Text:           "Allow",
			UnderlineIndex: 0, // "A"
			Selected:       p.selectedOption == 0,
		},
		{
			Text:           "Allow for Session",
			UnderlineIndex: 10, // "S" in "Session"
			Selected:       p.selectedOption == 1,
		},
		{
			Text:           "Deny",
			UnderlineIndex: 0, // "D"
			Selected:       p.selectedOption == 2,
		},
	}

	content := core.SelectableButtons(buttons, "  ")

	return baseStyle.AlignHorizontal(lipgloss.Right).Width(p.width - 4).Render(content)
}

func (p *permissionDialogCmp) renderHeader() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base

	toolKey := t.S().Muted.Render("Tool")
	toolValue := t.S().Text.
		Width(p.width - lipgloss.Width(toolKey)).
		Render(fmt.Sprintf(" %s", p.permission.ToolName))

	pathKey := t.S().Muted.Render("Path")
	pathValue := t.S().Text.
		Width(p.width - lipgloss.Width(pathKey)).
		Render(fmt.Sprintf(" %s", fsext.PrettyPath(p.permission.Path)))

	headerParts := []string{
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			toolKey,
			toolValue,
		),
		baseStyle.Render(strings.Repeat(" ", p.width)),
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			pathKey,
			pathValue,
		),
		baseStyle.Render(strings.Repeat(" ", p.width)),
	}

	// Add tool-specific header information
	switch p.permission.ToolName {
	case tools.BashToolName:
		headerParts = append(headerParts, t.S().Muted.Width(p.width).Render("Command"))
	case tools.EditToolName:
		params := p.permission.Params.(tools.EditPermissionsParams)
		fileKey := t.S().Muted.Render("File")
		filePath := t.S().Text.
			Width(p.width - lipgloss.Width(fileKey)).
			Render(fmt.Sprintf(" %s", fsext.PrettyPath(params.FilePath)))
		headerParts = append(headerParts,
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				fileKey,
				filePath,
			),
			baseStyle.Render(strings.Repeat(" ", p.width)),
		)

	case tools.WriteToolName:
		params := p.permission.Params.(tools.WritePermissionsParams)
		fileKey := t.S().Muted.Render("File")
		filePath := t.S().Text.
			Width(p.width - lipgloss.Width(fileKey)).
			Render(fmt.Sprintf(" %s", fsext.PrettyPath(params.FilePath)))
		headerParts = append(headerParts,
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				fileKey,
				filePath,
			),
			baseStyle.Render(strings.Repeat(" ", p.width)),
		)
	case tools.FetchToolName:
		headerParts = append(headerParts, t.S().Muted.Width(p.width).Bold(true).Render("URL"))
	}

	return baseStyle.Render(lipgloss.JoinVertical(lipgloss.Left, headerParts...))
}

func (p *permissionDialogCmp) getOrGenerateContent() string {
	// Return cached content if available and not dirty
	if !p.shouldRegenerateContent() && p.cachedContent != "" {
		return p.cachedContent
	}

	// Generate new content
	var content string
	switch p.permission.ToolName {
	case tools.BashToolName:
		content = p.generateBashContent()
	case tools.EditToolName:
		content = p.generateEditContent()
	case tools.WriteToolName:
		content = p.generateWriteContent()
	case tools.FetchToolName:
		content = p.generateFetchContent()
	default:
		content = p.generateDefaultContent()
	}

	// Cache the result and update viewport tracking
	p.cachedContent = content
	p.contentDirty = false
	p.lastViewportHeight = p.contentViewPort.Height()
	p.lastViewportWidth = p.contentViewPort.Width()

	return content
}

func (p *permissionDialogCmp) generateBashContent() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base.Background(t.BgSubtle)
	if pr, ok := p.permission.Params.(tools.BashPermissionsParams); ok {
		content := pr.Command
		t := styles.CurrentTheme()
		content = strings.TrimSpace(content)
		content = "\n" + content + "\n"
		lines := strings.Split(content, "\n")

		width := p.width - 4
		var out []string
		for _, ln := range lines {
			ln = " " + ln // left padding
			if len(ln) > width {
				ln = ansi.Truncate(ln, width, "…")
			}
			out = append(out, t.S().Muted.
				Width(width).
				Foreground(t.FgBase).
				Background(t.BgSubtle).
				Render(ln))
		}

		// Use the cache for markdown rendering
		renderedContent := strings.Join(out, "\n")
		finalContent := baseStyle.
			Width(p.contentViewPort.Width()).
			Render(renderedContent)

		return finalContent
	}
	return ""
}

func (p *permissionDialogCmp) generateEditContent() string {
	if pr, ok := p.permission.Params.(tools.EditPermissionsParams); ok {
		return p.generateDiffContent(pr.FilePath, pr.OldContent, pr.NewContent)
	}
	return ""
}

func (p *permissionDialogCmp) generateWriteContent() string {
	if pr, ok := p.permission.Params.(tools.WritePermissionsParams); ok {
		return p.generateDiffContent(pr.FilePath, pr.OldContent, pr.NewContent)
	}
	return ""
}

// generateDiffContent uses caching and async computation to avoid blocking UI
func (p *permissionDialogCmp) generateDiffContent(filePath, oldContent, newContent string) string {
	// Create cache key for current parameters
	currentKey := diffCacheKey{
		filePath:   filePath,
		oldContent: oldContent,
		newContent: newContent,
		splitMode:  p.diffSplitMode,
	}

	// Check if we can reuse cached diff
	if p.cachedDiff != nil && p.lastDiffParams.Equals(currentKey) {
		// Update only the viewport parameters and offsets
		diff := p.cachedDiff.
			Height(p.contentViewPort.Height()).
			Width(p.contentViewPort.Width()).
			XOffset(p.diffXOffset).
			YOffset(p.diffYOffset)
		result := diff.String()
		// Reset Y offset if result is empty (likely scrolled past content)
		if strings.TrimSpace(result) == "" && p.diffYOffset > 0 {
			p.diffYOffset = 0
			diff = p.cachedDiff.
				Height(p.contentViewPort.Height()).
				Width(p.contentViewPort.Width()).
				XOffset(p.diffXOffset).
				YOffset(0)
			result = diff.String()
		}
		return result
	}

	// If we're already computing this diff, show loading message
	if p.isComputingDiff && p.pendingDiffKey.Equals(currentKey) {
		return p.generateLoadingContent()
	}

	// For small files, compute synchronously
	const maxAsyncSize = 10000 // 10KB threshold for async processing
	if len(oldContent) < maxAsyncSize && len(newContent) < maxAsyncSize {
		return p.generateDiffSync(filePath, oldContent, newContent, currentKey)
	}

	// For large files, show loading and mark for async computation
	if !p.isComputingDiff {
		p.isComputingDiff = true
		p.pendingDiffKey = currentKey
		// The async computation will be triggered in the next Update cycle
		return p.generateLoadingContent()
	}

	// If we're computing a different diff than what's needed, cancel and restart
	if !p.pendingDiffKey.Equals(currentKey) {
		p.isComputingDiff = true
		p.pendingDiffKey = currentKey
	}

	return p.generateLoadingContent()
}

// needsAsyncDiffComputation checks if we need to start async diff computation
func (p *permissionDialogCmp) needsAsyncDiffComputation() (bool, diffCacheKey) {
	if !p.isComputingDiff {
		return false, diffCacheKey{}
	}

	// Check if we have a pending diff computation
	if p.pendingDiffKey.filePath != "" {
		return true, p.pendingDiffKey
	}

	return false, diffCacheKey{}
}

// generateDiffSync performs synchronous diff generation for small files
func (p *permissionDialogCmp) generateDiffSync(filePath, oldContent, newContent string, key diffCacheKey) string {
	formatter := core.DiffFormatter().
		Before(fsext.PrettyPath(filePath), oldContent).
		After(fsext.PrettyPath(filePath), newContent).
		Height(p.contentViewPort.Height()).
		Width(p.contentViewPort.Width()).
		XOffset(p.diffXOffset).
		YOffset(p.diffYOffset)

	if p.diffSplitMode {
		formatter = formatter.Split()
	} else {
		formatter = formatter.Unified()
	}

	// Cache the formatter and parameters
	p.cachedDiff = formatter
	p.lastDiffParams = key

	return formatter.String()
}

// generateLoadingContent shows a loading message while diff is being computed
func (p *permissionDialogCmp) generateLoadingContent() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base.Background(t.BgSubtle)

	content := "Computing diff..."
	return baseStyle.
		Width(p.contentViewPort.Width()).
		Height(p.contentViewPort.Height()).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)
}

// computeDiffAsync returns a command that computes the diff asynchronously
func (p *permissionDialogCmp) computeDiffAsync(key diffCacheKey) tea.Cmd {
	// Capture current viewport dimensions and offsets at command creation time
	height := p.contentViewPort.Height()
	width := p.contentViewPort.Width()
	xOffset := p.diffXOffset
	yOffset := p.diffYOffset

	return func() tea.Msg {
		// Use captured values instead of accessing struct fields
		formatter := core.DiffFormatter().
			Before(fsext.PrettyPath(key.filePath), key.oldContent).
			After(fsext.PrettyPath(key.filePath), key.newContent).
			Height(height).
			Width(width).
			XOffset(xOffset).
			YOffset(yOffset)

		if key.splitMode {
			formatter = formatter.Split()
		} else {
			formatter = formatter.Unified()
		}

		return diffResultMsg{
			key:    key,
			result: formatter,
			err:    nil,
		}
	}
}

func (p *permissionDialogCmp) generateFetchContent() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base.Background(t.BgSubtle)
	if pr, ok := p.permission.Params.(tools.FetchPermissionsParams); ok {
		content := fmt.Sprintf("```bash\n%s\n```", pr.URL)

		// Use the cache for markdown rendering
		renderedContent := p.GetOrSetMarkdown(p.permission.ID, func() (string, error) {
			r := styles.GetMarkdownRenderer(p.width - 4)
			s, err := r.Render(content)
			return s, err
		})

		finalContent := baseStyle.
			Width(p.contentViewPort.Width()).
			Render(renderedContent)

		return finalContent
	}
	return ""
}

func (p *permissionDialogCmp) generateDefaultContent() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base.Background(t.BgSubtle)

	content := p.permission.Description

	// Use the cache for markdown rendering
	renderedContent := p.GetOrSetMarkdown(p.permission.ID, func() (string, error) {
		r := styles.GetMarkdownRenderer(p.width - 4)
		s, err := r.Render(content)
		return s, err
	})

	finalContent := baseStyle.
		Width(p.contentViewPort.Width()).
		Render(renderedContent)

	if renderedContent == "" {
		return ""
	}

	return finalContent
}

func (p *permissionDialogCmp) styleViewport() string {
	t := styles.CurrentTheme()
	return t.S().Base.Render(p.contentViewPort.View())
}

func (p *permissionDialogCmp) render() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base
	title := core.Title("Permission Required", p.width-4)
	// Render header
	headerContent := p.renderHeader()
	// Render buttons
	buttons := p.renderButtons()

	p.contentViewPort.SetWidth(p.width - 4)

	// Get cached or generate content
	contentFinal := p.getOrGenerateContent()

	// Always set viewport content (the caching is handled in getOrGenerateContent)
	contentHeight := min(p.height-9, lipgloss.Height(contentFinal))
	p.contentViewPort.SetHeight(contentHeight)
	p.contentViewPort.SetContent(contentFinal)

	var contentHelp string
	if p.supportsDiffView() {
		contentHelp = help.New().View(p.keyMap)
	}
	// Calculate content height dynamically based on window size

	strs := []string{
		title,
		"",
		headerContent,
		p.styleViewport(),
		"",
		buttons,
		"",
	}
	if contentHelp != "" {
		strs = append(strs, "", contentHelp)
	}
	content := lipgloss.JoinVertical(lipgloss.Top, strs...)

	return baseStyle.
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(p.width).
		Render(
			content,
		)
}

func (p *permissionDialogCmp) View() tea.View {
	return tea.NewView(p.render())
}

func (p *permissionDialogCmp) SetSize() tea.Cmd {
	if p.permission.ID == "" {
		return nil
	}

	oldWidth, oldHeight := p.width, p.height

	switch p.permission.ToolName {
	case tools.BashToolName:
		p.width = int(float64(p.wWidth) * 0.4)
		p.height = int(float64(p.wHeight) * 0.3)
	case tools.EditToolName:
		p.width = int(float64(p.wWidth) * 0.8)
		p.height = int(float64(p.wHeight) * 0.8)
	case tools.WriteToolName:
		p.width = int(float64(p.wWidth) * 0.8)
		p.height = int(float64(p.wHeight) * 0.8)
	case tools.FetchToolName:
		p.width = int(float64(p.wWidth) * 0.4)
		p.height = int(float64(p.wHeight) * 0.3)
	default:
		p.width = int(float64(p.wWidth) * 0.7)
		p.height = int(float64(p.wHeight) * 0.5)
	}

	// Mark content as dirty if size changed
	if oldWidth != p.width || oldHeight != p.height {
		p.contentDirty = true
	}

	return nil
}

func (c *permissionDialogCmp) GetOrSetMarkdown(key string, generator func() (string, error)) string {
	content, err := generator()
	if err != nil {
		return fmt.Sprintf("Error rendering markdown: %v", err)
	}

	return content
}

// ID implements PermissionDialogCmp.
func (p *permissionDialogCmp) ID() dialogs.DialogID {
	return PermissionsDialogID
}

// Position implements PermissionDialogCmp.
func (p *permissionDialogCmp) Position() (int, int) {
	row := (p.wHeight / 2) - 2 // Just a bit above the center
	row -= p.height / 2
	col := p.wWidth / 2
	col -= p.width / 2
	return row, col
}
