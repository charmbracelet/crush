package dialog

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/stringext"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Permissions dialog sizing constants.
const (
	// diffMaxWidth is the maximum width for diff views.
	diffMaxWidth = 180
	// diffSizeRatio is the size ratio for diff views relative to window.
	diffSizeRatio = 0.8
	// simpleMaxWidth is the maximum width for simple content dialogs.
	simpleMaxWidth = 100
	// simpleSizeRatio is the size ratio for simple content dialogs.
	simpleSizeRatio = 0.6
	// simpleHeightRatio caps the height of simple content dialogs. They
	// shrink to fit their content, so the cap only matters for long
	// content, which should still get enough rows to read without
	// scrolling.
	simpleHeightRatio = 0.8
	// splitModeMinWidth is the minimum width to enable split diff mode.
	splitModeMinWidth = 140
	// layoutSpacingLines is the number of empty lines used for layout spacing.
	layoutSpacingLines = 4
	// minWindowWidth is the minimum window width before forcing fullscreen.
	minWindowWidth = 77
	// minWindowHeight is the minimum window height before forcing fullscreen.
	minWindowHeight = 20
)

// dialogSize returns the outer dialog width and maximum height for the
// given screen area, and whether the window is too small so the dialog
// must fill it. Diff views get more room than simple prompts.
func (p *Permissions) dialogSize(area uv.Rectangle) (width, maxHeight int, forceFullscreen bool) {
	forceFullscreen = area.Dx() <= minWindowWidth || area.Dy() <= minWindowHeight
	switch {
	case forceFullscreen || (p.fullscreen && p.hasDiffView()):
		width, maxHeight = area.Dx(), area.Dy()
	case p.hasDiffView():
		// Wide for side-by-side diffs, capped for readability.
		width = min(int(float64(area.Dx())*diffSizeRatio), diffMaxWidth)
		maxHeight = int(float64(area.Dy()) * diffSizeRatio)
	default:
		// Narrower for simple content like commands and URLs.
		width = min(int(float64(area.Dx())*simpleSizeRatio), simpleMaxWidth)
		maxHeight = int(float64(area.Dy()) * simpleHeightRatio)
	}
	return width, maxHeight, forceFullscreen
}

// contentViewportHeight returns the height for the scrollable content
// viewport given the height taken by the fixed chrome (fixedHeight) and
// the content's natural height. Simple prompts shrink to fit their
// content; diff and fullscreen views always take all remaining height.
func (p *Permissions) contentViewportHeight(maxHeight, fixedHeight, contentHeight int) int {
	// Never more than the room the chrome leaves, so the buttons and the
	// bottom border stay on screen in a short window.
	room := max(maxHeight-fixedHeight, 1)
	if p.hasDiffView() {
		return room
	}
	// A small window makes the dialog as large as the window, but simple
	// content still shrinks to fit rather than padding out empty rows.
	return min(max(contentHeight, 3), room)
}

// minContentLines is how many rows of the request content (the command or
// diff) the dialog tries to keep visible when the window is short.
const minContentLines = 3

// fitChrome trims the fixed chrome around the content viewport so the
// content keeps at least minContent rows within innerHeight (the dialog
// height inside its frame). The request content is what the user is
// approving, so it wins over decoration: spacer lines go first, then the
// stacked buttons collapse onto one compact row when it fits, then the
// help line goes, and finally the header is cut down from the bottom. The
// returned spacing is the number of blank lines to reserve.
func (p *Permissions) fitChrome(innerHeight, minContent, contentWidth int, header, buttons, helpView string) (string, string, string, int) {
	minContent = max(minContent, 1)
	spacing := layoutSpacingLines
	short := func() bool {
		used := lipgloss.Height(header) + lipgloss.Height(buttons) + spacing
		if helpView != "" {
			used += lipgloss.Height(helpView)
		}
		return innerHeight-used < minContent
	}
	if !short() {
		return header, buttons, helpView, spacing
	}
	spacing = 0
	if short() && lipgloss.Height(buttons) > 1 {
		if compact := p.renderButtonGroupPadded(" ", 1); lipgloss.Width(compact) <= contentWidth {
			buttons = lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(compact)
		}
	}
	if short() {
		helpView = ""
	}
	if short() {
		lines := slices.DeleteFunc(strings.Split(header, "\n"), func(l string) bool {
			return strings.TrimSpace(ansi.Strip(l)) == ""
		})
		keep := innerHeight - lipgloss.Height(buttons) - minContent
		header = strings.Join(lines[:max(1, min(len(lines), keep))], "\n")
	}
	return header, buttons, helpView, spacing
}

