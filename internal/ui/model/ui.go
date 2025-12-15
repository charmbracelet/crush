package model

import (
	"context"
	"image"
	"math/rand"
	"os"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/logo"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/version"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/screen"
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

// sessionsLoadedMsg is a message indicating that sessions have been loaded.
type sessionsLoadedMsg struct {
	sessions []session.Session
}

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

	// Chat components
	chat *chat.Chat

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

	ch := chat.NewChat(com)

	ui := &UI{
		com:      com,
		dialog:   dialog.NewOverlay(),
		keyMap:   DefaultKeyMap(),
		help:     help.New(),
		focus:    uiFocusNone,
		state:    uiConfigure,
		textarea: ta,
		chat:     ch,
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
	var cmds []tea.Cmd
	if m.QueryVersion {
		cmds = append(cmds, tea.RequestTerminalVersion)
	}
	return tea.Batch(cmds...)
}

// sessionLoadedDoneMsg indicates that session loading and message appending is
// done.
type sessionLoadedDoneMsg struct{}

// Update handles updates to the UI model.
func (m *UI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.EnvMsg:
		// Is this Windows Terminal?
		if !m.sendProgressBar {
			m.sendProgressBar = slices.Contains(msg, "WT_SESSION")
		}
	case sessionsLoadedMsg:
		sessions := dialog.NewSessions(m.com, msg.sessions...)
		sessions.SetSize(min(120, m.width-8), 30)
		m.dialog.AddDialog(sessions)
	case dialog.SessionSelectedMsg:
		m.dialog.RemoveDialog(dialog.SessionDialogID)
		cmds = append(cmds,
			m.loadSession(msg.Session.ID),
			m.loadSessionFiles(msg.Session.ID),
		)
	case sessionLoadedMsg:
		m.state = uiChat
		m.session = &msg.sess
		// TODO: handle error.
		msgs, _ := m.com.App.Messages.List(context.Background(), m.session.ID)

		m.chat.SetMessages(m.convertChatMessages(msgs)...)

		// Notify that session loading is done to scroll to bottom. This is
		// needed because we need to draw the chat list first before we can
		// scroll to bottom.
		cmds = append(cmds, func() tea.Msg {
			return sessionLoadedDoneMsg{}
		})
	case sessionLoadedDoneMsg:
		m.chat.ScrollToBottom()
		m.chat.SelectLast()
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
	case tea.MouseClickMsg:
		switch m.state {
		case uiChat:
			x, y := msg.X, msg.Y
			// Adjust for chat area position
			x -= m.layout.main.Min.X
			y -= m.layout.main.Min.Y
			m.chat.HandleMouseDown(x, y)
		}

	case tea.MouseMotionMsg:
		switch m.state {
		case uiChat:
			if msg.Y <= 0 {
				m.chat.ScrollBy(-1)
				if !m.chat.SelectedItemInView() {
					m.chat.SelectPrev()
					m.chat.ScrollToSelected()
				}
			} else if msg.Y >= m.chat.Height()-1 {
				m.chat.ScrollBy(1)
				if !m.chat.SelectedItemInView() {
					m.chat.SelectNext()
					m.chat.ScrollToSelected()
				}
			}

			x, y := msg.X, msg.Y
			// Adjust for chat area position
			x -= m.layout.main.Min.X
			y -= m.layout.main.Min.Y
			m.chat.HandleMouseDrag(x, y)
		}

	case tea.MouseReleaseMsg:
		switch m.state {
		case uiChat:
			x, y := msg.X, msg.Y
			// Adjust for chat area position
			x -= m.layout.main.Min.X
			y -= m.layout.main.Min.Y
			m.chat.HandleMouseUp(x, y)
		}
	case tea.MouseWheelMsg:
		switch m.state {
		case uiChat:
			switch msg.Button {
			case tea.MouseWheelUp:
				m.chat.ScrollBy(-5)
				if !m.chat.SelectedItemInView() {
					m.chat.SelectPrev()
					m.chat.ScrollToSelected()
				}
			case tea.MouseWheelDown:
				m.chat.ScrollBy(5)
				if !m.chat.SelectedItemInView() {
					m.chat.SelectNext()
					m.chat.ScrollToSelected()
				}
			}
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
	handleQuitKeys := func(msg tea.KeyPressMsg) bool {
		switch {
		case key.Matches(msg, m.keyMap.Quit):
			if !m.dialog.ContainsDialog(dialog.QuitDialogID) {
				m.dialog.AddDialog(dialog.NewQuit(m.com))
				return true
			}
		}
		return false
	}

	handleGlobalKeys := func(msg tea.KeyPressMsg) bool {
		if handleQuitKeys(msg) {
			return true
		}
		switch {
		case key.Matches(msg, m.keyMap.Tab):
		case key.Matches(msg, m.keyMap.Help):
			m.help.ShowAll = !m.help.ShowAll
			m.updateLayoutAndSize()
			return true
		case key.Matches(msg, m.keyMap.Commands):
			// TODO: Implement me
		case key.Matches(msg, m.keyMap.Models):
			// TODO: Implement me
		case key.Matches(msg, m.keyMap.Sessions):
			if m.dialog.ContainsDialog(dialog.SessionDialogID) {
				// Bring to front
				m.dialog.BringToFront(dialog.SessionDialogID)
			} else {
				cmds = append(cmds, m.loadSessionsCmd)
			}
			return true
		}
		return false
	}

	if m.dialog.HasDialogs() {
		// Always handle quit keys first
		if handleQuitKeys(msg) {
			return cmds
		}

		updatedDialog, cmd := m.dialog.Update(msg)
		m.dialog = updatedDialog
		cmds = append(cmds, cmd)
		return cmds
	}

	switch m.state {
	case uiChat:
		switch {
		case key.Matches(msg, m.keyMap.Tab):
			if m.focus == uiFocusMain {
				m.focus = uiFocusEditor
				cmds = append(cmds, m.textarea.Focus())
				m.chat.Blur()
			} else {
				m.focus = uiFocusMain
				m.textarea.Blur()
				m.chat.Focus()
				m.chat.SetSelected(m.chat.Len() - 1)
			}
		case key.Matches(msg, m.keyMap.Chat.Up) && m.focus == uiFocusMain:
			m.chat.ScrollBy(-1)
			if !m.chat.SelectedItemInView() {
				m.chat.SelectPrev()
				m.chat.ScrollToSelected()
			}
		case key.Matches(msg, m.keyMap.Chat.Down) && m.focus == uiFocusMain:
			m.chat.ScrollBy(1)
			if !m.chat.SelectedItemInView() {
				m.chat.SelectNext()
				m.chat.ScrollToSelected()
			}
		case key.Matches(msg, m.keyMap.Chat.UpOneItem) && m.focus == uiFocusMain:
			m.chat.SelectPrev()
			m.chat.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Chat.DownOneItem) && m.focus == uiFocusMain:
			m.chat.SelectNext()
			m.chat.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Chat.HalfPageUp) && m.focus == uiFocusMain:
			m.chat.ScrollBy(-m.chat.Height() / 2)
			m.chat.SelectFirstInView()
		case key.Matches(msg, m.keyMap.Chat.HalfPageDown) && m.focus == uiFocusMain:
			m.chat.ScrollBy(m.chat.Height() / 2)
			m.chat.SelectLastInView()
		case key.Matches(msg, m.keyMap.Chat.PageUp) && m.focus == uiFocusMain:
			m.chat.ScrollBy(-m.chat.Height())
			m.chat.SelectFirstInView()
		case key.Matches(msg, m.keyMap.Chat.PageDown) && m.focus == uiFocusMain:
			m.chat.ScrollBy(m.chat.Height())
			m.chat.SelectLastInView()
		case key.Matches(msg, m.keyMap.Chat.Home) && m.focus == uiFocusMain:
			m.chat.ScrollToTop()
			m.chat.SelectFirst()
		case key.Matches(msg, m.keyMap.Chat.End) && m.focus == uiFocusMain:
			m.chat.ScrollToBottom()
			m.chat.SelectLast()
		default:
			// Try to handle key press in focused item (for expansion, etc.)
			if m.focus == uiFocusMain && m.chat.HandleKeyPress(msg) {
				return cmds
			}
			handleGlobalKeys(msg)
		}
	default:
		handleGlobalKeys(msg)
	}

	cmds = append(cmds, m.updateFocused(msg)...)
	return cmds
}

