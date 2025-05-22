package chat

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/components/chat/messages"
	"github.com/opencode-ai/opencode/internal/tui/components/core/list"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type MessageListCmp interface {
	util.Model
	layout.Sizeable
}

type messageListCmp struct {
	app           *app.App
	width, height int
	session       session.Session
	messages      []util.Model
	listCmp       list.ListModel
}

func NewMessagesListCmp(app *app.App) MessageListCmp {
	return &messageListCmp{
		app: app,
		listCmp: list.New(
			list.WithGapSize(1),
			list.WithReverse(true),
		),
	}
}

func (m *messageListCmp) Init() tea.Cmd {
	return nil
}

func (m *messageListCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case dialog.ThemeChangedMsg:
		m.listCmp.ResetView()
		return m, nil
	case SessionSelectedMsg:
		if msg.ID != m.session.ID {
			cmd := m.SetSession(msg)
			return m, cmd
		}
		return m, nil
	default:
		var cmds []tea.Cmd
		u, cmd := m.listCmp.Update(msg)
		m.listCmp = u.(list.ListModel)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}
}

func (m *messageListCmp) View() string {
	return lipgloss.JoinVertical(lipgloss.Left, m.listCmp.View())
}

// GetSize implements MessageListCmp.
func (m *messageListCmp) GetSize() (int, int) {
	return m.width, m.height
}

// SetSize implements MessageListCmp.
func (m *messageListCmp) SetSize(width int, height int) tea.Cmd {
	m.width = width
	m.height = height - 1
	return m.listCmp.SetSize(width, height-1)
}

func (m *messageListCmp) SetSession(session session.Session) tea.Cmd {
	if m.session.ID == session.ID {
		return nil
	}
	m.session = session
	sessionMessages, err := m.app.Messages.List(context.Background(), session.ID)
	if err != nil {
		return util.ReportError(err)
	}
	m.messages = make([]util.Model, 0)
	lastUserMessageTime := sessionMessages[0].CreatedAt
	toolResultMap := make(map[string]message.ToolResult)
	// first pass to get all tool results
	for _, msg := range sessionMessages {
		for _, tr := range msg.ToolResults() {
			toolResultMap[tr.ToolCallID] = tr
		}
	}
	for _, msg := range sessionMessages {
		switch msg.Role {
		case message.User:
			lastUserMessageTime = msg.CreatedAt
			m.messages = append(m.messages, messages.NewMessageCmp(msg))
		case message.Assistant:
			// Only add assistant messages if they don't have tool calls or there is some content
			if len(msg.ToolCalls()) == 0 || msg.Content().Text != "" || msg.IsThinking() {
				m.messages = append(m.messages, messages.NewMessageCmp(msg, messages.WithLastUserMessageTime(time.Unix(lastUserMessageTime, 0))))
			}
			for _, tc := range msg.ToolCalls() {
				options := []messages.ToolCallOption{}
				if tr, ok := toolResultMap[tc.ID]; ok {
					options = append(options, messages.WithToolCallResult(tr))
				}
				if msg.FinishPart().Reason == message.FinishReasonCanceled {
					options = append(options, messages.WithToolCallCancelled())
				}
				m.messages = append(m.messages, messages.NewToolCallCmp(tc, options...))
			}
		}
	}
	m.listCmp.SetItems(m.messages)
	return nil
}
