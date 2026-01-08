package dialog

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
	"github.com/charmbracelet/crush/internal/ui/common"
)

// PermissionsID is the identifier for the permissions dialog.
const PermissionsID = "permissions"

// PermissionAction represents the user's response to a permission request.
type PermissionAction string

const (
	PermissionAllow           PermissionAction = "allow"
	PermissionAllowForSession PermissionAction = "allow_session"
	PermissionDeny            PermissionAction = "deny"
)

// PermissionResponseMsg is sent when the user responds to a permission request.
type PermissionResponseMsg struct {
	Permission permission.PermissionRequest
	Action     PermissionAction
}

// Permissions represents a dialog for permission requests.
type Permissions struct {
	com           *common.Common
	width, height int
	maxHeight     int
	windowWidth   int // Terminal window dimensions.
	windowHeight  int
	fullscreen    bool // true when dialog is fullscreen

	permission     permission.PermissionRequest
	selectedOption int // 0: Allow, 1: Allow for session, 2: Deny

	// Content viewport for scrollable content.
	viewport viewport.Model

	// Diff view state.
	diffSplitMode        *bool // nil means use default based on width
	defaultDiffSplitMode bool  // true for split when width >= 140
	diffXOffset          int
	diffYOffset          int

	help   help.Model
	keyMap permissionsKeyMap
}

type permissionsKeyMap struct {
	Left             key.Binding
	Right            key.Binding
	Tab              key.Binding
	Select           key.Binding
	Allow            key.Binding
	AllowSession     key.Binding
	Deny             key.Binding
	Close            key.Binding
	ToggleDiffMode   key.Binding
	ToggleFullscreen key.Binding
	ScrollUp         key.Binding
	ScrollDown       key.Binding
	ScrollLeft       key.Binding
	ScrollRight      key.Binding
}

func defaultPermissionsKeyMap() permissionsKeyMap {
	return permissionsKeyMap{
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←", "previous"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→", "next"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next option"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", "ctrl+y"),
			key.WithHelp("enter", "confirm"),
		),
		Allow: key.NewBinding(
			key.WithKeys("a", "A", "ctrl+a"),
			key.WithHelp("a", "allow"),
		),
		AllowSession: key.NewBinding(
			key.WithKeys("s", "S", "ctrl+s"),
			key.WithHelp("s", "allow session"),
		),
		Deny: key.NewBinding(
			key.WithKeys("d", "D"),
			key.WithHelp("d", "deny"),
		),
		Close: CloseKey,
		ToggleDiffMode: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "toggle diff view"),
		),
		ToggleFullscreen: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "toggle fullscreen"),
		),
		ScrollUp: key.NewBinding(
			key.WithKeys("shift+up", "K"),
			key.WithHelp("shift+↑", "scroll up"),
		),
		ScrollDown: key.NewBinding(
			key.WithKeys("shift+down", "J"),
			key.WithHelp("shift+↓", "scroll down"),
		),
		ScrollLeft: key.NewBinding(
			key.WithKeys("shift+left", "H"),
			key.WithHelp("shift+←", "scroll left"),
		),
		ScrollRight: key.NewBinding(
			key.WithKeys("shift+right", "L"),
			key.WithHelp("shift+→", "scroll right"),
		),
	}
}

var _ Dialog = (*Permissions)(nil)

// PermissionsOption configures the permissions dialog.
type PermissionsOption func(*Permissions)

// WithDiffMode sets the initial diff mode (split or unified).
func WithDiffMode(split bool) PermissionsOption {
	return func(p *Permissions) {
		p.diffSplitMode = &split
	}
}

