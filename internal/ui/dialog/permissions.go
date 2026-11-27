package dialog

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
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

// Permissions represents a dialog for permission requests.
type Permissions struct {
	com          *common.Common
	windowWidth  int // Terminal window dimensions.
	windowHeight int
	fullscreen   bool // true when dialog is fullscreen

	permission     permission.PermissionRequest
	selectedOption int // 0: Allow, 1: Allow for session, 2: Deny

	viewport      viewport.Model
	viewportDirty bool // true when viewport content needs to be re-rendered
	viewportWidth int

	// Diff view state.
	diffSplitMode        *bool // nil means use default based on width
	defaultDiffSplitMode bool  // default split mode based on width
	diffXOffset          int   // horizontal scroll offset for diff view
	unifiedDiffContent   string
	splitDiffContent     string

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
	Choose           key.Binding
	Scroll           key.Binding
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
			// ctrl+y is deliberately absent: it is the global yolo-mode
			// toggle, and dialogs see keys first, so accepting it here
			// turns "stop asking me" into "approve this one".
			key.WithKeys("enter"),
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
		// Esc refuses the request, so say so rather than "exit".
		Close: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "deny"),
		),
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
		Choose: key.NewBinding(
			key.WithKeys("left", "right"),
			key.WithHelp("←/→", "choose"),
		),
		Scroll: key.NewBinding(
			key.WithKeys("shift+left", "shift+down", "shift+up", "shift+right"),
			key.WithHelp("shift+←↓↑→", "scroll"),
		),
	}
}

// permissionOption describes one button in the dialog. The slice index is
// what selectedOption holds, so the buttons, the keyboard cursor, and the
// action they resolve to cannot drift apart.
type permissionOption struct {
	text           string
	underlineIndex int
	action         PermissionAction
}

// permissionOptions are the dialog's buttons in display order.
var permissionOptions = []permissionOption{
	{"Allow", 0, PermissionAllow},
	{"Allow for Session", 10, PermissionAllowForSession},
	{"Deny", 0, PermissionDeny},
}

// optionIndex returns the position of the button that resolves to action.
func optionIndex(action PermissionAction) int {
	for i, o := range permissionOptions {
		if o.action == action {
			return i
		}
	}
	return 0
}

