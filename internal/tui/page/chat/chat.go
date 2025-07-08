package chat

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/spinner"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/chat/editor"
	"github.com/charmbracelet/crush/internal/tui/components/chat/header"
	"github.com/charmbracelet/crush/internal/tui/components/chat/sidebar"
	"github.com/charmbracelet/crush/internal/tui/components/chat/splash"
	"github.com/charmbracelet/crush/internal/tui/components/completions"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/commands"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/filepicker"
	"github.com/charmbracelet/crush/internal/tui/page"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/charmbracelet/lipgloss/v2"
)

var ChatPageID page.PageID = "chat"

type (
	OpenFilePickerMsg struct{}
	ChatFocusedMsg    struct {
		Focused bool // True if the chat input is focused, false otherwise
	}
	CancelTimerExpiredMsg struct{}
)

type ChatState string

const (
	ChatStateOnboarding  ChatState = "onboarding"
	ChatStateInitProject ChatState = "init_project"
	ChatStateNewMessage  ChatState = "new_message"
	ChatStateInSession   ChatState = "in_session"
)

type PanelType string

const (
	PanelTypeChat   PanelType = "chat"
	PanelTypeEditor PanelType = "editor"
	PanelTypeSplash PanelType = "splash"
)

const (
	CompactModeBreakpoint = 120 // Width at which the chat page switches to compact mode
	EditorHeight          = 5   // Height of the editor input area including padding
	SideBarWidth          = 31  // Width of the sidebar
	SideBarDetailsPadding = 1   // Padding for the sidebar details section
	HeaderHeight          = 1   // Height of the header
)

type ChatPage interface {
	util.Model
	layout.Help
}

// cancelTimerCmd creates a command that expires the cancel timer after 2 seconds
func cancelTimerCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return CancelTimerExpiredMsg{}
	})
}

type chatPage struct {
	width, height               int
	detailsWidth, detailsHeight int
	app                         *app.App
	state                       ChatState
	session                     session.Session
	keyMap                      KeyMap
	focusedPane                 PanelType
	// Compact mode
	compact        bool
	header         header.Header
	showingDetails bool

	sidebar   sidebar.Sidebar
	chat      chat.MessageListCmp
	editor    editor.Editor
	splash    splash.Splash
	canceling bool

	// This will force the compact mode even in big screens
	// usually triggered by the user command
	// this will also be set when the user config is set to compact mode
	forceCompact bool
}

func New(app *app.App) ChatPage {
	return &chatPage{
		app:   app,
		state: ChatStateOnboarding,

		keyMap: DefaultKeyMap(),

		header:      header.New(app.LSPClients),
		sidebar:     sidebar.New(app.History, app.LSPClients, false),
		chat:        chat.New(app),
		editor:      editor.New(app),
		splash:      splash.New(),
		focusedPane: PanelTypeSplash,
	}
}

func (p *chatPage) Init() tea.Cmd {
	cfg := config.Get()
	if config.HasInitialDataConfig() {
		if b, _ := config.ProjectNeedsInitialization(); b {
			p.state = ChatStateInitProject
		} else {
			p.state = ChatStateNewMessage
			p.focusedPane = PanelTypeEditor
		}
	}

	compact := cfg.Options.TUI.CompactMode
	p.compact = compact
	p.forceCompact = compact
	p.sidebar.SetCompactMode(p.compact)
	return tea.Batch(
		p.header.Init(),
		p.sidebar.Init(),
		p.chat.Init(),
		p.editor.Init(),
		p.splash.Init(),
	)
}