// NewPermissions creates a new permissions dialog.
func NewPermissions(com *common.Common, perm permission.PermissionRequest, opts ...PermissionsOption) *Permissions {
	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()

	p := &Permissions{
		com:            com,
		permission:     perm,
		selectedOption: 0,
		viewport:       viewport.New(),
		help:           h,
		keyMap:         defaultPermissionsKeyMap(),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// SetWindowSize implements [Dialog].
func (p *Permissions) SetWindowSize(windowWidth, windowHeight int) {
	p.windowWidth = windowWidth
	p.windowHeight = windowHeight

	// Calculate dialog dimensions based on fullscreen state and content type.
	var width, height int
	if p.fullscreen && p.hasDiffView() {
		// Use nearly full window for fullscreen.
		width = windowWidth - 2
		height = windowHeight
	} else if p.hasDiffView() {
		// Wide for side-by-side diffs, capped for readability.
		width = min(int(float64(windowWidth)*0.8), 180)
		height = int(float64(windowHeight) * 0.8)
	} else {
		// Narrower for simple content like commands/URLs.
		width = min(int(float64(windowWidth)*0.6), 100)
		height = int(float64(windowHeight) * 0.5)
	}

	p.width = width
	p.maxHeight = height

	// Default to split mode when dialog is wide enough.
	p.defaultDiffSplitMode = width >= 140

	// Update viewport width.
	p.viewport.SetWidth(p.calculateContentWidth())
}

// Calculate usable content width (dialog border + horizontal padding).
func (p *Permissions) calculateContentWidth() int {
	t := p.com.Styles
	const dialogHorizontalPadding = 2
	return p.width - t.Dialog.View.GetHorizontalFrameSize() - dialogHorizontalPadding
}

// ID implements [Dialog].
func (*Permissions) ID() string {
	return PermissionsID
}

// Update implements [Dialog].
func (p *Permissions) Update(msg tea.Msg) tea.Msg {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, p.keyMap.Close):
			// Escape denies the permission request.
			return p.respond(PermissionDeny)
		case key.Matches(msg, p.keyMap.Right), key.Matches(msg, p.keyMap.Tab):
			p.selectedOption = (p.selectedOption + 1) % 3
		case key.Matches(msg, p.keyMap.Left):
			p.selectedOption = (p.selectedOption + 2) % 3
		case key.Matches(msg, p.keyMap.Select):
			return p.selectCurrentOption()
		case key.Matches(msg, p.keyMap.Allow):
			return p.respond(PermissionAllow)
		case key.Matches(msg, p.keyMap.AllowSession):
			return p.respond(PermissionAllowForSession)
		case key.Matches(msg, p.keyMap.Deny):
			return p.respond(PermissionDeny)
		case key.Matches(msg, p.keyMap.ToggleDiffMode):
			if p.hasDiffView() {
				newMode := !p.isSplitMode()
				p.diffSplitMode = &newMode
			}
		case key.Matches(msg, p.keyMap.ToggleFullscreen):
			if p.hasDiffView() {
				p.fullscreen = !p.fullscreen
				p.SetWindowSize(p.windowWidth, p.windowHeight)
			}
		case key.Matches(msg, p.keyMap.ScrollDown):
			p.diffYOffset++
		case key.Matches(msg, p.keyMap.ScrollUp):
			p.diffYOffset = max(0, p.diffYOffset-1)
		case key.Matches(msg, p.keyMap.ScrollLeft):
			p.diffXOffset = max(0, p.diffXOffset-5)
		case key.Matches(msg, p.keyMap.ScrollRight):
			p.diffXOffset += 5
		}
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelDown:
			p.diffYOffset++
		case tea.MouseWheelUp:
			p.diffYOffset = max(0, p.diffYOffset-1)
		case tea.MouseWheelLeft:
			p.diffXOffset = max(0, p.diffXOffset-5)
		case tea.MouseWheelRight:
			p.diffXOffset += 5
		}
	}

	return nil
}

func (p *Permissions) selectCurrentOption() tea.Msg {
	switch p.selectedOption {
	case 0:
		return p.respond(PermissionAllow)
	case 1:
		return p.respond(PermissionAllowForSession)
	default:
		return p.respond(PermissionDeny)
	}
}