// defaultSelectedOption picks the button the cursor starts on. A flagged
// request starts on Deny so that answering the prompt by reflex refuses
// instead of approving.
func defaultSelectedOption(perm permission.PermissionRequest) int {
	if perm.Danger != "" {
		return optionIndex(PermissionDeny)
	}
	return optionIndex(PermissionAllow)
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

	km := defaultPermissionsKeyMap()

	// Configure viewport with matching keybindings.
	vp := viewport.New()
	vp.KeyMap = viewport.KeyMap{
		Up:    km.ScrollUp,
		Down:  km.ScrollDown,
		Left:  km.ScrollLeft,
		Right: km.ScrollRight,
		// Disable other viewport keys to avoid conflicts with dialog shortcuts.
		PageUp:       key.NewBinding(key.WithDisabled()),
		PageDown:     key.NewBinding(key.WithDisabled()),
		HalfPageUp:   key.NewBinding(key.WithDisabled()),
		HalfPageDown: key.NewBinding(key.WithDisabled()),
	}

	p := &Permissions{
		com:            com,
		permission:     perm,
		selectedOption: defaultSelectedOption(perm),
		viewport:       vp,
		help:           h,
		keyMap:         km,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Calculate usable content width (dialog border + horizontal padding).
func (p *Permissions) calculateContentWidth(width int) int {
	t := p.com.Styles
	const dialogHorizontalPadding = 2
	return width - t.Dialog.View.GetHorizontalFrameSize() - dialogHorizontalPadding
}

// ID implements [Dialog].
func (*Permissions) ID() string {
	return PermissionsID
}

// ToolCallID returns the tool call ID associated with this dialog's
// permission request.
func (p *Permissions) ToolCallID() string {
	return p.permission.ToolCallID
}

// RequestID returns the ID of the permission request this dialog answers.
func (p *Permissions) RequestID() string {
	return p.permission.ID
}

// HandleMsg implements [Dialog].
func (p *Permissions) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, p.keyMap.Close):
			// Escape denies the permission request.
			return p.respond(PermissionDeny)
		case key.Matches(msg, p.keyMap.Right), key.Matches(msg, p.keyMap.Tab):
			p.selectedOption = (p.selectedOption + 1) % len(permissionOptions)
		case key.Matches(msg, p.keyMap.Left):
			// Step forward by one short of a full cycle to avoid negative
			// modulo.
			p.selectedOption = (p.selectedOption + len(permissionOptions) - 1) % len(permissionOptions)
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
				p.viewportDirty = true
			}
		case key.Matches(msg, p.keyMap.ToggleFullscreen):
			if p.hasDiffView() {
				p.fullscreen = !p.fullscreen
			}
		case key.Matches(msg, p.keyMap.ScrollDown):
			p.viewport, _ = p.viewport.Update(msg)
		case key.Matches(msg, p.keyMap.ScrollUp):
			p.viewport, _ = p.viewport.Update(msg)
		case key.Matches(msg, p.keyMap.ScrollLeft):
			if p.hasDiffView() {
				p.scrollLeft()
			} else {
				p.viewport, _ = p.viewport.Update(msg)
			}
		case key.Matches(msg, p.keyMap.ScrollRight):
			if p.hasDiffView() {
				p.scrollRight()
			} else {
				p.viewport, _ = p.viewport.Update(msg)
			}
		}
	case common.CoalescedWheelMsg:
		if p.hasDiffView() {
			if msg.DeltaX < 0 {
				p.scrollLeft()
			} else if msg.DeltaX > 0 {
				p.scrollRight()
			} else {
				p.viewport, _ = p.viewport.Update(tea.MouseWheelMsg(msg.Mouse))
			}
		} else {
			p.viewport, _ = p.viewport.Update(tea.MouseWheelMsg(msg.Mouse))
		}
	default:
		// Pass unhandled keys to viewport for non-diff content scrolling.
		if !p.hasDiffView() {
			p.viewport, _ = p.viewport.Update(msg)
			p.viewportDirty = true
		}
	}

	return nil
}

func (p *Permissions) selectCurrentOption() tea.Msg {
	if p.selectedOption < 0 || p.selectedOption >= len(permissionOptions) {
		// Only reachable through a bug, and refusing is the safe way to
		// be wrong.
		return p.respond(PermissionDeny)
	}
	return p.respond(permissionOptions[p.selectedOption].action)
}

func (p *Permissions) respond(action PermissionAction) tea.Msg {
	return ActionPermissionResponse{
		Permission: p.permission,
		Action:     action,
	}
}