func (p *chatPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return p, p.SetSize(msg.Width, msg.Height)
	case CancelTimerExpiredMsg:
		p.canceling = false
		return p, nil
	case chat.SendMsg:
		return p, p.sendMessage(msg.Text, msg.Attachments)
	case chat.SessionSelectedMsg:
		return p, p.setSession(msg)
	case commands.ToggleCompactModeMsg:
		p.forceCompact = !p.forceCompact
		var cmd tea.Cmd
		if p.forceCompact {
			p.setCompactMode(true)
			cmd = p.updateCompactConfig(true)
		} else if p.width >= CompactModeBreakpoint {
			p.setCompactMode(false)
			cmd = p.updateCompactConfig(false)
		}
		return p, tea.Batch(p.SetSize(p.width, p.height), cmd)
	case pubsub.Event[session.Session]:
		// this needs to go to header/sidebar
		u, cmd := p.header.Update(msg)
		p.header = u.(header.Header)
		cmds = append(cmds, cmd)
		u, cmd = p.sidebar.Update(msg)
		p.sidebar = u.(sidebar.Sidebar)
		cmds = append(cmds, cmd)
		return p, tea.Batch(cmds...)
	case chat.SessionClearedMsg:
		u, cmd := p.header.Update(msg)
		p.header = u.(header.Header)
		cmds = append(cmds, cmd)
		u, cmd = p.sidebar.Update(msg)
		p.sidebar = u.(sidebar.Sidebar)
		cmds = append(cmds, cmd)
		u, cmd = p.chat.Update(msg)
		p.chat = u.(chat.MessageListCmp)
		cmds = append(cmds, cmd)
		return p, tea.Batch(cmds...)
	case filepicker.FilePickedMsg,
		completions.CompletionsClosedMsg,
		completions.SelectCompletionMsg:
		u, cmd := p.editor.Update(msg)
		p.editor = u.(editor.Editor)
		cmds = append(cmds, cmd)
		return p, tea.Batch(cmds...)

	case pubsub.Event[message.Message],
		anim.StepMsg,
		spinner.TickMsg:
		// this needs to go to chat
		u, cmd := p.chat.Update(msg)
		p.chat = u.(chat.MessageListCmp)
		cmds = append(cmds, cmd)
		return p, tea.Batch(cmds...)

	case pubsub.Event[history.File], sidebar.SessionFilesMsg:
		// this needs to go to sidebar
		u, cmd := p.sidebar.Update(msg)
		p.sidebar = u.(sidebar.Sidebar)
		cmds = append(cmds, cmd)
		return p, tea.Batch(cmds...)

	case commands.CommandRunCustomMsg:
		// Check if the agent is busy before executing custom commands
		if p.app.CoderAgent.IsBusy() {
			return p, util.ReportWarn("Agent is busy, please wait before executing a command...")
		}

		// Handle custom command execution
		cmd := p.sendMessage(msg.Content, nil)
		if cmd != nil {
			return p, cmd
		}
	case splash.OnboardingCompleteMsg:
		p.state = ChatStateNewMessage
		err := p.app.InitCoderAgent()
		if err != nil {
			return p, util.ReportError(err)
		}
		p.focusedPane = PanelTypeEditor
		return p, p.SetSize(p.width, p.height)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, p.keyMap.NewSession):
			return p, p.newSession()
		case key.Matches(msg, p.keyMap.AddAttachment):
			agentCfg := config.Get().Agents["coder"]
			model := config.Get().GetModelByType(agentCfg.Model)
			if model.SupportsImages {
				return p, util.CmdHandler(OpenFilePickerMsg{})
			} else {
				return p, util.ReportWarn("File attachments are not supported by the current model: " + model.Name)
			}
		case key.Matches(msg, p.keyMap.Tab):
			if p.state == ChatStateOnboarding || p.state == ChatStateInitProject {
				u, cmd := p.splash.Update(msg)
				p.splash = u.(splash.Splash)
				return p, cmd
			}
			p.changeFocus()
			return p, nil
		case key.Matches(msg, p.keyMap.Cancel):
			return p, p.cancel()
		case key.Matches(msg, p.keyMap.Details):
			p.showDetails()
			return p, nil
		}

		// Send the key press to the focused pane
		switch p.focusedPane {
		case PanelTypeChat:
			u, cmd := p.chat.Update(msg)
			p.chat = u.(chat.MessageListCmp)
			cmds = append(cmds, cmd)
		case PanelTypeEditor:
			u, cmd := p.editor.Update(msg)
			p.editor = u.(editor.Editor)
			cmds = append(cmds, cmd)
		case PanelTypeSplash:
			u, cmd := p.splash.Update(msg)
			p.splash = u.(splash.Splash)
			cmds = append(cmds, cmd)
		}
	}
	return p, tea.Batch(cmds...)
}

