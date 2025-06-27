package chat

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/chat/editor"
	"github.com/charmbracelet/crush/internal/tui/components/chat/header"
	"github.com/charmbracelet/crush/internal/tui/components/chat/sidebar"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/commands"
	"github.com/charmbracelet/crush/internal/tui/page"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/charmbracelet/lipgloss/v2"
)

var ChatPageID page.PageID = "chat"

const CompactModeBreakpoint = 120 // Width at which the chat page switches to compact mode

type (
	OpenFilePickerMsg struct{}
	ChatFocusedMsg    struct {
		Focused bool // True if the chat input is focused, false otherwise
	}
	CancelTimerExpiredMsg struct{}
)

type ChatPage interface {
	util.Model
	layout.Help
}

type chatPage struct {
	wWidth, wHeight int // Window dimensions
	app             *app.App

	layout layout.SplitPaneLayout

	session session.Session

	keyMap KeyMap

	chatFocused bool

	compactMode      bool
	forceCompactMode bool // Force compact mode regardless of window size
	showDetails      bool // Show details in the header
	header           header.Header
	compactSidebar   layout.Container

	cancelPending bool // True if ESC was pressed once and waiting for second press
}

func (p *chatPage) Init() tea.Cmd {
	return tea.Batch(
		p.layout.Init(),
		p.compactSidebar.Init(),
		p.layout.FocusPanel(layout.BottomPanel), // Focus on the bottom panel (editor),
	)
}

// cancelTimerCmd creates a command that expires the cancel timer after 2 seconds
func (p *chatPage) cancelTimerCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return CancelTimerExpiredMsg{}
	})
}

