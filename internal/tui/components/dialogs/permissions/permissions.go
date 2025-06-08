package permissions

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/fileutil"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
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
	}
}

func (p *permissionDialogCmp) Init() tea.Cmd {
	return p.contentViewPort.Init()
}

func (p *permissionDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.wWidth = msg.Width
		p.wHeight = msg.Height
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
		default:
			// Pass other keys to viewport
			viewPort, cmd := p.contentViewPort.Update(msg)
			p.contentViewPort = viewPort
			cmds = append(cmds, cmd)
		}
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

	allowStyle := t.S().Text
	allowSessionStyle := allowStyle
	denyStyle := allowStyle

	// Style the selected button
	switch p.selectedOption {
	case 0:
		allowStyle = allowStyle.Foreground(t.White).Background(t.Secondary)
		allowSessionStyle = allowSessionStyle.Background(t.BgSubtle)
		denyStyle = denyStyle.Background(t.BgSubtle)
	case 1:
		allowStyle = allowStyle.Background(t.BgSubtle)
		allowSessionStyle = allowSessionStyle.Foreground(t.White).Background(t.Secondary)
		denyStyle = denyStyle.Background(t.BgSubtle)
	case 2:
		allowStyle = allowStyle.Background(t.BgSubtle)
		allowSessionStyle = allowSessionStyle.Background(t.BgSubtle)
		denyStyle = denyStyle.Foreground(t.White).Background(t.Secondary)
	}

	baseStyle := t.S().Base

	allowMessage := fmt.Sprintf("%s%s", allowStyle.Underline(true).Render("A"), allowStyle.Render("llow"))
	allowButton := allowStyle.Padding(0, 2).Render(allowMessage)
	allowSessionMessage := fmt.Sprintf("%s%s%s", allowSessionStyle.Render("Allow for "), allowSessionStyle.Underline(true).Render("S"), allowSessionStyle.Render("ession"))
	allowSessionButton := allowSessionStyle.Padding(0, 2).Render(allowSessionMessage)
	denyMessage := fmt.Sprintf("%s%s", denyStyle.Underline(true).Render("D"), denyStyle.Render("eny"))
	denyButton := denyStyle.Padding(0, 2).Render(denyMessage)

	content := lipgloss.JoinHorizontal(
		lipgloss.Left,
		allowButton,
		"  ",
		allowSessionButton,
		"  ",
		denyButton,
	)

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
		Render(fmt.Sprintf(" %s", fileutil.PrettyPath(p.permission.Path)))

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
			Render(fmt.Sprintf(" %s", fileutil.PrettyPath(params.FilePath)))
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
			Render(fmt.Sprintf(" %s", fileutil.PrettyPath(params.FilePath)))
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

func (p *permissionDialogCmp) renderBashContent() string {
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

		contentHeight := min(p.height-9, lipgloss.Height(finalContent))
		p.contentViewPort.SetHeight(contentHeight)
		p.contentViewPort.SetContent(finalContent)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogCmp) renderEditContent() string {
	if pr, ok := p.permission.Params.(tools.EditPermissionsParams); ok {
		diff := p.GetOrSetDiff(p.permission.ID, func() (string, error) {
			return diff.FormatDiff(pr.Diff, diff.WithTotalWidth(p.contentViewPort.Width()))
		})

		contentHeight := min(p.height-9, lipgloss.Height(diff))
		p.contentViewPort.SetHeight(contentHeight)
		p.contentViewPort.SetContent(diff)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogCmp) renderPatchContent() string {
	if pr, ok := p.permission.Params.(tools.EditPermissionsParams); ok {
		diff := p.GetOrSetDiff(p.permission.ID, func() (string, error) {
			return diff.FormatDiff(pr.Diff, diff.WithTotalWidth(p.contentViewPort.Width()))
		})

		contentHeight := min(p.height-9, lipgloss.Height(diff))
		p.contentViewPort.SetHeight(contentHeight)
		p.contentViewPort.SetContent(diff)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogCmp) renderWriteContent() string {
	if pr, ok := p.permission.Params.(tools.WritePermissionsParams); ok {
		// Use the cache for diff rendering
		diff := p.GetOrSetDiff(p.permission.ID, func() (string, error) {
			return diff.FormatDiff(pr.Diff, diff.WithTotalWidth(p.contentViewPort.Width()))
		})

		contentHeight := min(p.height-9, lipgloss.Height(diff))
		p.contentViewPort.SetHeight(contentHeight)
		p.contentViewPort.SetContent(diff)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogCmp) renderFetchContent() string {
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

		contentHeight := min(p.height-9, lipgloss.Height(finalContent))
		p.contentViewPort.SetHeight(contentHeight)
		p.contentViewPort.SetContent(finalContent)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogCmp) renderDefaultContent() string {
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
	p.contentViewPort.SetContent(finalContent)

	if renderedContent == "" {
		return ""
	}

	return p.styleViewport()
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

	// Render content based on tool type
	var contentFinal string
	switch p.permission.ToolName {
	case tools.BashToolName:
		contentFinal = p.renderBashContent()
	case tools.EditToolName:
		contentFinal = p.renderEditContent()
	case tools.PatchToolName:
		contentFinal = p.renderPatchContent()
	case tools.WriteToolName:
		contentFinal = p.renderWriteContent()
	case tools.FetchToolName:
		contentFinal = p.renderFetchContent()
	default:
		contentFinal = p.renderDefaultContent()
	}
	// Calculate content height dynamically based on window size

	content := lipgloss.JoinVertical(
		lipgloss.Top,
		title,
		"",
		headerContent,
		contentFinal,
		"",
		buttons,
		"",
	)

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
	return nil
}

func (c *permissionDialogCmp) GetOrSetDiff(key string, generator func() (string, error)) string {
	content, err := generator()
	if err != nil {
		return fmt.Sprintf("Error formatting diff: %v", err)
	}
	return content
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
