package model

import (
	"context"
	"image"
	"math/rand"
	"os"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/logo"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/version"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/screen"
	"github.com/charmbracelet/x/ansi"
)

// uiFocusState represents the current focus state of the UI.
type uiFocusState uint8

// Possible uiFocusState values.
const (
	uiFocusNone uiFocusState = iota
	uiFocusEditor
	uiFocusMain
)

type uiState uint8

// Possible uiState values.
const (
	uiConfigure uiState = iota
	uiInitialize
	uiLanding
	uiChat
	uiChatCompact
)

type sessionLoadedMsg struct {
	sess session.Session
}

type sessionFilesLoadedMsg struct {
	files []SessionFile
}

// UI represents the main user interface model.
type UI struct {
	com          *common.Common
	session      *session.Session
	sessionFiles []SessionFile

	// The width and height of the terminal in cells.
	width  int
	height int
	layout layout

	focus uiFocusState
	state uiState

	keyMap KeyMap
	keyenh tea.KeyboardEnhancementsMsg

	chat   *ChatModel
	dialog *dialog.Overlay
	help   help.Model

	// header is the last cached header logo
	header string

	// sendProgressBar instructs the TUI to send progress bar updates to the
	// terminal.
	sendProgressBar bool

	// QueryVersion instructs the TUI to query for the terminal version when it
	// starts.
	QueryVersion bool

	// Editor components
	textarea textarea.Model

	attachments []any // TODO: Implement attachments

	readyPlaceholder   string
	workingPlaceholder string

	// onboarding state
	onboarding struct {
		yesInitializeSelected bool
	}

	// lsp
	lspStates map[string]app.LSPClientInfo

	// mcp
	mcpStates map[string]mcp.ClientInfo

	// sidebarLogo keeps a cached version of the sidebar sidebarLogo.
	sidebarLogo string
}

// New creates a new instance of the [UI] model.
func New(com *common.Common) *UI {
	// Editor components
	ta := textarea.New()
	ta.SetStyles(com.Styles.TextArea)
	ta.ShowLineNumbers = false
	ta.CharLimit = -1
	ta.SetVirtualCursor(false)
	ta.Focus()

	ui := &UI{
		com:      com,
		dialog:   dialog.NewOverlay(),
		keyMap:   DefaultKeyMap(),
		help:     help.New(),
		focus:    uiFocusNone,
		state:    uiConfigure,
		textarea: ta,
	}

	// set onboarding state defaults
	ui.onboarding.yesInitializeSelected = true

	// If no provider is configured show the user the provider list
	if !com.Config().IsConfigured() {
		ui.state = uiConfigure
		// if the project needs initialization show the user the question
	} else if n, _ := config.ProjectNeedsInitialization(); n {
		ui.state = uiInitialize
		// otherwise go to the landing UI
	} else {
		ui.state = uiLanding
		ui.focus = uiFocusEditor
	}

	ui.setEditorPrompt()
	ui.randomizePlaceholders()
	ui.textarea.Placeholder = ui.readyPlaceholder
	ui.help.Styles = com.Styles.Help

	return ui
}

// Init initializes the UI model.
func (m *UI) Init() tea.Cmd {
	if m.QueryVersion {
		return tea.RequestTerminalVersion
	}
	return nil
}

