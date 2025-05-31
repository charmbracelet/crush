package chat

import (
	"context"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/components/chat"
	"github.com/opencode-ai/opencode/internal/tui/components/chat/editor"
	"github.com/opencode-ai/opencode/internal/tui/components/chat/sidebar"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs/commands"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/page"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

var ChatPage page.PageID = "chat"

type chatPage struct {
	app *app.App

	layout layout.SplitPaneLayout

	session session.Session
}

type ChatKeyMap struct {
	NewSession key.Binding
	Cancel     key.Binding
}

var keyMap = ChatKeyMap{
	NewSession: key.NewBinding(
		key.WithKeys("ctrl+n"),
		key.WithHelp("ctrl+n", "new session"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
}

func (p *chatPage) Init() tea.Cmd {
	return p.layout.Init()
}

func (p *chatPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		cmd := p.layout.SetSize(msg.Width, msg.Height)
		cmds = append(cmds, cmd)
	case chat.SendMsg:
		cmd := p.sendMessage(msg.Text, msg.Attachments)
		if cmd != nil {
			return p, cmd
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
		p.session = msg
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keyMap.NewSession):
			p.session = session.Session{}
			return p, tea.Batch(
				p.clearMessages(),
				util.CmdHandler(chat.SessionClearedMsg{}),
			)
		case key.Matches(msg, keyMap.Cancel):
			if p.session.ID != "" {
				// Cancel the current session's generation process
				// This allows users to interrupt long-running operations
				p.app.CoderAgent.Cancel(p.session.ID)
				return p, nil
			}
		}
	}
	u, cmd := p.layout.Update(msg)
	cmds = append(cmds, cmd)
	p.layout = u.(layout.SplitPaneLayout)

	return p, tea.Batch(cmds...)
}

func (p *chatPage) setMessages() tea.Cmd {
	messagesContainer := layout.NewContainer(
		chat.NewMessagesListCmp(p.app),
		layout.WithPadding(1, 1, 0, 1),
	)
	return tea.Batch(p.layout.SetLeftPanel(messagesContainer), messagesContainer.Init())
}

func (p *chatPage) clearMessages() tea.Cmd {
	return p.layout.ClearLeftPanel()
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
	return p.layout.View()
}

func (p *chatPage) BindingKeys() []key.Binding {
	bindings := layout.KeyMapToSlice(keyMap)
	return bindings
}

func NewChatPage(app *app.App) util.Model {
	sidebarContainer := layout.NewContainer(
		sidebar.NewSidebarCmp(),
		layout.WithPadding(1, 1, 1, 1),
	)
	editorContainer := layout.NewContainer(
		editor.NewEditorCmp(app),
	)
	return &chatPage{
		app: app,
		layout: layout.NewSplitPane(
			layout.WithRightPanel(sidebarContainer),
			layout.WithBottomPanel(editorContainer),
			layout.WithFixedBottomHeight(3),
			layout.WithFixedRightWidth(31),
		),
	}
}
