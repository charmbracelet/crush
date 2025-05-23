package chat

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/components/chat/messages"
	"github.com/opencode-ai/opencode/internal/tui/components/core/list"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

const (
	NotFound = -1
)

// MessageListCmp represents a component that displays a list of chat messages
// with support for real-time updates and session management.
type MessageListCmp interface {
	util.Model
	layout.Sizeable
}

// messageListCmp implements MessageListCmp, providing a virtualized list
// of chat messages with support for tool calls, real-time updates, and
// session switching.
type messageListCmp struct {
	app           *app.App
	width, height int
	session       session.Session
	listCmp       list.ListModel

	lastUserMessageTime int64
}

// NewMessagesListCmp creates a new message list component with custom keybindings
// and reverse ordering (newest messages at bottom).
func NewMessagesListCmp(app *app.App) MessageListCmp {
	defaultKeymaps := list.DefaultKeymap()
	defaultKeymaps.NDown.SetEnabled(false)
	defaultKeymaps.NUp.SetEnabled(false)
	defaultKeymaps.Home = key.NewBinding(
		key.WithKeys("ctrl+g"),
	)
	defaultKeymaps.End = key.NewBinding(
		key.WithKeys("ctrl+G"),
	)
	return &messageListCmp{
		app: app,
		listCmp: list.New(
			list.WithGapSize(1),
			list.WithReverse(true),
			list.WithKeyMap(defaultKeymaps),
		),
	}
}

// Init initializes the component (no initialization needed).
func (m *messageListCmp) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and updates the component state.
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
		cmd := m.handleMessageEvent(msg)
		return m, cmd
	default:
		var cmds []tea.Cmd
		u, cmd := m.listCmp.Update(msg)
		m.listCmp = u.(list.ListModel)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}
}

// View renders the message list or an initial screen if empty.
func (m *messageListCmp) View() string {
	if len(m.listCmp.Items()) == 0 {
		return initialScreen()
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.listCmp.View())
}

// handleChildSession handles messages from child sessions (agent tools).
// TODO: update the agent tool message with the changes
func (m *messageListCmp) handleChildSession(event pubsub.Event[message.Message]) {
	// Implementation pending
}

// handleMessageEvent processes different types of message events (created/updated).
func (m *messageListCmp) handleMessageEvent(event pubsub.Event[message.Message]) tea.Cmd {
	switch event.Type {
	case pubsub.CreatedEvent:
		if event.Payload.SessionID != m.session.ID {
			m.handleChildSession(event)
			return nil
		}
		
		if m.messageExists(event.Payload.ID) {
			return nil
		}
		
		return m.handleNewMessage(event.Payload)
	case pubsub.UpdatedEvent:
		return m.handleUpdateAssistantMessage(event.Payload)
	}
	return nil
}

// messageExists checks if a message with the given ID already exists in the list.
func (m *messageListCmp) messageExists(messageID string) bool {
	items := m.listCmp.Items()
	// Search backwards as new messages are more likely to be at the end
	for i := len(items) - 1; i >= 0; i-- {
		if msg, ok := items[i].(messages.MessageCmp); ok && msg.GetMessage().ID == messageID {
			return true
		}
	}
	return false
}

// handleNewMessage routes new messages to appropriate handlers based on role.
func (m *messageListCmp) handleNewMessage(msg message.Message) tea.Cmd {
	switch msg.Role {
	case message.User:
		return m.handleNewUserMessage(msg)
	case message.Assistant:
		return m.handleNewAssistantMessage(msg)
	case message.Tool:
		return m.handleToolMessage(msg)
	}
	return nil
}

// handleNewUserMessage adds a new user message to the list and updates the timestamp.
func (m *messageListCmp) handleNewUserMessage(msg message.Message) tea.Cmd {
	m.lastUserMessageTime = msg.CreatedAt
	return m.listCmp.AppendItem(messages.NewMessageCmp(msg))
}

// handleToolMessage updates existing tool calls with their results.
func (m *messageListCmp) handleToolMessage(msg message.Message) tea.Cmd {
	items := m.listCmp.Items()
	for _, tr := range msg.ToolResults() {
		if toolCallIndex := m.findToolCallByID(items, tr.ToolCallID); toolCallIndex != NotFound {
			toolCall := items[toolCallIndex].(messages.ToolCallCmp)
			toolCall.SetToolResult(tr)
			m.listCmp.UpdateItem(toolCallIndex, toolCall)
		}
	}
	return nil
}