func (p *chatPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case CancelTimerExpiredMsg:
		p.cancelPending = false
		return p, nil
	case tea.WindowSizeMsg:
		h, cmd := p.header.Update(msg)
		cmds = append(cmds, cmd)
		p.header = h.(header.Header)
		cmds = append(cmds, p.compactSidebar.SetSize(msg.Width-4, 0))
		// the mode is only relevant when there is a  session
		if p.session.ID != "" {
			// Only auto-switch to compact mode if not forced
			if !p.forceCompactMode {
				if msg.Width <= CompactModeBreakpoint && p.wWidth > CompactModeBreakpoint {
					p.wWidth = msg.Width
					p.wHeight = msg.Height
					cmds = append(cmds, p.setCompactMode(true))
					return p, tea.Batch(cmds...)
				} else if msg.Width > CompactModeBreakpoint && p.wWidth <= CompactModeBreakpoint {
					p.wWidth = msg.Width
					p.wHeight = msg.Height
					return p, p.setCompactMode(false)
				}
			}
		}
		p.wWidth = msg.Width
		p.wHeight = msg.Height
		layoutHeight := msg.Height
		if p.compactMode {
			// make space for the header
			layoutHeight -= 1
		}
		cmd = p.layout.SetSize(msg.Width, layoutHeight)
		cmds = append(cmds, cmd)
		return p, tea.Batch(cmds...)

	case chat.SendMsg:
		cmd := p.sendMessage(msg.Text, msg.Attachments)
		if cmd != nil {
			return p, cmd
		}
	case commands.ToggleCompactModeMsg:
		// Only allow toggling if window width is larger than compact breakpoint
		if p.wWidth > CompactModeBreakpoint {
			p.forceCompactMode = !p.forceCompactMode
			// If force compact mode is enabled, switch to compact mode
			// If force compact mode is disabled, switch based on window size
			if p.forceCompactMode {
				return p, p.setCompactMode(true)
			} else {
				// Return to auto mode based on window size
				shouldBeCompact := p.wWidth <= CompactModeBreakpoint
				return p, p.setCompactMode(shouldBeCompact)
			}
		}
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
	case chat.SessionSelectedMsg:
		if p.session.ID == "" {
			cmd := p.setMessages()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		needsModeChange := p.session.ID == ""
		p.session = msg
		p.header.SetSession(msg)
		if needsModeChange && (p.wWidth <= CompactModeBreakpoint || p.forceCompactMode) {
			cmds = append(cmds, p.setCompactMode(true))
		}
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, p.keyMap.NewSession):
			p.session = session.Session{}
			return p, tea.Batch(
				p.clearMessages(),
				util.CmdHandler(chat.SessionClearedMsg{}),
				p.setCompactMode(false),
				p.layout.FocusPanel(layout.BottomPanel),
				util.CmdHandler(ChatFocusedMsg{Focused: false}),
			)
		case key.Matches(msg, p.keyMap.AddAttachment):
			model := config.GetAgentModel(config.AgentCoder)
			if model.SupportsImages {
				return p, util.CmdHandler(OpenFilePickerMsg{})
			} else {
				return p, util.ReportWarn("File attachments are not supported by the current model: " + model.Name)
			}
		case key.Matches(msg, p.keyMap.Tab):
			if p.session.ID == "" {
				return p, nil
			}
			p.chatFocused = !p.chatFocused
			if p.chatFocused {
				cmds = append(cmds, p.layout.FocusPanel(layout.LeftPanel))
				cmds = append(cmds, util.CmdHandler(ChatFocusedMsg{Focused: true}))
			} else {
				cmds = append(cmds, p.layout.FocusPanel(layout.BottomPanel))
				cmds = append(cmds, util.CmdHandler(ChatFocusedMsg{Focused: false}))
			}
			return p, tea.Batch(cmds...)
		case key.Matches(msg, p.keyMap.Cancel):
			if p.session.ID != "" {
				if p.cancelPending {
					// Second ESC press - actually cancel the session
					p.cancelPending = false
					p.app.CoderAgent.Cancel(p.session.ID)
					return p, nil
				} else {
					// First ESC press - start the timer
					p.cancelPending = true
					return p, p.cancelTimerCmd()
				}
			}
		case key.Matches(msg, p.keyMap.Details):
			if p.session.ID == "" || !p.compactMode {
				return p, nil // No session to show details for
			}
			p.showDetails = !p.showDetails
			p.header.SetDetailsOpen(p.showDetails)
			if p.showDetails {
				return p, tea.Batch()
			}

			return p, nil
		}
	}
	u, cmd := p.layout.Update(msg)
	cmds = append(cmds, cmd)
	p.layout = u.(layout.SplitPaneLayout)
	h, cmd := p.header.Update(msg)
	p.header = h.(header.Header)
	cmds = append(cmds, cmd)
	s, cmd := p.compactSidebar.Update(msg)
	p.compactSidebar = s.(layout.Container)
	cmds = append(cmds, cmd)
	return p, tea.Batch(cmds...)
}

func (p *chatPage) setMessages() tea.Cmd {
	messagesContainer := layout.NewContainer(
		chat.NewMessagesListCmp(p.app),
		layout.WithPadding(1, 1, 0, 1),
	)
	return tea.Batch(p.layout.SetLeftPanel(messagesContainer), messagesContainer.Init())
}

func (p *chatPage) setSidebar() tea.Cmd {
	sidebarContainer := sidebarCmp(p.app, false, p.session)
	sidebarContainer.Init()
	return p.layout.SetRightPanel(sidebarContainer)
}

func (p *chatPage) clearMessages() tea.Cmd {
	return p.layout.ClearLeftPanel()
}

func (p *chatPage) setCompactMode(compact bool) tea.Cmd {
	p.compactMode = compact
	var cmds []tea.Cmd
	if compact {
		// add offset for the header
		p.layout.SetOffset(0, 1)
		// make space for the header
		cmds = append(cmds, p.layout.SetSize(p.wWidth, p.wHeight-1))
		// remove the sidebar
		cmds = append(cmds, p.layout.ClearRightPanel())
		return tea.Batch(cmds...)
	} else {
		// remove the offset for the header
		p.layout.SetOffset(0, 0)
		// restore the original size
		cmds = append(cmds, p.layout.SetSize(p.wWidth, p.wHeight))
		// set the sidebar
		cmds = append(cmds, p.setSidebar())
		l, cmd := p.layout.Update(chat.SessionSelectedMsg(p.session))
		p.layout = l.(layout.SplitPaneLayout)
		cmds = append(cmds, cmd)

		return tea.Batch(cmds...)
	}
}