// Update handles updates to the UI model.
func (m *UI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.EnvMsg:
		// Is this Windows Terminal?
		if !m.sendProgressBar {
			m.sendProgressBar = slices.Contains(msg, "WT_SESSION")
		}
	case sessionLoadedMsg:
		m.state = uiChat
		m.session = &msg.sess
	case sessionFilesLoadedMsg:
		m.sessionFiles = msg.files
	case pubsub.Event[history.File]:
		cmds = append(cmds, m.handleFileEvent(msg.Payload))
	case pubsub.Event[app.LSPEvent]:
		m.lspStates = app.GetLSPStates()
	case pubsub.Event[mcp.Event]:
		m.mcpStates = mcp.GetStates()
	case tea.TerminalVersionMsg:
		termVersion := strings.ToLower(msg.Name)
		// Only enable progress bar for the following terminals.
		if !m.sendProgressBar {
			m.sendProgressBar = strings.Contains(termVersion, "ghostty")
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.updateLayoutAndSize()
	case tea.KeyboardEnhancementsMsg:
		m.keyenh = msg
		if msg.SupportsKeyDisambiguation() {
			m.keyMap.Models.SetHelp("ctrl+m", "models")
			m.keyMap.Editor.Newline.SetHelp("shift+enter", "newline")
		}
	case tea.KeyPressMsg:
		cmds = append(cmds, m.handleKeyPressMsg(msg)...)
	}

	// This logic gets triggered on any message type, but should it?
	switch m.focus {
	case uiFocusMain:
	case uiFocusEditor:
		// Textarea placeholder logic
		if m.com.App.AgentCoordinator != nil && m.com.App.AgentCoordinator.IsBusy() {
			m.textarea.Placeholder = m.workingPlaceholder
		} else {
			m.textarea.Placeholder = m.readyPlaceholder
		}
		if m.com.App.Permissions.SkipRequests() {
			m.textarea.Placeholder = "Yolo mode!"
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *UI) loadSession(sessionID string) tea.Cmd {
	return func() tea.Msg {
		// TODO: handle error
		session, _ := m.com.App.Sessions.Get(context.Background(), sessionID)
		return sessionLoadedMsg{session}
	}
}

func (m *UI) handleKeyPressMsg(msg tea.KeyPressMsg) (cmds []tea.Cmd) {
	if m.dialog.HasDialogs() {
		return m.updateDialogs(msg)
	}

	switch {
	case key.Matches(msg, m.keyMap.Tab):
		switch m.state {
		case uiChat:
			if m.focus == uiFocusMain {
				m.focus = uiFocusEditor
				cmds = append(cmds, m.textarea.Focus())
			} else {
				m.focus = uiFocusMain
				m.textarea.Blur()
			}
		}
	case key.Matches(msg, m.keyMap.Help):
		m.help.ShowAll = !m.help.ShowAll
		m.updateLayoutAndSize()
		return cmds
	case key.Matches(msg, m.keyMap.Quit):
		if !m.dialog.ContainsDialog(dialog.QuitDialogID) {
			m.dialog.AddDialog(dialog.NewQuit(m.com))
			return
		}
		return cmds
	case key.Matches(msg, m.keyMap.Commands):
		// TODO: Implement me
		return cmds
	case key.Matches(msg, m.keyMap.Models):
		// TODO: Implement me
		return cmds
	case key.Matches(msg, m.keyMap.Sessions):
		// TODO: Implement me
		return cmds
	}

	cmds = append(cmds, m.updateFocused(msg)...)
	return cmds
}

// Draw implements [tea.Layer] and draws the UI model.
func (m *UI) Draw(scr uv.Screen, area uv.Rectangle) {
	layout := generateLayout(m, area.Dx(), area.Dy())

	// Update cached layout and component sizes if needed.
	if m.layout != layout {
		m.layout = layout
		m.updateSize()
	}

	// Clear the screen first
	screen.Clear(scr)

	switch m.state {
	case uiConfigure:
		header := uv.NewStyledString(m.header)
		header.Draw(scr, layout.header)

		mainView := lipgloss.NewStyle().Width(layout.main.Dx()).
			Height(layout.main.Dy()).
			Background(lipgloss.ANSIColor(rand.Intn(256))).
			Render(" Configure ")
		main := uv.NewStyledString(mainView)
		main.Draw(scr, layout.main)

	case uiInitialize:
		header := uv.NewStyledString(m.header)
		header.Draw(scr, layout.header)

		main := uv.NewStyledString(m.initializeView())
		main.Draw(scr, layout.main)

	case uiLanding:
		header := uv.NewStyledString(m.header)
		header.Draw(scr, layout.header)
		main := uv.NewStyledString(m.landingView())
		main.Draw(scr, layout.main)

		editor := uv.NewStyledString(m.textarea.View())
		editor.Draw(scr, layout.editor)

	case uiChat:
		header := uv.NewStyledString(m.header)
		header.Draw(scr, layout.header)
		m.drawSidebar(scr, layout.sidebar)
		mainView := lipgloss.NewStyle().Width(layout.main.Dx()).
			Height(layout.main.Dy()).
			Render(" Chat Messages ")
		main := uv.NewStyledString(mainView)
		main.Draw(scr, layout.main)

		editor := uv.NewStyledString(m.textarea.View())
		editor.Draw(scr, layout.editor)

	case uiChatCompact:
		header := uv.NewStyledString(m.header)
		header.Draw(scr, layout.header)

		mainView := lipgloss.NewStyle().Width(layout.main.Dx()).
			Height(layout.main.Dy()).
			Background(lipgloss.ANSIColor(rand.Intn(256))).
			Render(" Compact Chat Messages ")
		main := uv.NewStyledString(mainView)
		main.Draw(scr, layout.main)

		editor := uv.NewStyledString(m.textarea.View())
		editor.Draw(scr, layout.editor)
	}

	// Add help layer
	help := uv.NewStyledString(m.help.View(m))
	help.Draw(scr, layout.help)

	// Debugging rendering (visually see when the tui rerenders)
	if os.Getenv("CRUSH_UI_DEBUG") == "true" {
		debugView := lipgloss.NewStyle().Background(lipgloss.ANSIColor(rand.Intn(256))).Width(4).Height(2)
		debug := uv.NewStyledString(debugView.String())
		debug.Draw(scr, image.Rectangle{
			Min: image.Pt(4, 1),
			Max: image.Pt(8, 3),
		})
	}

	// This needs to come last to overlay on top of everything
	if m.dialog.HasDialogs() {
		if dialogView := m.dialog.View(); dialogView != "" {
			dialogWidth, dialogHeight := lipgloss.Width(dialogView), lipgloss.Height(dialogView)
			dialogArea := common.CenterRect(area, dialogWidth, dialogHeight)
			dialog := uv.NewStyledString(dialogView)
			dialog.Draw(scr, dialogArea)
		}
	}
}

// View renders the UI model's view.
func (m *UI) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.BackgroundColor = m.com.Styles.Background

	layout := generateLayout(m, m.width, m.height)
	if m.focus == uiFocusEditor && m.textarea.Focused() {
		cur := m.textarea.Cursor()
		cur.X++ // Adjust for app margins
		cur.Y += layout.editor.Min.Y
		v.Cursor = cur
	}

	// TODO: Switch to lipgloss.Canvas when available
	canvas := uv.NewScreenBuffer(m.width, m.height)
	canvas.Method = ansi.GraphemeWidth

	m.Draw(canvas, canvas.Bounds())

	content := strings.ReplaceAll(canvas.Render(), "\r\n", "\n") // normalize newlines
	contentLines := strings.Split(content, "\n")
	for i, line := range contentLines {
		// Trim trailing spaces for concise rendering
		contentLines[i] = strings.TrimRight(line, " ")
	}

	content = strings.Join(contentLines, "\n")
	v.Content = content
	if m.sendProgressBar && m.com.App != nil && m.com.App.AgentCoordinator != nil && m.com.App.AgentCoordinator.IsBusy() {
		// HACK: use a random percentage to prevent ghostty from hiding it
		// after a timeout.
		v.ProgressBar = tea.NewProgressBar(tea.ProgressBarIndeterminate, rand.Intn(100))
	}

	return v
}

// ShortHelp implements [help.KeyMap].
func (m *UI) ShortHelp() []key.Binding {
	var binds []key.Binding
	k := &m.keyMap

	switch m.state {
	case uiInitialize:
		binds = append(binds, k.Quit)
	default:
		// TODO: other states
		// if m.session == nil {
		// no session selected
		binds = append(binds,
			k.Commands,
			k.Models,
			k.Editor.Newline,
			k.Quit,
			k.Help,
		)
		// }
		// else {
		// we have a session
		// }

		// switch m.state {
		// case uiChat:
		// case uiEdit:
		// 	binds = append(binds,
		// 		k.Editor.AddFile,
		// 		k.Editor.SendMessage,
		// 		k.Editor.OpenEditor,
		// 		k.Editor.Newline,
		// 	)
		//
		// 	if len(m.attachments) > 0 {
		// 		binds = append(binds,
		// 			k.Editor.AttachmentDeleteMode,
		// 			k.Editor.DeleteAllAttachments,
		// 			k.Editor.Escape,
		// 		)
		// 	}
		// }
	}

	return binds
}

// FullHelp implements [help.KeyMap].
func (m *UI) FullHelp() [][]key.Binding {
	var binds [][]key.Binding
	k := &m.keyMap
	help := k.Help
	help.SetHelp("ctrl+g", "less")

	switch m.state {
	case uiInitialize:
		binds = append(binds,
			[]key.Binding{
				k.Quit,
			})
	default:
		if m.session == nil {
			// no session selected
			binds = append(binds,
				[]key.Binding{
					k.Commands,
					k.Models,
					k.Sessions,
				},
				[]key.Binding{
					k.Editor.Newline,
					k.Editor.AddImage,
					k.Editor.MentionFile,
					k.Editor.OpenEditor,
				},
				[]key.Binding{
					help,
				},
			)
		}
		// else {
		// we have a session
		// }
	}

	// switch m.state {
	// case uiChat:
	// case uiEdit:
	// 	binds = append(binds, m.ShortHelp())
	// }

	return binds
}

// updateDialogs updates the dialog overlay with the given message and returns cmds
func (m *UI) updateDialogs(msg tea.KeyPressMsg) (cmds []tea.Cmd) {
	updatedDialog, cmd := m.dialog.Update(msg)
	m.dialog = updatedDialog
	cmds = append(cmds, cmd)
	return cmds
}

// updateFocused updates the focused model (chat or editor) with the given message
// and appends any resulting commands to the cmds slice.
func (m *UI) updateFocused(msg tea.KeyPressMsg) (cmds []tea.Cmd) {
	switch m.state {
	case uiConfigure:
		return cmds
	case uiInitialize:
		return append(cmds, m.updateInitializeView(msg)...)
	case uiChat, uiLanding, uiChatCompact:
		switch m.focus {
		case uiFocusMain:
			cmds = append(cmds, m.updateChat(msg)...)
		case uiFocusEditor:
			switch {
			case key.Matches(msg, m.keyMap.Editor.Newline):
				m.textarea.InsertRune('\n')
			}

			ta, cmd := m.textarea.Update(msg)
			m.textarea = ta
			cmds = append(cmds, cmd)
			return cmds
		}
	}
	return cmds
}

// updateChat updates the chat model with the given message and appends any
// resulting commands to the cmds slice.
func (m *UI) updateChat(msg tea.KeyPressMsg) (cmds []tea.Cmd) {
	updatedChat, cmd := m.chat.Update(msg)
	m.chat = updatedChat
	cmds = append(cmds, cmd)
	return cmds
}

// updateLayoutAndSize updates the layout and sizes of UI components.
func (m *UI) updateLayoutAndSize() {
	m.layout = generateLayout(m, m.width, m.height)
	m.updateSize()
}

// updateSize updates the sizes of UI components based on the current layout.
func (m *UI) updateSize() {
	// Set help width
	m.help.SetWidth(m.layout.help.Dx())

	// Handle different app states
	switch m.state {
	case uiConfigure, uiInitialize:
		m.renderHeader(false, m.layout.header.Dx())

	case uiLanding:
		m.renderHeader(false, m.layout.header.Dx())
		m.textarea.SetWidth(m.layout.editor.Dx())
		m.textarea.SetHeight(m.layout.editor.Dy())

	case uiChat:
		m.renderSidebarLogo(m.layout.sidebar.Dx())
		m.textarea.SetWidth(m.layout.editor.Dx())
		m.textarea.SetHeight(m.layout.editor.Dy())

	case uiChatCompact:
		// TODO: set the width and heigh of the chat component
		m.renderHeader(true, m.layout.header.Dx())
		m.textarea.SetWidth(m.layout.editor.Dx())
		m.textarea.SetHeight(m.layout.editor.Dy())
	}
}

// generateLayout calculates the layout rectangles for all UI components based
// on the current UI state and terminal dimensions.
func generateLayout(m *UI, w, h int) layout {
	// The screen area we're working with
	area := image.Rect(0, 0, w, h)

	// The help height
	helpHeight := 1
	// The editor height
	editorHeight := 5
	// The sidebar width
	sidebarWidth := 30
	// The header height
	// TODO: handle compact
	headerHeight := 4

	var helpKeyMap help.KeyMap = m
	if m.help.ShowAll {
		for _, row := range helpKeyMap.FullHelp() {
			helpHeight = max(helpHeight, len(row))
		}
	}

	// Add app margins
	appRect := area
	appRect.Min.X += 1
	appRect.Min.Y += 1
	appRect.Max.X -= 1
	appRect.Max.Y -= 1

	if slices.Contains([]uiState{uiConfigure, uiInitialize, uiLanding}, m.state) {
		// extra padding on left and right for these states
		appRect.Min.X += 1
		appRect.Max.X -= 1
	}

	appRect, helpRect := uv.SplitVertical(appRect, uv.Fixed(appRect.Dy()-helpHeight))

	layout := layout{
		area: area,
		help: helpRect,
	}

	// Handle different app states
	switch m.state {
	case uiConfigure, uiInitialize:
		// Layout
		//
		// header
		// ------
		// main
		// ------
		// help

		headerRect, mainRect := uv.SplitVertical(appRect, uv.Fixed(headerHeight))
		layout.header = headerRect
		layout.main = mainRect

	case uiLanding:
		// Layout
		//
		// header
		// ------
		// main
		// ------
		// editor
		// ------
		// help
		headerRect, mainRect := uv.SplitVertical(appRect, uv.Fixed(headerHeight))
		mainRect, editorRect := uv.SplitVertical(mainRect, uv.Fixed(mainRect.Dy()-editorHeight))
		// Remove extra padding from editor (but keep it for header and main)
		editorRect.Min.X -= 1
		editorRect.Max.X += 1
		layout.header = headerRect
		layout.main = mainRect
		layout.editor = editorRect

	case uiChat:
		// Layout
		//
		// ------|---
		// main  |
		// ------| side
		// editor|
		// ----------
		// help

		mainRect, sideRect := uv.SplitHorizontal(appRect, uv.Fixed(appRect.Dx()-sidebarWidth))
		// Add padding left
		sideRect.Min.X += 1
		mainRect, editorRect := uv.SplitVertical(mainRect, uv.Fixed(mainRect.Dy()-editorHeight))
		layout.sidebar = sideRect
		layout.main = mainRect
		layout.editor = editorRect

	case uiChatCompact:
		// Layout
		//
		// compact-header
		// ------
		// main
		// ------
		// editor
		// ------
		// help
		headerRect, mainRect := uv.SplitVertical(appRect, uv.Fixed(appRect.Dy()-headerHeight))
		mainRect, editorRect := uv.SplitVertical(mainRect, uv.Fixed(mainRect.Dy()-editorHeight))
		layout.header = headerRect
		layout.main = mainRect
		layout.editor = editorRect
	}

	if !layout.editor.Empty() {
		// Add editor margins 1 top and bottom
		layout.editor.Min.Y += 1
		layout.editor.Max.Y -= 1
	}

	return layout
}

// layout defines the positioning of UI elements.
type layout struct {
	// area is the overall available area.
	area uv.Rectangle

	// header is the header shown in special cases
	// e.x when the sidebar is collapsed
	// or when in the landing page
	// or in init/config
	header uv.Rectangle

	// main is the area for the main pane. (e.x chat, configure, landing)
	main uv.Rectangle

	// editor is the area for the editor pane.
	editor uv.Rectangle

	// sidebar is the area for the sidebar.
	sidebar uv.Rectangle

	// help is the area for the help view.
	help uv.Rectangle
}

// setEditorPrompt configures the textarea prompt function based on whether
// yolo mode is enabled.
func (m *UI) setEditorPrompt() {
	if m.com.App.Permissions.SkipRequests() {
		m.textarea.SetPromptFunc(4, m.yoloPromptFunc)
		return
	}
	m.textarea.SetPromptFunc(4, m.normalPromptFunc)
}

// normalPromptFunc returns the normal editor prompt style ("  > " on first
// line, "::: " on subsequent lines).
func (m *UI) normalPromptFunc(info textarea.PromptInfo) string {
	t := m.com.Styles
	if info.LineNumber == 0 {
		return "  > "
	}
	if info.Focused {
		return t.EditorPromptNormalFocused.Render()
	}
	return t.EditorPromptNormalBlurred.Render()
}

// yoloPromptFunc returns the yolo mode editor prompt style with warning icon
// and colored dots.
func (m *UI) yoloPromptFunc(info textarea.PromptInfo) string {
	t := m.com.Styles
	if info.LineNumber == 0 {
		if info.Focused {
			return t.EditorPromptYoloIconFocused.Render()
		} else {
			return t.EditorPromptYoloIconBlurred.Render()
		}
	}
	if info.Focused {
		return t.EditorPromptYoloDotsFocused.Render()
	}
	return t.EditorPromptYoloDotsBlurred.Render()
}

var readyPlaceholders = [...]string{
	"Ready!",
	"Ready...",
	"Ready?",
	"Ready for instructions",
}

var workingPlaceholders = [...]string{
	"Working!",
	"Working...",
	"Brrrrr...",
	"Prrrrrrrr...",
	"Processing...",
	"Thinking...",
}

// randomizePlaceholders selects random placeholder text for the textarea's
// ready and working states.
func (m *UI) randomizePlaceholders() {
	m.workingPlaceholder = workingPlaceholders[rand.Intn(len(workingPlaceholders))]
	m.readyPlaceholder = readyPlaceholders[rand.Intn(len(readyPlaceholders))]
}

// renderHeader renders and caches the header logo at the specified width.
func (m *UI) renderHeader(compact bool, width int) {
	// TODO: handle the compact case differently
	m.header = renderLogo(m.com.Styles, compact, width)
}

// renderSidebarLogo renders and caches the sidebar logo at the specified
// width.
func (m *UI) renderSidebarLogo(width int) {
	m.sidebarLogo = renderLogo(m.com.Styles, true, width)
}

// renderLogo renders the Crush logo with the given styles and dimensions.
func renderLogo(t *styles.Styles, compact bool, width int) string {
	return logo.Render(version.Version, compact, logo.Opts{
		FieldColor:   t.LogoFieldColor,
		TitleColorA:  t.LogoTitleColorA,
		TitleColorB:  t.LogoTitleColorB,
		CharmColor:   t.LogoCharmColor,
		VersionColor: t.LogoVersionColor,
		Width:        width,
	})
}
