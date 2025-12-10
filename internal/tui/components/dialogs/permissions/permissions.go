package permissions

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/x/ansi"
)

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
	defaultDiffSplitMode bool  // true for split, false for unified
	diffSplitMode        *bool // nil means use defaultDiffSplitMode
	diffXOffset          int   // horizontal scroll offset
	diffYOffset          int   // vertical scroll offset

	// Caching
	cachedContent string
	contentDirty  bool

	positionRow int // Row position for dialog
	positionCol int // Column position for dialog

	finalDialogHeight int

	keyMap KeyMap
}

func NewPermissionDialogCmp(permission permission.PermissionRequest, opts *Options) PermissionDialogCmp {
	if opts == nil {
		opts = &Options{}
	}

	// Create viewport for content
	contentViewport := viewport.New()
	return &permissionDialogCmp{
		contentViewPort: contentViewport,
		selectedOption:  0, // Default to "Allow"
		permission:      permission,
		diffSplitMode:   opts.isSplitMode(),
		keyMap:          DefaultKeyMap(),
		contentDirty:    true, // Mark as dirty initially
	}
}

func (p *permissionDialogCmp) Init() tea.Cmd {
	return p.contentViewPort.Init()
}

func (p *permissionDialogCmp) supportsDiffView() bool {
	return p.permission.ToolName == tools.EditToolName || p.permission.ToolName == tools.WriteToolName || p.permission.ToolName == tools.MultiEditToolName
}

func (p *permissionDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.wWidth = msg.Width
		p.wHeight = msg.Height
		p.contentDirty = true // Mark content as dirty on window resize
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
				if p.diffSplitMode == nil {
					diffSplitMode := !p.defaultDiffSplitMode
					p.diffSplitMode = &diffSplitMode
				} else {
					*p.diffSplitMode = !*p.diffSplitMode
				}
				p.contentDirty = true // Mark content as dirty when diff mode changes
				return p, nil
			}
		case key.Matches(msg, p.keyMap.ScrollDown):
			if p.supportsDiffView() {
				p.scrollDown()
				return p, nil
			}
		case key.Matches(msg, p.keyMap.ScrollUp):
			if p.supportsDiffView() {
				p.scrollUp()
				return p, nil
			}
		case key.Matches(msg, p.keyMap.ScrollLeft):
			if p.supportsDiffView() {
				p.scrollLeft()
				return p, nil
			}
		case key.Matches(msg, p.keyMap.ScrollRight):
			if p.supportsDiffView() {
				p.scrollRight()
				return p, nil
			}
		default:
			// Pass other keys to viewport
			viewPort, cmd := p.contentViewPort.Update(msg)
			p.contentViewPort = viewPort
			cmds = append(cmds, cmd)
		}
	case tea.MouseWheelMsg:
		if p.supportsDiffView() && p.isMouseOverDialog(msg.Mouse().X, msg.Mouse().Y) {
			switch msg.Button {
			case tea.MouseWheelDown:
				p.scrollDown()
			case tea.MouseWheelUp:
				p.scrollUp()
			case tea.MouseWheelLeft:
				p.scrollLeft()
			case tea.MouseWheelRight:
				p.scrollRight()
			}
		}
	}

	return p, tea.Batch(cmds...)
}

func (p *permissionDialogCmp) scrollDown() {
	p.diffYOffset += 1
	p.contentDirty = true
}

func (p *permissionDialogCmp) scrollUp() {
	p.diffYOffset = max(0, p.diffYOffset-1)
	p.contentDirty = true
}

func (p *permissionDialogCmp) scrollLeft() {
	p.diffXOffset = max(0, p.diffXOffset-5)
	p.contentDirty = true
}

func (p *permissionDialogCmp) scrollRight() {
	p.diffXOffset += 5
	p.contentDirty = true
}