func (p *chatPage) sendMessage(text string, attachments []message.Attachment) tea.Cmd {
	var cmds []tea.Cmd
	if p.session.ID == "" {
		session, err := p.app.Sessions.Create(context.Background(), "New Session")
		if err != nil {
			return util.ReportError(err)
		}

		p.session = session
		cmd := p.setMessages()
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, util.CmdHandler(chat.SessionSelectedMsg(session)))
	}

	_, err := p.app.CoderAgent.Run(context.Background(), p.session.ID, text, attachments...)
	if err != nil {
		return util.ReportError(err)
	}
	return tea.Batch(cmds...)
}

func (p *chatPage) SetSize(width, height int) tea.Cmd {
	return p.layout.SetSize(width, height)
}

func (p *chatPage) GetSize() (int, int) {
	return p.layout.GetSize()
}

func (p *chatPage) View() tea.View {
	if !p.compactMode || p.session.ID == "" {
		// If not in compact mode or there is no session, we don't show the header
		return p.layout.View()
	}
	layoutView := p.layout.View()
	chatView := strings.Join(
		[]string{
			p.header.View().String(),
			layoutView.String(),
		}, "\n",
	)
	layers := []*lipgloss.Layer{
		lipgloss.NewLayer(chatView).X(0).Y(0),
	}
	if p.showDetails {
		t := styles.CurrentTheme()
		style := t.S().Base.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderFocus)
		version := t.S().Subtle.Padding(0, 1).AlignHorizontal(lipgloss.Right).Width(p.wWidth - 4).Render(version.Version)
		details := style.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				p.compactSidebar.View().String(),
				version,
			),
		)
		layers = append(layers, lipgloss.NewLayer(details).X(1).Y(1))
	}
	canvas := lipgloss.NewCanvas(
		layers...,
	)
	view := tea.NewView(canvas.Render())
	view.SetCursor(layoutView.Cursor())
	return view
}

func (p *chatPage) Bindings() []key.Binding {
	bindings := []key.Binding{
		p.keyMap.NewSession,
		p.keyMap.AddAttachment,
	}
	if p.app.CoderAgent.IsBusy() {
		cancelBinding := p.keyMap.Cancel
		if p.cancelPending {
			cancelBinding = key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "press again to cancel"),
			)
		}
		bindings = append([]key.Binding{cancelBinding}, bindings...)
	}

	if p.chatFocused {
		bindings = append([]key.Binding{
			key.NewBinding(
				key.WithKeys("tab"),
				key.WithHelp("tab", "focus editor"),
			),
		}, bindings...)
	} else {
		bindings = append([]key.Binding{
			key.NewBinding(
				key.WithKeys("tab"),
				key.WithHelp("tab", "focus chat"),
			),
		}, bindings...)
	}

	bindings = append(bindings, p.layout.Bindings()...)
	return bindings
}

func sidebarCmp(app *app.App, compact bool, session session.Session) layout.Container {
	padding := layout.WithPadding(1, 1, 1, 1)
	if compact {
		padding = layout.WithPadding(0, 1, 1, 1)
	}
	sidebar := sidebar.NewSidebarCmp(app.History, app.LSPClients, compact)
	if session.ID != "" {
		sidebar.SetSession(session)
	}

	return layout.NewContainer(
		sidebar,
		padding,
	)
}

func NewChatPage(app *app.App) ChatPage {
	editorContainer := layout.NewContainer(
		editor.NewEditorCmp(app),
	)
	return &chatPage{
		app: app,
		layout: layout.NewSplitPane(
			layout.WithRightPanel(sidebarCmp(app, false, session.Session{})),
			layout.WithBottomPanel(editorContainer),
			layout.WithFixedBottomHeight(5),
			layout.WithFixedRightWidth(31),
		),
		compactSidebar: sidebarCmp(app, true, session.Session{}),
		keyMap:         DefaultKeyMap(),
		header:         header.New(app.LSPClients),
	}
}
