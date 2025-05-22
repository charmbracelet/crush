package chat

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/pubsub"
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
	listCmp       list.ListModel

	lastUserMessageTime int64
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
	case SessionClearedMsg:
		m.session = session.Session{}
		return m, m.listCmp.SetItems([]util.Model{})

	case pubsub.Event[message.Message]:
		return m, m.handleMessageEvent(msg)
	default:
		var cmds []tea.Cmd
		u, cmd := m.listCmp.Update(msg)
		m.listCmp = u.(list.ListModel)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}
}

func (m *messageListCmp) View() string {
	if len(m.listCmp.Items()) == 0 {
		return initialScreen()
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.listCmp.View())
}

func (m *messageListCmp) handleChildSession(event pubsub.Event[message.Message]) {
	// TODO: update the agent tool message with the changes
}

func (m *messageListCmp) handleMessageEvent(event pubsub.Event[message.Message]) tea.Cmd {
	switch event.Type {
	case pubsub.CreatedEvent:
		if event.Payload.SessionID != m.session.ID {
			m.handleChildSession(event)
		}
		messageExists := false
		// more likely to be at the end of the list
		items := m.listCmp.Items()
		for i := len(items) - 1; i >= 0; i-- {
			msg := items[i].(messages.MessageCmp)
			if msg.GetMessage().ID == event.Payload.ID {
				messageExists = true
				break
			}
		}
		if messageExists {
			return nil
		}
		switch event.Payload.Role {
		case message.User:
			return m.handleNewUserMessage(event.Payload)
		case message.Assistant:
			return m.handleNewAssistantMessage(event.Payload)
		}
		// TODO: handle tools
	case pubsub.UpdatedEvent:
		return m.handleUpdateAssistantMessage(event.Payload)
	}
	return nil
}

func (m *messageListCmp) handleNewUserMessage(msg message.Message) tea.Cmd {
	m.lastUserMessageTime = msg.CreatedAt
	return m.listCmp.AppendItem(messages.NewMessageCmp(msg))
}

func (m *messageListCmp) handleUpdateAssistantMessage(msg message.Message) tea.Cmd {
	// Simple update the content
	items := m.listCmp.Items()
	lastItem := items[len(items)-1].(messages.MessageCmp)
	// TODO:handle tool calls
	if lastItem.GetMessage().ID != msg.ID {
		return nil
	}
	// for now just updet the last message
	if len(msg.ToolCalls()) == 0 || msg.Content().Text != "" || msg.IsThinking() {
		m.listCmp.UpdateItem(
			len(items)-1,
			messages.NewMessageCmp(
				msg,
				messages.WithLastUserMessageTime(time.Unix(m.lastUserMessageTime, 0)),
			),
		)
	}
	return nil
}

func (m *messageListCmp) handleNewAssistantMessage(msg message.Message) tea.Cmd {
	var cmds []tea.Cmd
	// Only add assistant messages if they don't have tool calls or there is some content
	if len(msg.ToolCalls()) == 0 || msg.Content().Text != "" || msg.IsThinking() {
		cmd := m.listCmp.AppendItem(
			messages.NewMessageCmp(
				msg,
				messages.WithLastUserMessageTime(time.Unix(m.lastUserMessageTime, 0)),
			),
		)
		cmds = append(cmds, cmd)
	}
	for _, tc := range msg.ToolCalls() {
		cmd := m.listCmp.AppendItem(messages.NewToolCallCmp(tc))
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
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
	uiMessages := make([]util.Model, 0)
	m.lastUserMessageTime = sessionMessages[0].CreatedAt
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
			m.lastUserMessageTime = msg.CreatedAt
			uiMessages = append(uiMessages, messages.NewMessageCmp(msg))
		case message.Assistant:
			// Only add assistant messages if they don't have tool calls or there is some content
			if len(msg.ToolCalls()) == 0 || msg.Content().Text != "" || msg.IsThinking() {
				uiMessages = append(
					uiMessages,
					messages.NewMessageCmp(
						msg,
						messages.WithLastUserMessageTime(time.Unix(m.lastUserMessageTime, 0)),
					),
				)
			}
			for _, tc := range msg.ToolCalls() {
				options := []messages.ToolCallOption{}
				if tr, ok := toolResultMap[tc.ID]; ok {
					options = append(options, messages.WithToolCallResult(tr))
				}
				if msg.FinishPart().Reason == message.FinishReasonCanceled {
					options = append(options, messages.WithToolCallCancelled())
				}
				uiMessages = append(uiMessages, messages.NewToolCallCmp(tc, options...))
			}
		}
	}
	return m.listCmp.SetItems(uiMessages)
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