func (p *Permissions) respond(action PermissionAction) tea.Msg {
	return PermissionResponseMsg{
		Permission: p.permission,
		Action:     action,
	}
}

func (p *Permissions) hasDiffView() bool {
	switch p.permission.ToolName {
	case tools.EditToolName, tools.WriteToolName, tools.MultiEditToolName:
		return true
	}
	return false
}

func (p *Permissions) isSplitMode() bool {
	if p.diffSplitMode != nil {
		return *p.diffSplitMode
	}
	return p.defaultDiffSplitMode
}

// View implements [Dialog].
func (p *Permissions) View() string {
	t := p.com.Styles
	dialogStyle := t.Dialog.View.Width(p.width).Padding(0, 1)

	contentWidth := p.calculateContentWidth()
	header := p.renderHeader(contentWidth)
	buttons := p.renderButtons(contentWidth)
	helpView := p.help.View(p)

	// Calculate available height for content.
	headerHeight := lipgloss.Height(header)
	buttonsHeight := lipgloss.Height(buttons)
	helpHeight := lipgloss.Height(helpView)
	frameHeight := dialogStyle.GetVerticalFrameSize() + 4 // spacing
	availableHeight := p.maxHeight - headerHeight - buttonsHeight - helpHeight - frameHeight

	content := p.renderContent(contentWidth, availableHeight)

	parts := []string{header}
	if content != "" {
		parts = append(parts, "", content)
	}
	parts = append(parts, "", buttons, "", helpView)

	innerContent := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return dialogStyle.Render(innerContent)
}