// isMouseOverDialog checks if the given mouse coordinates are within the dialog bounds.
// Returns true if the mouse is over the dialog area, false otherwise.
func (p *permissionDialogCmp) isMouseOverDialog(x, y int) bool {
	if p.permission.ID == "" {
		return false
	}
	var (
		dialogX      = p.positionCol
		dialogY      = p.positionRow
		dialogWidth  = p.width
		dialogHeight = p.finalDialogHeight
	)
	return x >= dialogX && x < dialogX+dialogWidth && y >= dialogY && y < dialogY+dialogHeight
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
	if lipgloss.Width(content) > p.width-4 {
		content = core.SelectableButtonsVertical(buttons, 1)
		return baseStyle.AlignVertical(lipgloss.Center).
			AlignHorizontal(lipgloss.Center).
			Width(p.width - 4).
			Render(content)
	}

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
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			pathKey,
			pathValue,
		),
	}

	// Add tool-specific header information
	switch p.permission.ToolName {
	case tools.BashToolName:
		params := p.permission.Params.(tools.BashPermissionsParams)
		descKey := t.S().Muted.Render("Desc")
		descValue := t.S().Text.
			Width(p.width - lipgloss.Width(descKey)).
			Render(fmt.Sprintf(" %s", params.Description))
		headerParts = append(headerParts,
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				descKey,
				descValue,
			),
			baseStyle.Render(strings.Repeat(" ", p.width)),
			t.S().Muted.Width(p.width).Render("Command"),
		)
	case tools.DownloadToolName:
		params := p.permission.Params.(tools.DownloadPermissionsParams)
		urlKey := t.S().Muted.Render("URL")
		urlValue := t.S().Text.
			Width(p.width - lipgloss.Width(urlKey)).
			Render(fmt.Sprintf(" %s", params.URL))
		fileKey := t.S().Muted.Render("File")
		filePath := t.S().Text.
			Width(p.width - lipgloss.Width(fileKey)).
			Render(fmt.Sprintf(" %s", fsext.PrettyPath(params.FilePath)))
		headerParts = append(headerParts,
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				urlKey,
				urlValue,
			),
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				fileKey,
				filePath,
			),
			baseStyle.Render(strings.Repeat(" ", p.width)),
		)
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
	case tools.MultiEditToolName:
		params := p.permission.Params.(tools.MultiEditPermissionsParams)
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
		headerParts = append(headerParts,
			baseStyle.Render(strings.Repeat(" ", p.width)),
			t.S().Muted.Width(p.width).Bold(true).Render("URL"),
		)
	case tools.AgenticFetchToolName:
		headerParts = append(headerParts,
			baseStyle.Render(strings.Repeat(" ", p.width)),
			t.S().Muted.Width(p.width).Bold(true).Render("Web"),
		)
	case tools.ViewToolName:
		params := p.permission.Params.(tools.ViewPermissionsParams)
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
	case tools.LSToolName:
		params := p.permission.Params.(tools.LSPermissionsParams)
		pathKey := t.S().Muted.Render("Directory")
		pathValue := t.S().Text.
			Width(p.width - lipgloss.Width(pathKey)).
			Render(fmt.Sprintf(" %s", fsext.PrettyPath(params.Path)))
		headerParts = append(headerParts,
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				pathKey,
				pathValue,
			),
			baseStyle.Render(strings.Repeat(" ", p.width)),
		)
	}

	return baseStyle.Render(lipgloss.JoinVertical(lipgloss.Left, headerParts...))
}

func (p *permissionDialogCmp) getOrGenerateContent() string {
	// Return cached content if available and not dirty
	if !p.contentDirty && p.cachedContent != "" {
		return p.cachedContent
	}

	// Generate new content
	var content string
	switch p.permission.ToolName {
	case tools.BashToolName:
		content = p.generateBashContent()
	case tools.DownloadToolName:
		content = p.generateDownloadContent()
	case tools.EditToolName:
		content = p.generateEditContent()
	case tools.WriteToolName:
		content = p.generateWriteContent()
	case tools.MultiEditToolName:
		content = p.generateMultiEditContent()
	case tools.FetchToolName:
		content = p.generateFetchContent()
	case tools.AgenticFetchToolName:
		content = p.generateAgenticFetchContent()
	case tools.ViewToolName:
		content = p.generateViewContent()
	case tools.LSToolName:
		content = p.generateLSContent()
	default:
		content = p.generateDefaultContent()
	}

	// Cache the result
	p.cachedContent = content
	p.contentDirty = false

	return content
}