// Draw implements [tea.Layer] and draws the UI model.
func (m *UI) Draw(scr uv.Screen, area uv.Rectangle) {
	layout := generateLayout(m, area.Dx(), area.Dy())

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
		m.chat.Draw(scr, layout.main)

		header := uv.NewStyledString(m.header)
		header.Draw(scr, layout.header)
		m.drawSidebar(scr, layout.sidebar)

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
		dialogLayers := m.dialog.Layers()
		layers := make([]*lipgloss.Layer, 0)
		for _, layer := range dialogLayers {
			if layer == nil {
				continue
			}
			layerW, layerH := layer.Width(), layer.Height()
			layerArea := common.CenterRect(area, layerW, layerH)
			layers = append(layers, layer.X(layerArea.Min.X).Y(layerArea.Min.Y))
		}

		comp := lipgloss.NewCompositor(layers...)
		comp.Draw(scr, area)
	}
}

// Cursor returns the cursor position and properties for the UI model. It
// returns nil if the cursor should not be shown.
func (m *UI) Cursor() *tea.Cursor {
	if m.layout.editor.Dy() <= 0 {
		// Don't show cursor if editor is not visible
		return nil
	}
	if m.focus == uiFocusEditor && m.textarea.Focused() {
		cur := m.textarea.Cursor()
		cur.X++ // Adjust for app margins
		cur.Y += m.layout.editor.Min.Y
		return cur
	}
	return nil
}