func (p *Permissions) hasDiffView() bool {
	switch p.permission.ToolName {
	case tools.EditToolName, tools.WriteToolName, tools.MultiEditToolName, tools.ReplaceSymbolToolName:
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

const horizontalScrollStep = 5

func (p *Permissions) scrollLeft() {
	p.diffXOffset = max(0, p.diffXOffset-horizontalScrollStep)
	p.viewportDirty = true
}

func (p *Permissions) scrollRight() {
	p.diffXOffset += horizontalScrollStep
	p.viewportDirty = true
}

// Draw implements [Dialog].
func (p *Permissions) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := p.com.Styles
	width, maxHeight, forceFullscreen := p.dialogSize(area)
	dialogStyle := t.Dialog.View.Width(width).Padding(0, 1)
	// The dialog fills the screen when forced small or when a diff is
	// expanded; center the buttons then instead of hugging the far edge.
	fullscreen := forceFullscreen || (p.fullscreen && p.hasDiffView())

	contentWidth := p.calculateContentWidth(width)
	header := p.renderHeader(contentWidth)
	buttons := p.renderButtons(contentWidth, fullscreen)
	// Pack the hints to the content width so they truncate cleanly instead
	// of overflowing. The dialog frame supplies the padding, so this renders
	// the hint line without the extra help view inset that renderDialogHelp
	// applies for RenderContext dialogs.
	helpView := shortHelpLine(&p.help, p.ShortHelp(), contentWidth)

	p.defaultDiffSplitMode = width >= splitModeMinWidth

	// Pre-render content to measure its actual height, then fit the
	// scrollable viewport into whatever height the fixed chrome leaves.
	renderedContent := p.renderContent(contentWidth)
	contentHeight := lipgloss.Height(renderedContent)
	// Diffs always fill the remaining height, and their pre-rendered
	// height can be stale here, so they always ask for the full minimum.
	minContent := min(contentHeight, minContentLines)
	if p.hasDiffView() {
		minContent = minContentLines
	}
	header, buttons, helpView, spacing := p.fitChrome(maxHeight-dialogStyle.GetVerticalFrameSize(),
		minContent, contentWidth, header, buttons, helpView)
	fixedHeight := lipgloss.Height(header) + lipgloss.Height(buttons) +
		dialogStyle.GetVerticalFrameSize() + spacing
	if helpView != "" {
		fixedHeight += lipgloss.Height(helpView)
	}
	availableHeight := p.contentViewportHeight(maxHeight, fixedHeight, contentHeight)

	// Determine if scrollbar is needed.
	needsScrollbar := p.hasDiffView() || contentHeight > availableHeight
	viewportWidth := contentWidth
	if needsScrollbar {
		viewportWidth = contentWidth - 1 // Reserve space for scrollbar.
	}

	if p.viewport.Width() != viewportWidth {
		// Mark content as dirty if width has changed.
		p.viewportDirty = true
		renderedContent = p.renderContent(viewportWidth)
	}

	var content string
	p.viewport.SetWidth(viewportWidth)
	p.viewport.SetHeight(availableHeight)
	if p.viewportDirty {
		p.viewport.SetContent(renderedContent)
		p.viewportWidth = p.viewport.Width()
		p.viewportDirty = false
	} else {
		// SetHeight does not clamp the offset; only SetContent does. Re-set
		// it so a taller viewport does not stay scrolled past the end.
		p.viewport.SetYOffset(p.viewport.YOffset())
	}
	content = p.viewport.View()
	if needsScrollbar {
		content = joinScrollbar(t, content, availableHeight, p.viewport.TotalLineCount(), availableHeight, p.viewport.YOffset())
	}

	// Blank spacer lines are dropped by fitChrome when the window is too
	// short to afford them.
	var gap []string
	if spacing > 0 {
		gap = []string{""}
	}
	parts := []string{header}
	if content != "" {
		parts = append(append(parts, gap...), content)
	}
	parts = append(append(parts, gap...), buttons)
	if helpView != "" {
		parts = append(append(parts, gap...), helpView)
	}

	innerContent := lipgloss.JoinVertical(lipgloss.Left, parts...)
	DrawCenterCursor(scr, area, dialogStyle.Render(innerContent), nil)
	return nil
}

func (p *Permissions) canScroll() bool {
	if p.hasDiffView() {
		// Diff views can always scroll.
		return true
	}
	// For non-diff content, check if viewport has scrollable content.
	return !p.viewport.AtTop() || !p.viewport.AtBottom()
}

// ShortHelp implements [help.KeyMap].
func (p *Permissions) ShortHelp() []key.Binding {
	bindings := []key.Binding{
		p.keyMap.Choose,
		p.keyMap.Select,
		p.keyMap.Close,
	}

	if p.canScroll() {
		bindings = append(bindings, p.keyMap.Scroll)
	}

	if p.hasDiffView() {
		bindings = append(
			bindings,
			p.keyMap.ToggleDiffMode,
			p.keyMap.ToggleFullscreen,
		)
	}

	return bindings
}

// FullHelp implements [help.KeyMap].
func (p *Permissions) FullHelp() [][]key.Binding {
	return [][]key.Binding{p.ShortHelp()}
}