func (p *permissionDialogCmp) generateBashContent() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base.Background(t.BgSubtle)
	if pr, ok := p.permission.Params.(tools.BashPermissionsParams); ok {
		content := pr.Command
		t := styles.CurrentTheme()
		content = strings.TrimSpace(content)
		lines := strings.Split(content, "\n")

		width := p.width - 4
		var out []string
		for _, ln := range lines {
			out = append(out, t.S().Muted.
				Width(width).
				Padding(0, 3).
				Foreground(t.FgBase).
				Background(t.BgSubtle).
				Render(ln))
		}

		// Ensure minimum of 7 lines for command display
		minLines := 7
		for len(out) < minLines {
			out = append(out, t.S().Muted.
				Width(width).
				Padding(0, 3).
				Foreground(t.FgBase).
				Background(t.BgSubtle).
				Render(""))
		}

		// Use the cache for markdown rendering
		renderedContent := strings.Join(out, "\n")
		finalContent := baseStyle.
			Width(p.contentViewPort.Width()).
			Padding(1, 0).
			Render(renderedContent)

		return finalContent
	}
	return ""
}

func (p *permissionDialogCmp) generateEditContent() string {
	if pr, ok := p.permission.Params.(tools.EditPermissionsParams); ok {
		formatter := core.DiffFormatter().
			Before(fsext.PrettyPath(pr.FilePath), pr.OldContent).
			After(fsext.PrettyPath(pr.FilePath), pr.NewContent).
			Height(p.contentViewPort.Height()).
			Width(p.contentViewPort.Width()).
			XOffset(p.diffXOffset).
			YOffset(p.diffYOffset)
		if p.useDiffSplitMode() {
			formatter = formatter.Split()
		} else {
			formatter = formatter.Unified()
		}

		diff := formatter.String()
		return diff
	}
	return ""
}

func (p *permissionDialogCmp) generateWriteContent() string {
	if pr, ok := p.permission.Params.(tools.WritePermissionsParams); ok {
		// Use the cache for diff rendering
		formatter := core.DiffFormatter().
			Before(fsext.PrettyPath(pr.FilePath), pr.OldContent).
			After(fsext.PrettyPath(pr.FilePath), pr.NewContent).
			Height(p.contentViewPort.Height()).
			Width(p.contentViewPort.Width()).
			XOffset(p.diffXOffset).
			YOffset(p.diffYOffset)
		if p.useDiffSplitMode() {
			formatter = formatter.Split()
		} else {
			formatter = formatter.Unified()
		}

		diff := formatter.String()
		return diff
	}
	return ""
}

func (p *permissionDialogCmp) generateDownloadContent() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base.Background(t.BgSubtle)
	if pr, ok := p.permission.Params.(tools.DownloadPermissionsParams); ok {
		content := fmt.Sprintf("URL: %s\nFile: %s", pr.URL, fsext.PrettyPath(pr.FilePath))
		if pr.Timeout > 0 {
			content += fmt.Sprintf("\nTimeout: %ds", pr.Timeout)
		}

		finalContent := baseStyle.
			Padding(1, 2).
			Width(p.contentViewPort.Width()).
			Render(content)
		return finalContent
	}
	return ""
}

func (p *permissionDialogCmp) generateMultiEditContent() string {
	if pr, ok := p.permission.Params.(tools.MultiEditPermissionsParams); ok {
		// Use the cache for diff rendering
		formatter := core.DiffFormatter().
			Before(fsext.PrettyPath(pr.FilePath), pr.OldContent).
			After(fsext.PrettyPath(pr.FilePath), pr.NewContent).
			Height(p.contentViewPort.Height()).
			Width(p.contentViewPort.Width()).
			XOffset(p.diffXOffset).
			YOffset(p.diffYOffset)
		if p.useDiffSplitMode() {
			formatter = formatter.Split()
		} else {
			formatter = formatter.Unified()
		}

		diff := formatter.String()
		return diff
	}
	return ""
}