// View renders the UI model's view.
func (m *UI) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.BackgroundColor = m.com.Styles.Background
	v.Cursor = m.Cursor()
	v.MouseMode = tea.MouseModeCellMotion

	canvas := uv.NewScreenBuffer(m.width, m.height)
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

// updateLayoutAndSize updates the layout and sizes of UI components.
func (m *UI) updateLayoutAndSize() {
	m.layout = generateLayout(m, m.width, m.height)
	m.updateSize()
}

// updateSize updates the sizes of UI components based on the current layout.
func (m *UI) updateSize() {
	// Set help width
	m.help.SetWidth(m.layout.help.Dx())

	m.chat.SetSize(m.layout.main.Dx(), m.layout.main.Dy())
	m.textarea.SetWidth(m.layout.editor.Dx())
	m.textarea.SetHeight(m.layout.editor.Dy())

	// Handle different app states
	switch m.state {
	case uiConfigure, uiInitialize, uiLanding:
		m.renderHeader(false, m.layout.header.Dx())

	case uiChat:
		m.renderSidebarLogo(m.layout.sidebar.Dx())

	case uiChatCompact:
		// TODO: set the width and heigh of the chat component
		m.renderHeader(true, m.layout.header.Dx())
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
		mainRect.Max.X -= 1 // Add padding right
		// Add bottom margin to main
		mainRect.Max.Y -= 1
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

// loadSessionsCmd loads the list of sessions and returns a command that sends
// a sessionFilesLoadedMsg when done.
func (m *UI) loadSessionsCmd() tea.Msg {
	allSessions, _ := m.com.App.Sessions.List(context.TODO())
	return sessionsLoadedMsg{sessions: allSessions}
}

// convertChatMessages converts messages to chat message items
func (m *UI) convertChatMessages(msgs []message.Message) []chat.MessageItem {
	items := make([]chat.MessageItem, 0)

	// Build tool result map for efficient lookup.
	toolResultMap := m.buildToolResultMap(msgs)

	var lastUserMessageTime time.Time

	for _, msg := range msgs {
		switch msg.Role {
		case message.User:
			lastUserMessageTime = time.Unix(msg.CreatedAt, 0)
			items = append(items, chat.NewUserMessage(msg.ID, msg.Content().Text, msg.BinaryContent(), m.com.Styles))
		case message.Assistant:
			// Add assistant message and its tool calls.
			assistantItems := m.convertAssistantMessage(msg, toolResultMap)
			items = append(items, assistantItems...)

			// Add section separator if assistant finished with EndTurn.
			if msg.FinishReason() == message.FinishReasonEndTurn {
				modelName := m.getModelName(msg)
				items = append(items, chat.NewSectionItem(msg, lastUserMessageTime, modelName, m.com.Styles))
			}
		}
	}
	return items
}

// getModelName returns the display name for a model, or "Unknown Model" if not found.
func (m *UI) getModelName(msg message.Message) string {
	model := m.com.Config().GetModel(msg.Provider, msg.Model)
	if model == nil {
		return "Unknown Model"
	}
	return model.Name
}

// buildToolResultMap creates a map of tool call ID to tool result for efficient lookup.
func (m *UI) buildToolResultMap(messages []message.Message) map[string]message.ToolResult {
	toolResultMap := make(map[string]message.ToolResult)
	for _, msg := range messages {
		for _, tr := range msg.ToolResults() {
			toolResultMap[tr.ToolCallID] = tr
		}
	}
	return toolResultMap
}

// convertAssistantMessage converts an assistant message and its tool calls to UI items.
func (m *UI) convertAssistantMessage(msg message.Message, toolResultMap map[string]message.ToolResult) []chat.MessageItem {
	var items []chat.MessageItem

	// Add assistant text/thinking message if it has content.
	content := strings.TrimSpace(msg.Content().Text)
	thinking := strings.TrimSpace(msg.ReasoningContent().Thinking)
	isError := msg.FinishReason() == message.FinishReasonError
	isCancelled := msg.FinishReason() == message.FinishReasonCanceled

	// Show assistant message if there's content, thinking, or status to display.
	if content != "" || thinking != "" || isError || isCancelled {
		var finish message.Finish
		if fp := msg.FinishPart(); fp != nil {
			finish = *fp
		}

		items = append(items, chat.NewAssistantMessage(
			msg.ID,
			content,
			thinking,
			msg.IsFinished(),
			finish,
			m.com.Styles,
		))
	}

	// Add tool call items.
	for _, tc := range msg.ToolCalls() {
		ctx := m.buildToolCallContext(tc, msg, toolResultMap)

		// Handle nested tool calls for agent/agentic_fetch.
		if tc.Name == agent.AgentToolName || tc.Name == tools.AgenticFetchToolName {
			ctx.NestedCalls = m.loadNestedToolCalls(msg.ID, tc.ID)
		}

		items = append(items, chat.NewToolItem(ctx))
	}

	return items
}

// buildToolCallContext creates a ToolCallContext from a tool call and its result.
func (m *UI) buildToolCallContext(tc message.ToolCall, msg message.Message, toolResultMap map[string]message.ToolResult) chat.ToolCallContext {
	ctx := chat.ToolCallContext{
		Call:      tc,
		Styles:    m.com.Styles,
		IsNested:  false,
		Cancelled: msg.FinishReason() == message.FinishReasonCanceled,
	}

	// Add tool result if available.
	if tr, ok := toolResultMap[tc.ID]; ok {
		ctx.Result = &tr
	}

	// TODO: Add permission tracking when we have permission service.
	// ctx.PermissionRequested = ...
	// ctx.PermissionGranted = ...

	return ctx
}

// loadNestedToolCalls loads nested tool calls for agent/agentic_fetch tools.
func (m *UI) loadNestedToolCalls(msgID, toolCallID string) []chat.ToolCallContext {
	agentSessionID := m.com.App.Sessions.CreateAgentToolSessionID(msgID, toolCallID)
	nestedMsgs, err := m.com.App.Messages.List(context.Background(), agentSessionID)
	if err != nil || len(nestedMsgs) == 0 {
		return nil
	}

	// Build nested tool result map.
	nestedToolResultMap := m.buildToolResultMap(nestedMsgs)

	var nestedContexts []chat.ToolCallContext
	for _, nestedMsg := range nestedMsgs {
		for _, nestedTC := range nestedMsg.ToolCalls() {
			ctx := chat.ToolCallContext{
				Call:      nestedTC,
				Styles:    m.com.Styles,
				IsNested:  true,
				Cancelled: nestedMsg.FinishReason() == message.FinishReasonCanceled,
			}

			if tr, ok := nestedToolResultMap[nestedTC.ID]; ok {
				ctx.Result = &tr
			}

			nestedContexts = append(nestedContexts, ctx)
		}
	}

	return nestedContexts
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