// findToolCallByID searches for a tool call with the specified ID.
// Returns the index if found, NotFound otherwise.
func (m *messageListCmp) findToolCallByID(items []util.Model, toolCallID string) int {
	// Search backwards as tool calls are more likely to be recent
	for i := len(items) - 1; i >= 0; i-- {
		if toolCall, ok := items[i].(messages.ToolCallCmp); ok && toolCall.GetToolCall().ID == toolCallID {
			return i
		}
	}
	return NotFound
}

// handleUpdateAssistantMessage processes updates to assistant messages,
// managing both message content and associated tool calls.
func (m *messageListCmp) handleUpdateAssistantMessage(msg message.Message) tea.Cmd {
	var cmds []tea.Cmd
	items := m.listCmp.Items()
	
	// Find existing assistant message and tool calls for this message
	assistantIndex, existingToolCalls := m.findAssistantMessageAndToolCalls(items, msg.ID)
	
	logging.Info("Update Assistant Message", "msg", msg, "assistantMessageInx", assistantIndex, "toolCalls", existingToolCalls)
	
	// Handle assistant message content
	if cmd := m.updateAssistantMessageContent(msg, assistantIndex); cmd != nil {
		cmds = append(cmds, cmd)
	}
	
	// Handle tool calls
	if cmd := m.updateToolCalls(msg, existingToolCalls); cmd != nil {
		cmds = append(cmds, cmd)
	}
	
	return tea.Batch(cmds...)
}

// findAssistantMessageAndToolCalls locates the assistant message and its tool calls.
func (m *messageListCmp) findAssistantMessageAndToolCalls(items []util.Model, messageID string) (int, map[int]messages.ToolCallCmp) {
	assistantIndex := NotFound
	toolCalls := make(map[int]messages.ToolCallCmp)
	
	// Search backwards as messages are more likely to be at the end
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if asMsg, ok := item.(messages.MessageCmp); ok {
			if asMsg.GetMessage().ID == messageID {
				assistantIndex = i
			}
		} else if tc, ok := item.(messages.ToolCallCmp); ok {
			if tc.ParentMessageId() == messageID {
				toolCalls[i] = tc
			}
		}
	}
	
	return assistantIndex, toolCalls
}

// updateAssistantMessageContent updates or removes the assistant message based on content.
func (m *messageListCmp) updateAssistantMessageContent(msg message.Message, assistantIndex int) tea.Cmd {
	if assistantIndex == NotFound {
		return nil
	}
	
	shouldShowMessage := m.shouldShowAssistantMessage(msg)
	hasToolCallsOnly := len(msg.ToolCalls()) > 0 && msg.Content().Text == ""
	
	if shouldShowMessage {
		m.listCmp.UpdateItem(
			assistantIndex,
			messages.NewMessageCmp(
				msg,
				messages.WithLastUserMessageTime(time.Unix(m.lastUserMessageTime, 0)),
			),
		)
	} else if hasToolCallsOnly {
		m.listCmp.DeleteItem(assistantIndex)
	}
	
	return nil
}

// shouldShowAssistantMessage determines if an assistant message should be displayed.
func (m *messageListCmp) shouldShowAssistantMessage(msg message.Message) bool {
	return len(msg.ToolCalls()) == 0 || msg.Content().Text != "" || msg.IsThinking()
}