func (p *permissionDialogCmp) generateFetchContent() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base.Background(t.BgSubtle)
	if pr, ok := p.permission.Params.(tools.FetchPermissionsParams); ok {
		finalContent := baseStyle.
			Padding(1, 2).
			Width(p.contentViewPort.Width()).
			Render(pr.URL)
		return finalContent
	}
	return ""
}

func (p *permissionDialogCmp) generateAgenticFetchContent() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base.Background(t.BgSubtle)
	if pr, ok := p.permission.Params.(tools.AgenticFetchPermissionsParams); ok {
		var content string
		if pr.URL != "" {
			content = fmt.Sprintf("URL: %s\n\nPrompt: %s", pr.URL, pr.Prompt)
		} else {
			content = fmt.Sprintf("Prompt: %s", pr.Prompt)
		}
		finalContent := baseStyle.
			Padding(1, 2).
			Width(p.contentViewPort.Width()).
			Render(content)
		return finalContent
	}
	return ""
}

func (p *permissionDialogCmp) generateViewContent() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base.Background(t.BgSubtle)
	if pr, ok := p.permission.Params.(tools.ViewPermissionsParams); ok {
		content := fmt.Sprintf("File: %s", fsext.PrettyPath(pr.FilePath))
		if pr.Offset > 0 {
			content += fmt.Sprintf("\nStarting from line: %d", pr.Offset+1)
		}
		if pr.Limit > 0 && pr.Limit != 2000 { // 2000 is the default limit
			content += fmt.Sprintf("\nLines to read: %d", pr.Limit)
		}

		finalContent := baseStyle.
			Padding(1, 2).
			Width(p.contentViewPort.Width()).
			Render(content)
		return finalContent
	}
	return ""
}

func (p *permissionDialogCmp) generateLSContent() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base.Background(t.BgSubtle)
	if pr, ok := p.permission.Params.(tools.LSPermissionsParams); ok {
		content := fmt.Sprintf("Directory: %s", fsext.PrettyPath(pr.Path))
		if len(pr.Ignore) > 0 {
			content += fmt.Sprintf("\nIgnore patterns: %s", strings.Join(pr.Ignore, ", "))
		}

		finalContent := baseStyle.
			Padding(1, 2).
			Width(p.contentViewPort.Width()).
			Render(content)
		return finalContent
	}
	return ""
}

func (p *permissionDialogCmp) generateDefaultContent() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base.Background(t.BgSubtle)

	content := p.permission.Description

	// Add pretty-printed JSON parameters for MCP tools
	if p.permission.Params != nil {
		var paramStr string

		// Ensure params is a string
		if str, ok := p.permission.Params.(string); ok {
			paramStr = str
		} else {
			paramStr = fmt.Sprintf("%v", p.permission.Params)
		}

		// Try to parse as JSON for pretty printing
		var parsed any
		if err := json.Unmarshal([]byte(paramStr), &parsed); err == nil {
			if b, err := json.MarshalIndent(parsed, "", "  "); err == nil {
				if content != "" {
					content += "\n\n"
				}
				content += string(b)
			}
		} else {
			// Not JSON, show as-is
			if content != "" {
				content += "\n\n"
			}
			content += paramStr
		}
	}

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

	if renderedContent == "" {
		return ""
	}

	return finalContent
}