func (p *chatPage) View() tea.View {
	var chatView tea.View
	t := styles.CurrentTheme()
	switch p.state {
	case ChatStateOnboarding, ChatStateInitProject:
		chatView = p.splash.View()
	case ChatStateNewMessage:
		editorView := p.editor.View()
		chatView = tea.NewView(
			lipgloss.JoinVertical(
				lipgloss.Left,
				t.S().Base.Render(
					p.splash.View().String(),
				),
				editorView.String(),
			),
		)
		chatView.SetCursor(editorView.Cursor())
	case ChatStateInSession:
		messagesView := p.chat.View()
		editorView := p.editor.View()
		if p.compact {
			headerView := p.header.View()
			chatView = tea.NewView(
				lipgloss.JoinVertical(
					lipgloss.Left,
					headerView.String(),
					messagesView.String(),
					editorView.String(),
				),
			)
			chatView.SetCursor(editorView.Cursor())
		} else {
			sidebarView := p.sidebar.View()
			messages := lipgloss.JoinHorizontal(
				lipgloss.Left,
				messagesView.String(),
				sidebarView.String(),
			)
			chatView = tea.NewView(
				lipgloss.JoinVertical(
					lipgloss.Left,
					messages,
					p.editor.View().String(),
				),
			)
			chatView.SetCursor(editorView.Cursor())
		}
	default:
		chatView = tea.NewView("Unknown chat state")
	}

	layers := []*lipgloss.Layer{
		lipgloss.NewLayer(chatView.String()).X(0).Y(0),
	}

	if p.showingDetails {
		style := t.S().Base.
			Width(p.detailsWidth).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderFocus)
		version := t.S().Subtle.Width(p.detailsWidth - 2).AlignHorizontal(lipgloss.Right).Render(version.Version)
		details := style.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				p.sidebar.View().String(),
				version,
			),
		)
		layers = append(layers, lipgloss.NewLayer(details).X(1).Y(1))
	}
	canvas := lipgloss.NewCanvas(
		layers...,
	)
	view := tea.NewView(canvas.Render())
	view.SetCursor(chatView.Cursor())
	return view
}

func (p *chatPage) updateCompactConfig(compact bool) tea.Cmd {
	return func() tea.Msg {
		err := config.Get().SetCompactMode(compact)
		if err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  "Failed to update compact mode configuration: " + err.Error(),
			}
		}
		return nil
	}
}

func (p *chatPage) setCompactMode(compact bool) {
	if p.compact == compact {
		return
	}
	p.compact = compact
	if compact {
		p.compact = true
		p.sidebar.SetCompactMode(true)
	} else {
		p.compact = false
		p.showingDetails = false
		p.sidebar.SetCompactMode(false)
	}
}

func (p *chatPage) handleCompactMode(newWidth int) {
	if p.forceCompact {
		return
	}
	if newWidth < CompactModeBreakpoint && !p.compact {
		p.setCompactMode(true)
	}
	if newWidth >= CompactModeBreakpoint && p.compact {
		p.setCompactMode(false)
	}
}

func (p *chatPage) SetSize(width, height int) tea.Cmd {
	p.handleCompactMode(width)
	p.width = width
	p.height = height
	var cmds []tea.Cmd
	switch p.state {
	case ChatStateOnboarding, ChatStateInitProject:
		// here we should just have the splash screen
		cmds = append(cmds, p.splash.SetSize(width, height))
	case ChatStateNewMessage:
		cmds = append(cmds, p.splash.SetSize(width, height-EditorHeight))
		cmds = append(cmds, p.editor.SetSize(width, EditorHeight))
		cmds = append(cmds, p.editor.SetPosition(0, height-EditorHeight))
	case ChatStateInSession:
		if p.compact {
			cmds = append(cmds, p.chat.SetSize(width, height-EditorHeight-HeaderHeight))
			// In compact mode, the sidebar is shown in the details section, the width needs to be adjusted for the padding and border
			p.detailsWidth = width - 2                                                  // because of position
			cmds = append(cmds, p.sidebar.SetSize(p.detailsWidth-2, p.detailsHeight-2)) // adjust for border
			cmds = append(cmds, p.editor.SetSize(width, EditorHeight))
			cmds = append(cmds, p.header.SetWidth(width-1))
		} else {
			cmds = append(cmds, p.chat.SetSize(width-SideBarWidth, height-EditorHeight))
			cmds = append(cmds, p.editor.SetSize(width, EditorHeight))
			cmds = append(cmds, p.sidebar.SetSize(SideBarWidth, height-EditorHeight))
		}
		cmds = append(cmds, p.editor.SetPosition(0, height-EditorHeight))
	}
	return tea.Batch(cmds...)
}