func (p *Permissions) renderHeader(contentWidth int) string {
	t := p.com.Styles

	// The dialog frame already pads the content, so drop the title's
	// own horizontal padding to line it up with the body and help.
	titleStyle := t.Dialog.Title.PaddingLeft(0).PaddingRight(0)
	title := common.DialogTitle(t, "Permission Required", contentWidth-titleStyle.GetHorizontalFrameSize(), t.Dialog.TitleGradFromColor, t.Dialog.TitleGradToColor)
	title = titleStyle.Render(title)

	lines := []string{title}

	// Add warning for dangerous commands, naming what tripped the check so
	// the user can judge it rather than just be alarmed by it. The reason
	// names a shell construct where it cannot name a command, and those
	// run long, so the line wraps rather than clipping: a warning cut off
	// mid-phrase is worse than one that costs a second line.
	if p.permission.Danger != "" {
		warning := lipgloss.NewStyle().
			Foreground(t.Status.WarnIndicator.GetBackground()).
			Width(contentWidth).
			Render(ansi.Wordwrap("⚠ Potentially dangerous: "+p.permission.Danger, contentWidth, ""))
		lines = append(lines, "", warning)
	}

	lines = append(lines, "")

	// Tool info.
	toolLine := p.renderToolName(contentWidth)
	lines = append(lines, toolLine)

	// Show generic Path only for tools that don't render their own file/path line.
	switch p.permission.ToolName {
	case tools.EditToolName, tools.WriteToolName, tools.MultiEditToolName,
		tools.ViewToolName, tools.ReplaceSymbolToolName,
		tools.DownloadToolName, tools.LSToolName:
		// These tools show their own File/Directory line below.
	default:
		lines = append(lines, p.renderKeyValue("Path", fsext.PrettyPath(p.permission.Path), contentWidth))
	}

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
	case tools.EditToolName, tools.WriteToolName, tools.MultiEditToolName, tools.ViewToolName, tools.ReplaceSymbolToolName:
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
		case tools.ReplaceSymbolPermissionsParams:
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
	keyStyle := t.Dialog.Permissions.KeyText
	valueStyle := t.Dialog.Permissions.ValueText

	keyStr := keyStyle.Render(key)
	valueStr := valueStyle.Width(width - lipgloss.Width(keyStr) - 1).Render(" " + value)

	return lipgloss.JoinHorizontal(lipgloss.Left, keyStr, valueStr)
}

func (p *Permissions) renderToolName(width int) string {
	toolName := p.permission.ToolName

	// Check if this is an MCP tool (format: mcp_<mcpname>_<toolname>).
	if strings.HasPrefix(toolName, "mcp_") {
		parts := strings.SplitN(toolName, "_", 3)
		if len(parts) == 3 {
			mcpName := prettyName(parts[1])
			toolPart := prettyName(parts[2])
			toolName = fmt.Sprintf("%s %s %s", mcpName, styles.ArrowRightIcon, toolPart)
		}
	}

	return p.renderKeyValue("Tool", toolName, width)
}

// prettyName converts snake_case or kebab-case to Title Case.
func prettyName(name string) string {
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	return stringext.Capitalize(name)
}

// renderButtonGroup renders the permission buttons joined by spacing. On a
// flagged request the approving buttons are painted in the negative style,
// because approving is the destructive choice here and it should not wear
// the same inviting highlight it does on an ordinary prompt.
func (p *Permissions) renderButtonGroup(spacing string) string {
	return p.renderButtonGroupPadded(spacing, 0)
}

// renderButtonGroupPadded is renderButtonGroup with a custom horizontal
// button padding; zero keeps the default.
func (p *Permissions) renderButtonGroupPadded(spacing string, padding int) string {
	t := p.com.Styles
	approveStyles := t
	if p.permission.Danger != "" {
		danger := *t
		danger.Button.Focused = danger.Button.Negative
		approveStyles = &danger
	}

	parts := make([]string, len(permissionOptions))
	for i, o := range permissionOptions {
		st := t
		if o.action != PermissionDeny {
			st = approveStyles
		}
		text, underline := o.text, o.underlineIndex
		if o.action == PermissionAllowForSession {
			text, underline = p.sessionButtonLabel(o)
		}
		parts[i] = common.Button(st, common.ButtonOpts{
			Text:           text,
			UnderlineIndex: underline,
			Selected:       p.selectedOption == i,
			Padding:        padding,
		})
	}

	return strings.Join(parts, spacing)
}

// sessionButtonLabel returns the "allow for session" button's text and
// underline position. When the session grant for a shell command covers
// every call of its program, the button names the program so the user can
// see how far the approval reaches.
func (p *Permissions) sessionButtonLabel(o permissionOption) (string, int) {
	params, ok := p.permission.Params.(tools.BashPermissionsParams)
	if !ok || params.SessionProgram == "" {
		return o.text, o.underlineIndex
	}
	prefix := "Allow " + params.SessionProgram + " for "
	return prefix + "Session", ansi.StringWidth(prefix)
}

func (p *Permissions) renderButtons(contentWidth int, fullscreen bool) string {
	content := p.renderButtonGroup("  ")

	// Center when stacked or when the dialog fills the screen; otherwise
	// hug the right edge next to the content. Right-aligning across a
	// full-screen width leaves the buttons stranded in the corner.
	align := lipgloss.Right
	if fullscreen {
		align = lipgloss.Center
	}
	if lipgloss.Width(content) > contentWidth {
		content = p.renderButtonGroup("\n")
		align = lipgloss.Center
	}

	return lipgloss.NewStyle().
		Width(contentWidth).
		Align(align).
		Render(content)
}