func (p *Permissions) renderHeader(contentWidth int) string {
	t := p.com.Styles

	title := common.DialogTitle(t, "Permission Required", contentWidth-t.Dialog.Title.GetHorizontalFrameSize())
	title = t.Dialog.Title.Render(title)

	// Tool info.
	toolLine := p.renderKeyValue("Tool", p.permission.ToolName, contentWidth)
	pathLine := p.renderKeyValue("Path", fsext.PrettyPath(p.permission.Path), contentWidth)

	lines := []string{title, "", toolLine, pathLine}

	// Add tool-specific header info.
	switch p.permission.ToolName {
	case tools.BashToolName:
		if params, ok := p.permission.Params.(tools.BashPermissionsParams); ok {
			lines = append(lines, p.renderKeyValue("Desc", params.Description, contentWidth))
		}
	case tools.DownloadToolName:
		if params, ok := p.permission.Params.(tools.DownloadPermissionsParams); ok {
			lines = append(lines, p.renderKeyValue("URL", params.URL, contentWidth))
			lines = append(lines, p.renderKeyValue("File", fsext.PrettyPath(params.FilePath), contentWidth))
		}
	case tools.EditToolName, tools.WriteToolName, tools.MultiEditToolName, tools.ViewToolName:
		var filePath string
		switch params := p.permission.Params.(type) {
		case tools.EditPermissionsParams:
			filePath = params.FilePath
		case tools.WritePermissionsParams:
			filePath = params.FilePath
		case tools.MultiEditPermissionsParams:
			filePath = params.FilePath
		case tools.ViewPermissionsParams:
			filePath = params.FilePath
		}
		if filePath != "" {
			lines = append(lines, p.renderKeyValue("File", fsext.PrettyPath(filePath), contentWidth))
		}
	case tools.LSToolName:
		if params, ok := p.permission.Params.(tools.LSPermissionsParams); ok {
			lines = append(lines, p.renderKeyValue("Directory", fsext.PrettyPath(params.Path), contentWidth))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (p *Permissions) renderKeyValue(key, value string, width int) string {
	t := p.com.Styles
	keyStyle := t.Muted
	valueStyle := t.Base

	keyStr := keyStyle.Render(key)
	valueStr := valueStyle.Width(width - lipgloss.Width(keyStr) - 1).Render(" " + value)

	return lipgloss.JoinHorizontal(lipgloss.Left, keyStr, valueStr)
}

func (p *Permissions) renderContent(width, height int) string {
	switch p.permission.ToolName {
	case tools.BashToolName:
		return p.renderBashContent(width)
	case tools.EditToolName:
		return p.renderEditContent(width, height)
	case tools.WriteToolName:
		return p.renderWriteContent(width, height)
	case tools.MultiEditToolName:
		return p.renderMultiEditContent(width, height)
	case tools.DownloadToolName:
		return p.renderDownloadContent(width)
	case tools.FetchToolName:
		return p.renderFetchContent(width)
	case tools.AgenticFetchToolName:
		return p.renderAgenticFetchContent(width)
	case tools.ViewToolName:
		return p.renderViewContent(width)
	case tools.LSToolName:
		return p.renderLSContent(width)
	default:
		return p.renderDefaultContent(width)
	}
}

func (p *Permissions) renderBashContent(contentWidth int) string {
	params, ok := p.permission.Params.(tools.BashPermissionsParams)
	if !ok {
		return ""
	}

	return p.com.Styles.Dialog.ContentPanel.Width(contentWidth).Render(params.Command)
}

func (p *Permissions) renderEditContent(contentWidth, contentHeight int) string {
	params, ok := p.permission.Params.(tools.EditPermissionsParams)
	if !ok {
		return ""
	}
	return p.renderDiff(params.FilePath, params.OldContent, params.NewContent, contentWidth, contentHeight)
}

func (p *Permissions) renderWriteContent(contentWidth, contentHeight int) string {
	params, ok := p.permission.Params.(tools.WritePermissionsParams)
	if !ok {
		return ""
	}
	return p.renderDiff(params.FilePath, params.OldContent, params.NewContent, contentWidth, contentHeight)
}

func (p *Permissions) renderMultiEditContent(contentWidth, contentHeight int) string {
	params, ok := p.permission.Params.(tools.MultiEditPermissionsParams)
	if !ok {
		return ""
	}
	return p.renderDiff(params.FilePath, params.OldContent, params.NewContent, contentWidth, contentHeight)
}

func (p *Permissions) renderDiff(filePath, oldContent, newContent string, contentWidth, contentHeight int) string {
	formatter := common.DiffFormatter(p.com.Styles).
		Before(fsext.PrettyPath(filePath), oldContent).
		After(fsext.PrettyPath(filePath), newContent).
		Height(contentHeight).
		Width(contentWidth).
		XOffset(p.diffXOffset).
		YOffset(p.diffYOffset)

	if p.isSplitMode() {
		formatter = formatter.Split()
	} else {
		formatter = formatter.Unified()
	}
	// in full screen we want it to take the full space
	if p.fullscreen {
		return lipgloss.NewStyle().Width(contentWidth).Height(contentHeight).Render(formatter.String())
	}
	return formatter.String()
}

func (p *Permissions) renderDownloadContent(contentWidth int) string {
	params, ok := p.permission.Params.(tools.DownloadPermissionsParams)
	if !ok {
		return ""
	}

	content := fmt.Sprintf("URL: %s\nFile: %s", params.URL, fsext.PrettyPath(params.FilePath))
	if params.Timeout > 0 {
		content += fmt.Sprintf("\nTimeout: %ds", params.Timeout)
	}

	return p.com.Styles.Dialog.ContentPanel.Width(contentWidth).Render(content)
}

func (p *Permissions) renderFetchContent(contentWidth int) string {
	params, ok := p.permission.Params.(tools.FetchPermissionsParams)
	if !ok {
		return ""
	}

	return p.com.Styles.Dialog.ContentPanel.Width(contentWidth).Render(params.URL)
}

func (p *Permissions) renderAgenticFetchContent(contentWidth int) string {
	params, ok := p.permission.Params.(tools.AgenticFetchPermissionsParams)
	if !ok {
		return ""
	}

	var content string
	if params.URL != "" {
		content = fmt.Sprintf("URL: %s\n\nPrompt: %s", params.URL, params.Prompt)
	} else {
		content = fmt.Sprintf("Prompt: %s", params.Prompt)
	}

	return p.com.Styles.Dialog.ContentPanel.Width(contentWidth).Render(content)
}

func (p *Permissions) renderViewContent(contentWidth int) string {
	params, ok := p.permission.Params.(tools.ViewPermissionsParams)
	if !ok {
		return ""
	}

	content := fmt.Sprintf("File: %s", fsext.PrettyPath(params.FilePath))
	if params.Offset > 0 {
		content += fmt.Sprintf("\nStarting from line: %d", params.Offset+1)
	}
	if params.Limit > 0 && params.Limit != 2000 {
		content += fmt.Sprintf("\nLines to read: %d", params.Limit)
	}

	return p.com.Styles.Dialog.ContentPanel.Width(contentWidth).Render(content)
}

func (p *Permissions) renderLSContent(contentWidth int) string {
	params, ok := p.permission.Params.(tools.LSPermissionsParams)
	if !ok {
		return ""
	}

	content := fmt.Sprintf("Directory: %s", fsext.PrettyPath(params.Path))
	if len(params.Ignore) > 0 {
		content += fmt.Sprintf("\nIgnore patterns: %s", strings.Join(params.Ignore, ", "))
	}

	return p.com.Styles.Dialog.ContentPanel.Width(contentWidth).Render(content)
}

func (p *Permissions) renderDefaultContent(contentWidth int) string {
	content := p.permission.Description

	// Pretty-print JSON params if available.
	if p.permission.Params != nil {
		var paramStr string
		if str, ok := p.permission.Params.(string); ok {
			paramStr = str
		} else {
			paramStr = fmt.Sprintf("%v", p.permission.Params)
		}

		var parsed any
		if err := json.Unmarshal([]byte(paramStr), &parsed); err == nil {
			if b, err := json.MarshalIndent(parsed, "", "  "); err == nil {
				if content != "" {
					content += "\n\n"
				}
				content += string(b)
			}
		} else if paramStr != "" {
			if content != "" {
				content += "\n\n"
			}
			content += paramStr
		}
	}

	if content == "" {
		return ""
	}

	return p.com.Styles.Dialog.ContentPanel.Width(contentWidth).Render(strings.TrimSpace(content))
}

func (p *Permissions) renderButtons(contentWidth int) string {
	buttons := []common.ButtonOpts{
		{Text: "Allow", UnderlineIndex: 0, Selected: p.selectedOption == 0},
		{Text: "Allow for Session", UnderlineIndex: 10, Selected: p.selectedOption == 1},
		{Text: "Deny", UnderlineIndex: 0, Selected: p.selectedOption == 2},
	}

	content := common.ButtonGroup(p.com.Styles, buttons, "  ")

	// If buttons are too wide, stack them vertically.
	if lipgloss.Width(content) > contentWidth {
		content = common.ButtonGroup(p.com.Styles, buttons, "\n")
		return lipgloss.NewStyle().
			Width(contentWidth).
			Align(lipgloss.Center).
			Render(content)
	}

	return lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Right).
		Render(content)
}

// ShortHelp implements [help.KeyMap].
func (p *Permissions) ShortHelp() []key.Binding {
	bindings := []key.Binding{
		key.NewBinding(key.WithKeys("←/→"), key.WithHelp("←/→", "choose")),
		p.keyMap.Select,
		p.keyMap.Close,
	}

	if p.hasDiffView() {
		bindings = append(bindings,
			p.keyMap.ToggleDiffMode,
			p.keyMap.ToggleFullscreen,
			key.NewBinding(
				key.WithKeys("shift+←↓↑→"),
				key.WithHelp("shift+←↓↑→", "scroll"),
			),
		)
	}

	return bindings
}

// FullHelp implements [help.KeyMap].
func (p *Permissions) FullHelp() [][]key.Binding {
	return [][]key.Binding{p.ShortHelp()}
}
