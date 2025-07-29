package chat

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/llm/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/chat/messages"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

type SendMsg struct {
	Text        string
	Attachments []message.Attachment
}

type SessionSelectedMsg = session.Session

type SessionClearedMsg struct{}

const (
	NotFound = -1
)

// MessageListCmp represents a component that displays a list of chat messages
// with support for real-time updates and session management.
type MessageListCmp interface {
	util.Model
	layout.Sizeable
	layout.Focusable
	layout.Help

	SetSession(session.Session) tea.Cmd
	GoToBottom() tea.Cmd
}

// messageListCmp implements MessageListCmp, providing a virtualized list
// of chat messages with support for tool calls, real-time updates, and
// session switching.
type messageListCmp struct {
	app              *app.App
	width, height    int
	session          session.Session
	listCmp          list.List[list.Item]
	previousSelected string // Last selected item index for restoring focus

	lastUserMessageTime int64
	defaultListKeyMap   list.KeyMap
}

// New creates a new message list component with custom keybindings
// and reverse ordering (newest messages at bottom).
func New(app *app.App) MessageListCmp {
	defaultListKeyMap := list.DefaultKeyMap()
	listCmp := list.New(
		[]list.Item{},
		list.WithGap(1),
		list.WithDirectionBackward(),
		list.WithFocus(false),
		list.WithKeyMap(defaultListKeyMap),
		list.WithEnableMouse(),
	)
	return &messageListCmp{
		app:               app,
		listCmp:           listCmp,
		previousSelected:  "",
		defaultListKeyMap: defaultListKeyMap,
	}
}

// Init initializes the component.
func (m *messageListCmp) Init() tea.Cmd {
	return m.listCmp.Init()
}

// Update handles incoming messages and updates the component state.
func (m *messageListCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case pubsub.Event[permission.PermissionNotification]:
		return m, m.handlePermissionRequest(msg.Payload)
	case SessionSelectedMsg:
		if msg.ID != m.session.ID {
			cmd := m.SetSession(msg)
			return m, cmd
		}
		return m, nil
	case SessionClearedMsg:
		m.session = session.Session{}
		return m, m.listCmp.SetItems([]list.Item{})

	case pubsub.Event[message.Message]:
		cmd := m.handleMessageEvent(msg)
		return m, cmd

	case tea.MouseWheelMsg:
		u, cmd := m.listCmp.Update(msg)
		m.listCmp = u.(list.List[list.Item])
		return m, cmd
	default:
		var cmds []tea.Cmd
		u, cmd := m.listCmp.Update(msg)
		m.listCmp = u.(list.List[list.Item])
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}
}

// View renders the message list or an initial screen if empty.
func (m *messageListCmp) View() string {
	t := styles.CurrentTheme()
	return t.S().Base.
		Padding(1, 1, 0, 1).
		Width(m.width).
		Height(m.height).
		Render(
			m.listCmp.View(),
		)
}

func (m *messageListCmp) handlePermissionRequest(permission permission.PermissionNotification) tea.Cmd {
	items := m.listCmp.Items()
	if toolCallIndex := m.findToolCallByID(items, permission.ToolCallID); toolCallIndex != NotFound {
		toolCall := items[toolCallIndex].(messages.ToolCallCmp)
		toolCall.SetPermissionRequested()
		if permission.Granted {
			toolCall.SetPermissionGranted()
		}
		m.listCmp.UpdateItem(toolCall.ID(), toolCall)
	}
	return nil
}