func (p *chatPage) newSession() tea.Cmd {
	if p.state != ChatStateInSession {
		// Cannot start a new session if we are not in the session state
		return nil
	}

	// blank session
	p.session = session.Session{}
	p.state = ChatStateNewMessage
	p.focusedPane = PanelTypeEditor
	p.canceling = false
	// Reset the chat and editor components
	return tea.Batch(
		util.CmdHandler(chat.SessionClearedMsg{}),
		p.SetSize(p.width, p.height),
	)
}

func (p *chatPage) setSession(session session.Session) tea.Cmd {
	if p.session.ID == session.ID {
		return nil
	}

	var cmds []tea.Cmd
	p.session = session
	// We want to first resize the components
	if p.state != ChatStateInSession {
		p.state = ChatStateInSession
		cmds = append(cmds, p.SetSize(p.width, p.height))
	}
	cmds = append(cmds, p.chat.SetSession(session))
	cmds = append(cmds, p.sidebar.SetSession(session))
	cmds = append(cmds, p.header.SetSession(session))
	cmds = append(cmds, p.editor.SetSession(session))

	return tea.Sequence(cmds...)
}

func (p *chatPage) changeFocus() {
	if p.state != ChatStateInSession {
		// Cannot change focus if we are not in the session state
		return
	}
	switch p.focusedPane {
	case PanelTypeChat:
		p.focusedPane = PanelTypeEditor
		p.editor.Focus()
		p.chat.Blur()
	case PanelTypeEditor:
		p.focusedPane = PanelTypeChat
		p.chat.Focus()
		p.editor.Blur()
	}
}

func (p *chatPage) cancel() tea.Cmd {
	if p.state != ChatStateInSession || !p.app.CoderAgent.IsBusy() {
		// Cannot cancel if we are not in the session state
		return nil
	}

	// second press of cancel key will actually cancel the session
	if p.canceling {
		p.canceling = false
		p.app.CoderAgent.Cancel(p.session.ID)
		return nil
	}

	p.canceling = true
	return cancelTimerCmd()
}

func (p *chatPage) showDetails() {
	if p.state != ChatStateInSession || !p.compact {
		// Cannot show details if we are not in the session state or if we are not in compact mode
		return
	}
	p.showingDetails = !p.showingDetails
	p.header.SetDetailsOpen(p.showingDetails)
}

func (p *chatPage) sendMessage(text string, attachments []message.Attachment) tea.Cmd {
	session := p.session
	var cmds []tea.Cmd
	if p.state != ChatStateInSession {
		// branch new session
		newSession, err := p.app.Sessions.Create(context.Background(), "New Session")
		if err != nil {
			return util.ReportError(err)
		}
		session = newSession
		cmds = append(cmds, util.CmdHandler(chat.SessionSelectedMsg(session)))
	}
	_, err := p.app.CoderAgent.Run(context.Background(), session.ID, text, attachments...)
	if err != nil {
		return util.ReportError(err)
	}
	return tea.Batch(cmds...)
}

func (p *chatPage) Bindings() []key.Binding {
	bindings := []key.Binding{
		p.keyMap.NewSession,
		p.keyMap.AddAttachment,
	}
	if p.app.CoderAgent != nil && p.app.CoderAgent.IsBusy() {
		cancelBinding := p.keyMap.Cancel
		if p.canceling {
			cancelBinding = key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "press again to cancel"),
			)
		}
		bindings = append([]key.Binding{cancelBinding}, bindings...)
	}

	switch p.focusedPane {
	case PanelTypeChat:
		bindings = append([]key.Binding{
			key.NewBinding(
				key.WithKeys("tab"),
				key.WithHelp("tab", "focus editor"),
			),
		}, bindings...)
		bindings = append(bindings, p.chat.Bindings()...)
	case PanelTypeEditor:
		bindings = append([]key.Binding{
			key.NewBinding(
				key.WithKeys("tab"),
				key.WithHelp("tab", "focus chat"),
			),
		}, bindings...)
		bindings = append(bindings, p.editor.Bindings()...)
	case PanelTypeSplash:
		bindings = append(bindings, p.splash.Bindings()...)
	}

	return bindings
}