// updateToolCalls handles updates to tool calls, updating existing ones and adding new ones.
func (m *messageListCmp) updateToolCalls(msg message.Message, existingToolCalls map[int]messages.ToolCallCmp) tea.Cmd {
	var cmds []tea.Cmd
	
	for _, tc := range msg.ToolCalls() {
		if cmd := m.updateOrAddToolCall(tc, existingToolCalls, msg.ID); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	
	return tea.Batch(cmds...)
}

// updateOrAddToolCall updates an existing tool call or adds a new one.
func (m *messageListCmp) updateOrAddToolCall(tc message.ToolCall, existingToolCalls map[int]messages.ToolCallCmp, messageID string) tea.Cmd {
	// Try to find existing tool call
	for index, existingTC := range existingToolCalls {
		if tc.ID == existingTC.GetToolCall().ID {
			existingTC.SetToolCall(tc)
			m.listCmp.UpdateItem(index, existingTC)
			return nil
		}
	}
	
	// Add new tool call if not found
	return m.listCmp.AppendItem(messages.NewToolCallCmp(messageID, tc))
}

// handleNewAssistantMessage processes new assistant messages and their tool calls.
func (m *messageListCmp) handleNewAssistantMessage(msg message.Message) tea.Cmd {
	var cmds []tea.Cmd
	
	// Add assistant message if it should be displayed
	if m.shouldShowAssistantMessage(msg) {
		cmd := m.listCmp.AppendItem(
			messages.NewMessageCmp(
				msg,
				messages.WithLastUserMessageTime(time.Unix(m.lastUserMessageTime, 0)),
			),
		)
		cmds = append(cmds, cmd)
	}
	
	// Add tool calls
	for _, tc := range msg.ToolCalls() {
		cmd := m.listCmp.AppendItem(messages.NewToolCallCmp(msg.ID, tc))
		cmds = append(cmds, cmd)
	}
	
	return tea.Batch(cmds...)
}

// SetSession loads and displays messages for a new session.
func (m *messageListCmp) SetSession(session session.Session) tea.Cmd {
	if m.session.ID == session.ID {
		return nil
	}
	
	m.session = session
	sessionMessages, err := m.app.Messages.List(context.Background(), session.ID)
	if err != nil {
		return util.ReportError(err)
	}
	
	if len(sessionMessages) == 0 {
		return m.listCmp.SetItems([]util.Model{})
	}
	
	// Initialize with first message timestamp
	m.lastUserMessageTime = sessionMessages[0].CreatedAt
	
	// Build tool result map for efficient lookup
	toolResultMap := m.buildToolResultMap(sessionMessages)
	
	// Convert messages to UI components
	uiMessages := m.convertMessagesToUI(sessionMessages, toolResultMap)
	
	return m.listCmp.SetItems(uiMessages)
}

// buildToolResultMap creates a map of tool call ID to tool result for efficient lookup.
func (m *messageListCmp) buildToolResultMap(messages []message.Message) map[string]message.ToolResult {
	toolResultMap := make(map[string]message.ToolResult)
	for _, msg := range messages {
		for _, tr := range msg.ToolResults() {
			toolResultMap[tr.ToolCallID] = tr
		}
	}
	return toolResultMap
}

// convertMessagesToUI converts database messages to UI components.
func (m *messageListCmp) convertMessagesToUI(sessionMessages []message.Message, toolResultMap map[string]message.ToolResult) []util.Model {
	uiMessages := make([]util.Model, 0)
	
	for _, msg := range sessionMessages {
		switch msg.Role {
		case message.User:
			m.lastUserMessageTime = msg.CreatedAt
			uiMessages = append(uiMessages, messages.NewMessageCmp(msg))
		case message.Assistant:
			uiMessages = append(uiMessages, m.convertAssistantMessage(msg, toolResultMap)...)
		}
	}
	
	return uiMessages
}

// convertAssistantMessage converts an assistant message and its tool calls to UI components.
func (m *messageListCmp) convertAssistantMessage(msg message.Message, toolResultMap map[string]message.ToolResult) []util.Model {
	var uiMessages []util.Model
	
	// Add assistant message if it should be displayed
	if m.shouldShowAssistantMessage(msg) {
		uiMessages = append(
			uiMessages,
			messages.NewMessageCmp(
				msg,
				messages.WithLastUserMessageTime(time.Unix(m.lastUserMessageTime, 0)),
			),
		)
	}
	
	// Add tool calls with their results and status
	for _, tc := range msg.ToolCalls() {
		options := m.buildToolCallOptions(tc, msg, toolResultMap)
		uiMessages = append(uiMessages, messages.NewToolCallCmp(msg.ID, tc, options...))
	}
	
	return uiMessages
}

// buildToolCallOptions creates options for tool call components based on results and status.
func (m *messageListCmp) buildToolCallOptions(tc message.ToolCall, msg message.Message, toolResultMap map[string]message.ToolResult) []messages.ToolCallOption {
	var options []messages.ToolCallOption
	
	// Add tool result if available
	if tr, ok := toolResultMap[tc.ID]; ok {
		options = append(options, messages.WithToolCallResult(tr))
	}
	
	// Add cancelled status if applicable
	if msg.FinishPart() != nil && msg.FinishPart().Reason == message.FinishReasonCanceled {
		options = append(options, messages.WithToolCallCancelled())
	}
	
	return options
}

// GetSize returns the current width and height of the component.
func (m *messageListCmp) GetSize() (int, int) {
	return m.width, m.height
}

// SetSize updates the component dimensions and propagates to the list component.
func (m *messageListCmp) SetSize(width int, height int) tea.Cmd {
	m.width = width
	m.height = height - 1
	return m.listCmp.SetSize(width, height-1)
}