// handleChildSession handles messages from child sessions (agent tools).
func (m *messageListCmp) handleChildSession(event pubsub.Event[message.Message]) tea.Cmd {
	var cmds []tea.Cmd
	if len(event.Payload.ToolCalls()) == 0 && len(event.Payload.ToolResults()) == 0 {
		return nil
	}
	items := m.listCmp.Items()
	toolCallInx := NotFound
	var toolCall messages.ToolCallCmp
	for i := len(items) - 1; i >= 0; i-- {
		if msg, ok := items[i].(messages.ToolCallCmp); ok {
			if msg.GetToolCall().ID == event.Payload.SessionID {
				toolCallInx = i
				toolCall = msg
			}
		}
	}
	if toolCallInx == NotFound {
		return nil
	}
	nestedToolCalls := toolCall.GetNestedToolCalls()
	for _, tc := range event.Payload.ToolCalls() {
		found := false
		for existingInx, existingTC := range nestedToolCalls {
			if existingTC.GetToolCall().ID == tc.ID {
				nestedToolCalls[existingInx].SetToolCall(tc)
				found = true
				break
			}
		}
		if !found {
			nestedCall := messages.NewToolCallCmp(
				event.Payload.ID,
				tc,
				m.app.Permissions,
				messages.WithToolCallNested(true),
			)
			cmds = append(cmds, nestedCall.Init())
			nestedToolCalls = append(
				nestedToolCalls,
				nestedCall,
			)
		}
	}
	for _, tr := range event.Payload.ToolResults() {
		for nestedInx, nestedTC := range nestedToolCalls {
			if nestedTC.GetToolCall().ID == tr.ToolCallID {
				nestedToolCalls[nestedInx].SetToolResult(tr)
				break
			}
		}
	}

	toolCall.SetNestedToolCalls(nestedToolCalls)
	m.listCmp.UpdateItem(
		toolCall.ID(),
		toolCall,
	)
	return tea.Batch(cmds...)
}

// handleMessageEvent processes different types of message events (created/updated).
func (m *messageListCmp) handleMessageEvent(event pubsub.Event[message.Message]) tea.Cmd {
	switch event.Type {
	case pubsub.CreatedEvent:
		if event.Payload.SessionID != m.session.ID {
			return m.handleChildSession(event)
		}
		if m.messageExists(event.Payload.ID) {
			return nil
		}
		return m.handleNewMessage(event.Payload)
	case pubsub.UpdatedEvent:
		if event.Payload.SessionID != m.session.ID {
			return m.handleChildSession(event)
		}
		switch event.Payload.Role {
		case message.Assistant:
			return m.handleUpdateAssistantMessage(event.Payload)
		case message.Tool:
			return m.handleToolMessage(event.Payload)
		}
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
			m.listCmp.UpdateItem(toolCall.ID(), toolCall)
		}
	}
	return nil
}