func (p *permissionDialogCmp) useDiffSplitMode() bool {
	if p.diffSplitMode != nil {
		return *p.diffSplitMode
	}
	return p.defaultDiffSplitMode
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

	// Always set viewport content (the caching is handled in getOrGenerateContent)
	const minContentHeight = 9

	availableDialogHeight := max(minContentHeight, p.height-minContentHeight)
	p.contentViewPort.SetHeight(availableDialogHeight)
	contentFinal := p.getOrGenerateContent()
	contentHeight := min(availableDialogHeight, lipgloss.Height(contentFinal))

	p.contentViewPort.SetHeight(contentHeight)
	p.contentViewPort.SetContent(contentFinal)

	p.positionRow = p.wHeight / 2
	p.positionRow -= (contentHeight + 9) / 2
	p.positionRow -= 3 // Move dialog slightly higher than middle

	var contentHelp string
	if p.supportsDiffView() {
		contentHelp = help.New().View(p.keyMap)
	}

	// Calculate content height dynamically based on window size
	strs := []string{
		title,
		"",
		headerContent,
		"",
		p.styleViewport(),
		"",
		buttons,
		"",
	}
	if contentHelp != "" {
		strs = append(strs, "", contentHelp)
	}
	content := lipgloss.JoinVertical(lipgloss.Top, strs...)

	dialog := baseStyle.
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(p.width).
		Render(
			content,
		)
	p.finalDialogHeight = lipgloss.Height(dialog)
	return dialog
}

func (p *permissionDialogCmp) View() string {
	return p.render()
}

func (p *permissionDialogCmp) SetSize() tea.Cmd {
	if p.permission.ID == "" {
		return nil
	}

	oldWidth, oldHeight := p.width, p.height

	switch p.permission.ToolName {
	case tools.BashToolName:
		p.width = int(float64(p.wWidth) * 0.8)
		p.height = int(float64(p.wHeight) * 0.3)
	case tools.DownloadToolName:
		p.width = int(float64(p.wWidth) * 0.8)
		p.height = int(float64(p.wHeight) * 0.4)
	case tools.EditToolName:
		p.width = int(float64(p.wWidth) * 0.8)
		p.height = int(float64(p.wHeight) * 0.8)
	case tools.WriteToolName:
		p.width = int(float64(p.wWidth) * 0.8)
		p.height = int(float64(p.wHeight) * 0.8)
	case tools.MultiEditToolName:
		p.width = int(float64(p.wWidth) * 0.8)
		p.height = int(float64(p.wHeight) * 0.8)
	case tools.FetchToolName:
		p.width = int(float64(p.wWidth) * 0.8)
		p.height = int(float64(p.wHeight) * 0.3)
	case tools.AgenticFetchToolName:
		p.width = int(float64(p.wWidth) * 0.8)
		p.height = int(float64(p.wHeight) * 0.4)
	case tools.ViewToolName:
		p.width = int(float64(p.wWidth) * 0.8)
		p.height = int(float64(p.wHeight) * 0.4)
	case tools.LSToolName:
		p.width = int(float64(p.wWidth) * 0.8)
		p.height = int(float64(p.wHeight) * 0.4)
	default:
		p.width = int(float64(p.wWidth) * 0.7)
		p.height = int(float64(p.wHeight) * 0.5)
	}

	// Default to diff split mode when dialog is wide enough.
	p.defaultDiffSplitMode = p.width >= 140

	// Set a maximum width for the dialog
	p.width = min(p.width, 180)

	// Mark content as dirty if size changed
	if oldWidth != p.width || oldHeight != p.height {
		p.contentDirty = true
	}
	p.positionRow = p.wHeight / 2
	p.positionRow -= p.height / 2
	p.positionRow -= 3 // Move dialog slightly higher than middle
	p.positionCol = p.wWidth / 2
	p.positionCol -= p.width / 2
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
	return p.positionRow, p.positionCol
}

// Options for create a new permission dialog
type Options struct {
	DiffMode string // split or unified, empty means use defaultDiffSplitMode
}

// isSplitMode returns internal representation of diff mode switch
func (o Options) isSplitMode() *bool {
	var split bool

	switch o.DiffMode {
	case "split":
		split = true
	case "unified":
		split = false
	default:
		return nil
	}

	return &split
}