// findToolCallByID searches for a tool call with the specified ID.
// Returns the index if found, NotFound otherwise.
func (m *messageListCmp) findToolCallByID(items []list.Item, toolCallID string) int {
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
func (m *messageListCmp) findAssistantMessageAndToolCalls(items []list.Item, messageID string) (int, map[int]messages.ToolCallCmp) {
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
			if tc.ParentMessageID() == messageID {
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

	var cmd tea.Cmd
	if shouldShowMessage {
		items := m.listCmp.Items()
		uiMsg := items[assistantIndex].(messages.MessageCmp)
		uiMsg.SetMessage(msg)
		m.listCmp.UpdateItem(
			items[assistantIndex].ID(),
			uiMsg,
		)
		if msg.FinishPart() != nil && msg.FinishPart().Reason == message.FinishReasonEndTurn {
			m.listCmp.AppendItem(
				messages.NewAssistantSection(
					msg,
					time.Unix(m.lastUserMessageTime, 0),
				),
			)
		}
	} else if hasToolCallsOnly {
		items := m.listCmp.Items()
		m.listCmp.DeleteItem(items[assistantIndex].ID())
	}

	return cmd
}

// shouldShowAssistantMessage determines if an assistant message should be displayed.
func (m *messageListCmp) shouldShowAssistantMessage(msg message.Message) bool {
	return len(msg.ToolCalls()) == 0 || msg.Content().Text != "" || msg.ReasoningContent().Thinking != "" || msg.IsThinking()
}

// updateToolCalls handles updates to tool calls, updating existing ones and adding new ones.
func (m *messageListCmp) updateToolCalls(msg message.Message, existingToolCalls map[int]messages.ToolCallCmp) tea.Cmd {
	var cmds []tea.Cmd

	for _, tc := range msg.ToolCalls() {
		if cmd := m.updateOrAddToolCall(msg, tc, existingToolCalls); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

// updateOrAddToolCall updates an existing tool call or adds a new one.
func (m *messageListCmp) updateOrAddToolCall(msg message.Message, tc message.ToolCall, existingToolCalls map[int]messages.ToolCallCmp) tea.Cmd {
	// Try to find existing tool call
	for _, existingTC := range existingToolCalls {
		if tc.ID == existingTC.GetToolCall().ID {
			existingTC.SetToolCall(tc)
			if msg.FinishPart() != nil && msg.FinishPart().Reason == message.FinishReasonCanceled {
				existingTC.SetCancelled()
			}
			m.listCmp.UpdateItem(tc.ID, existingTC)
			return nil
		}
	}

	// Add new tool call if not found
	return m.listCmp.AppendItem(messages.NewToolCallCmp(msg.ID, tc, m.app.Permissions))
}

// handleNewAssistantMessage processes new assistant messages and their tool calls.
func (m *messageListCmp) handleNewAssistantMessage(msg message.Message) tea.Cmd {
	var cmds []tea.Cmd

	// Add assistant message if it should be displayed
	if m.shouldShowAssistantMessage(msg) {
		cmd := m.listCmp.AppendItem(
			messages.NewMessageCmp(
				msg,
			),
		)
		cmds = append(cmds, cmd)
	}

	// Add tool calls
	for _, tc := range msg.ToolCalls() {
		cmd := m.listCmp.AppendItem(messages.NewToolCallCmp(msg.ID, tc, m.app.Permissions))
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
		return m.listCmp.SetItems([]list.Item{})
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
func (m *messageListCmp) convertMessagesToUI(sessionMessages []message.Message, toolResultMap map[string]message.ToolResult) []list.Item {
	uiMessages := make([]list.Item, 0)

	for _, msg := range sessionMessages {
		switch msg.Role {
		case message.User:
			m.lastUserMessageTime = msg.CreatedAt
			uiMessages = append(uiMessages, messages.NewMessageCmp(msg))
		case message.Assistant:
			uiMessages = append(uiMessages, m.convertAssistantMessage(msg, toolResultMap)...)
			if msg.FinishPart() != nil && msg.FinishPart().Reason == message.FinishReasonEndTurn {
				uiMessages = append(uiMessages, messages.NewAssistantSection(msg, time.Unix(m.lastUserMessageTime, 0)))
			}
		}
	}

	return uiMessages
}

// convertAssistantMessage converts an assistant message and its tool calls to UI components.
func (m *messageListCmp) convertAssistantMessage(msg message.Message, toolResultMap map[string]message.ToolResult) []list.Item {
	var uiMessages []list.Item

	// Add assistant message if it should be displayed
	if m.shouldShowAssistantMessage(msg) {
		uiMessages = append(
			uiMessages,
			messages.NewMessageCmp(
				msg,
			),
		)
	}

	// Add tool calls with their results and status
	for _, tc := range msg.ToolCalls() {
		options := m.buildToolCallOptions(tc, msg, toolResultMap)
		uiMessages = append(uiMessages, messages.NewToolCallCmp(msg.ID, tc, m.app.Permissions, options...))
		// If this tool call is the agent tool, fetch nested tool calls
		if tc.Name == agent.AgentToolName {
			nestedMessages, _ := m.app.Messages.List(context.Background(), tc.ID)
			nestedToolResultMap := m.buildToolResultMap(nestedMessages)
			nestedUIMessages := m.convertMessagesToUI(nestedMessages, nestedToolResultMap)
			nestedToolCalls := make([]messages.ToolCallCmp, 0, len(nestedUIMessages))
			for _, nestedMsg := range nestedUIMessages {
				if toolCall, ok := nestedMsg.(messages.ToolCallCmp); ok {
					toolCall.SetIsNested(true)
					nestedToolCalls = append(nestedToolCalls, toolCall)
				}
			}
			uiMessages[len(uiMessages)-1].(messages.ToolCallCmp).SetNestedToolCalls(nestedToolCalls)
		}
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
	m.height = height
	return m.listCmp.SetSize(width-2, height-1) // for padding
}

// Blur implements MessageListCmp.
func (m *messageListCmp) Blur() tea.Cmd {
	return m.listCmp.Blur()
}

// Focus implements MessageListCmp.
func (m *messageListCmp) Focus() tea.Cmd {
	return m.listCmp.Focus()
}

// IsFocused implements MessageListCmp.
func (m *messageListCmp) IsFocused() bool {
	return m.listCmp.IsFocused()
}

func (m *messageListCmp) Bindings() []key.Binding {
	return m.defaultListKeyMap.KeyBindings()
}

func (m *messageListCmp) GoToBottom() tea.Cmd {
	return m.listCmp.GoToBottom()
}
