package model

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/filepicker"
	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/completions"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/logo"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/uiutil"
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

type openEditorMsg struct {
	Text string
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
	status *Status

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

	attachments []message.Attachment // TODO: Implement attachments

	readyPlaceholder   string
	workingPlaceholder string

	// Completions state
	completions              *completions.Completions
	completionsOpen          bool
	completionsStartIndex    int
	completionsQuery         string
	completionsPositionStart image.Point // x,y where user typed '@'

	// Chat components
	chat *Chat

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

	ch := NewChat(com)

	// Completions component
	comp := completions.New(
		com.Styles.Completions.Normal,
		com.Styles.Completions.Focused,
		com.Styles.Completions.Match,
	)

	ui := &UI{
		com:         com,
		dialog:      dialog.NewOverlay(),
		keyMap:      DefaultKeyMap(),
		focus:       uiFocusNone,
		state:       uiConfigure,
		textarea:    ta,
		chat:        ch,
		completions: comp,
	}

	status := NewStatus(com, ui)

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

	ui.setEditorPrompt(false)
	ui.randomizePlaceholders()
	ui.textarea.Placeholder = ui.readyPlaceholder
	ui.status = status

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

// Update handles updates to the UI model.
func (m *UI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.EnvMsg:
		// Is this Windows Terminal?
		if !m.sendProgressBar {
			m.sendProgressBar = slices.Contains(msg, "WT_SESSION")
		}
	case loadSessionMsg:
		m.state = uiChat
		m.session = msg.session
		m.sessionFiles = msg.files
		msgs, err := m.com.App.Messages.List(context.Background(), m.session.ID)
		if err != nil {
			cmds = append(cmds, uiutil.ReportError(err))
			break
		}
		if cmd := m.setSessionMessages(msgs); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case pubsub.Event[message.Message]:
		// Check if this is a child session message for an agent tool.
		if m.session == nil {
			break
		}
		if msg.Payload.SessionID != m.session.ID {
			// This might be a child session message from an agent tool.
			if cmd := m.handleChildSessionMessage(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
			break
		}
		switch msg.Type {
		case pubsub.CreatedEvent:
			cmds = append(cmds, m.appendSessionMessage(msg.Payload))
		case pubsub.UpdatedEvent:
			cmds = append(cmds, m.updateSessionMessage(msg.Payload))
		}
	case pubsub.Event[history.File]:
		cmds = append(cmds, m.handleFileEvent(msg.Payload))
	case pubsub.Event[app.LSPEvent]:
		m.lspStates = app.GetLSPStates()
	case pubsub.Event[mcp.Event]:
		m.mcpStates = mcp.GetStates()
		if msg.Type == pubsub.UpdatedEvent && m.dialog.ContainsDialog(dialog.CommandsID) {
			dia := m.dialog.Dialog(dialog.CommandsID)
			if dia == nil {
				break
			}

			commands, ok := dia.(*dialog.Commands)
			if ok {
				if cmd := commands.ReloadMCPPrompts(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
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
				if cmd := m.chat.ScrollByAndAnimate(-1); cmd != nil {
					cmds = append(cmds, cmd)
				}
				if !m.chat.SelectedItemInView() {
					m.chat.SelectPrev()
					if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			} else if msg.Y >= m.chat.Height()-1 {
				if cmd := m.chat.ScrollByAndAnimate(1); cmd != nil {
					cmds = append(cmds, cmd)
				}
				if !m.chat.SelectedItemInView() {
					m.chat.SelectNext()
					if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
						cmds = append(cmds, cmd)
					}
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
				if cmd := m.chat.ScrollByAndAnimate(-5); cmd != nil {
					cmds = append(cmds, cmd)
				}
				if !m.chat.SelectedItemInView() {
					m.chat.SelectPrev()
					if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			case tea.MouseWheelDown:
				if cmd := m.chat.ScrollByAndAnimate(5); cmd != nil {
					cmds = append(cmds, cmd)
				}
				if !m.chat.SelectedItemInView() {
					m.chat.SelectNext()
					if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			}
		}
	case anim.StepMsg:
		if m.state == uiChat {
			if cmd := m.chat.Animate(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case completions.SelectionMsg:
		// Handle file completion selection.
		if item, ok := msg.Value.(completions.FileCompletionValue); ok {
			m.insertFileCompletion(item.Path)
		}
		if !msg.Insert {
			m.closeCompletions()
		}
	case completions.FilesLoadedMsg:
		// Handle async file loading for completions.
		if m.completionsOpen {
			m.completions.SetFiles(msg.Files)
		}
	case completions.ClosedMsg:
		m.completionsOpen = false
	case tea.KeyPressMsg:
		if cmd := m.handleKeyPressMsg(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case tea.PasteMsg:
		if cmd := m.handlePasteMsg(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case openEditorMsg:
		m.textarea.SetValue(msg.Text)
		m.textarea.MoveToEnd()
	case uiutil.InfoMsg:
		m.status.SetInfoMsg(msg)
		ttl := msg.TTL
		if ttl <= 0 {
			ttl = DefaultStatusTTL
		}
		cmds = append(cmds, clearInfoMsgCmd(ttl))
	case uiutil.ClearStatusMsg:
		m.status.ClearInfoMsg()
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

// setSessionMessages sets the messages for the current session in the chat
func (m *UI) setSessionMessages(msgs []message.Message) tea.Cmd {
	var cmds []tea.Cmd
	// Build tool result map to link tool calls with their results
	msgPtrs := make([]*message.Message, len(msgs))
	for i := range msgs {
		msgPtrs[i] = &msgs[i]
	}
	toolResultMap := chat.BuildToolResultMap(msgPtrs)

	// Add messages to chat with linked tool results
	items := make([]chat.MessageItem, 0, len(msgs)*2)
	for _, msg := range msgPtrs {
		items = append(items, chat.ExtractMessageItems(m.com.Styles, msg, toolResultMap)...)
	}

	// Load nested tool calls for agent/agentic_fetch tools.
	m.loadNestedToolCalls(items)

	// If the user switches between sessions while the agent is working we want
	// to make sure the animations are shown.
	for _, item := range items {
		if animatable, ok := item.(chat.Animatable); ok {
			if cmd := animatable.StartAnimation(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	m.chat.SetMessages(items...)
	if cmd := m.chat.ScrollToBottomAndAnimate(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.chat.SelectLast()
	return tea.Batch(cmds...)
}

// loadNestedToolCalls recursively loads nested tool calls for agent/agentic_fetch tools.
func (m *UI) loadNestedToolCalls(items []chat.MessageItem) {
	for _, item := range items {
		nestedContainer, ok := item.(chat.NestedToolContainer)
		if !ok {
			continue
		}
		toolItem, ok := item.(chat.ToolMessageItem)
		if !ok {
			continue
		}

		tc := toolItem.ToolCall()
		messageID := toolItem.MessageID()

		// Get the agent tool session ID.
		agentSessionID := m.com.App.Sessions.CreateAgentToolSessionID(messageID, tc.ID)

		// Fetch nested messages.
		nestedMsgs, err := m.com.App.Messages.List(context.Background(), agentSessionID)
		if err != nil || len(nestedMsgs) == 0 {
			continue
		}

		// Build tool result map for nested messages.
		nestedMsgPtrs := make([]*message.Message, len(nestedMsgs))
		for i := range nestedMsgs {
			nestedMsgPtrs[i] = &nestedMsgs[i]
		}
		nestedToolResultMap := chat.BuildToolResultMap(nestedMsgPtrs)

		// Extract nested tool items.
		var nestedTools []chat.ToolMessageItem
		for _, nestedMsg := range nestedMsgPtrs {
			nestedItems := chat.ExtractMessageItems(m.com.Styles, nestedMsg, nestedToolResultMap)
			for _, nestedItem := range nestedItems {
				if nestedToolItem, ok := nestedItem.(chat.ToolMessageItem); ok {
					// Mark nested tools as simple (compact) rendering.
					if simplifiable, ok := nestedToolItem.(chat.Compactable); ok {
						simplifiable.SetCompact(true)
					}
					nestedTools = append(nestedTools, nestedToolItem)
				}
			}
		}

		// Recursively load nested tool calls for any agent tools within.
		nestedMessageItems := make([]chat.MessageItem, len(nestedTools))
		for i, nt := range nestedTools {
			nestedMessageItems[i] = nt
		}
		m.loadNestedToolCalls(nestedMessageItems)

		// Set nested tools on the parent.
		nestedContainer.SetNestedTools(nestedTools)
	}
}

// appendSessionMessage appends a new message to the current session in the chat
// if the message is a tool result it will update the corresponding tool call message
func (m *UI) appendSessionMessage(msg message.Message) tea.Cmd {
	var cmds []tea.Cmd
	switch msg.Role {
	case message.User, message.Assistant:
		items := chat.ExtractMessageItems(m.com.Styles, &msg, nil)
		for _, item := range items {
			if animatable, ok := item.(chat.Animatable); ok {
				if cmd := animatable.StartAnimation(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
		m.chat.AppendMessages(items...)
		if cmd := m.chat.ScrollToBottomAndAnimate(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case message.Tool:
		for _, tr := range msg.ToolResults() {
			toolItem := m.chat.MessageItem(tr.ToolCallID)
			if toolItem == nil {
				// we should have an item!
				continue
			}
			if toolMsgItem, ok := toolItem.(chat.ToolMessageItem); ok {
				toolMsgItem.SetResult(&tr)
			}
		}
	}
	return tea.Batch(cmds...)
}

// updateSessionMessage updates an existing message in the current session in the chat
// when an assistant message is updated it may include updated tool calls as well
// that is why we need to handle creating/updating each tool call message too
func (m *UI) updateSessionMessage(msg message.Message) tea.Cmd {
	var cmds []tea.Cmd
	existingItem := m.chat.MessageItem(msg.ID)

	if existingItem != nil {
		if assistantItem, ok := existingItem.(*chat.AssistantMessageItem); ok {
			assistantItem.SetMessage(&msg)
		}
	}

	// if the message of the assistant does not have any  response just tool calls we need to remove it
	if !chat.ShouldRenderAssistantMessage(&msg) && len(msg.ToolCalls()) > 0 && existingItem != nil {
		m.chat.RemoveMessage(msg.ID)
	}

	var items []chat.MessageItem
	for _, tc := range msg.ToolCalls() {
		existingToolItem := m.chat.MessageItem(tc.ID)
		if toolItem, ok := existingToolItem.(chat.ToolMessageItem); ok {
			existingToolCall := toolItem.ToolCall()
			// only update if finished state changed or input changed
			// to avoid clearing the cache
			if (tc.Finished && !existingToolCall.Finished) || tc.Input != existingToolCall.Input {
				toolItem.SetToolCall(tc)
			}
		}
		if existingToolItem == nil {
			items = append(items, chat.NewToolMessageItem(m.com.Styles, msg.ID, tc, nil, false))
		}
	}

	for _, item := range items {
		if animatable, ok := item.(chat.Animatable); ok {
			if cmd := animatable.StartAnimation(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	m.chat.AppendMessages(items...)
	if cmd := m.chat.ScrollToBottomAndAnimate(); cmd != nil {
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

// handleChildSessionMessage handles messages from child sessions (agent tools).
func (m *UI) handleChildSessionMessage(event pubsub.Event[message.Message]) tea.Cmd {
	var cmds []tea.Cmd

	// Only process messages with tool calls or results.
	if len(event.Payload.ToolCalls()) == 0 && len(event.Payload.ToolResults()) == 0 {
		return nil
	}

	// Check if this is an agent tool session and parse it.
	childSessionID := event.Payload.SessionID
	_, toolCallID, ok := m.com.App.Sessions.ParseAgentToolSessionID(childSessionID)
	if !ok {
		return nil
	}

	// Find the parent agent tool item.
	var agentItem chat.NestedToolContainer
	for i := 0; i < m.chat.Len(); i++ {
		item := m.chat.MessageItem(toolCallID)
		if item == nil {
			continue
		}
		if agent, ok := item.(chat.NestedToolContainer); ok {
			if toolMessageItem, ok := item.(chat.ToolMessageItem); ok {
				if toolMessageItem.ToolCall().ID == toolCallID {
					// Verify this agent belongs to the correct parent message.
					// We can't directly check parentMessageID on the item, so we trust the session parsing.
					agentItem = agent
					break
				}
			}
		}
	}

	if agentItem == nil {
		return nil
	}

	// Get existing nested tools.
	nestedTools := agentItem.NestedTools()

	// Update or create nested tool calls.
	for _, tc := range event.Payload.ToolCalls() {
		found := false
		for _, existingTool := range nestedTools {
			if existingTool.ToolCall().ID == tc.ID {
				existingTool.SetToolCall(tc)
				found = true
				break
			}
		}
		if !found {
			// Create a new nested tool item.
			nestedItem := chat.NewToolMessageItem(m.com.Styles, event.Payload.ID, tc, nil, false)
			if simplifiable, ok := nestedItem.(chat.Compactable); ok {
				simplifiable.SetCompact(true)
			}
			if animatable, ok := nestedItem.(chat.Animatable); ok {
				if cmd := animatable.StartAnimation(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			nestedTools = append(nestedTools, nestedItem)
		}
	}

	// Update nested tool results.
	for _, tr := range event.Payload.ToolResults() {
		for _, nestedTool := range nestedTools {
			if nestedTool.ToolCall().ID == tr.ToolCallID {
				nestedTool.SetResult(&tr)
				break
			}
		}
	}

	// Update the agent item with the new nested tools.
	agentItem.SetNestedTools(nestedTools)

	// Update the chat so it updates the index map for animations to work as expected
	m.chat.UpdateNestedToolIDs(toolCallID)

	return tea.Batch(cmds...)
}

func (m *UI) handleKeyPressMsg(msg tea.KeyPressMsg) tea.Cmd {
	var cmds []tea.Cmd

	handleGlobalKeys := func(msg tea.KeyPressMsg) bool {
		switch {
		case key.Matches(msg, m.keyMap.Help):
			m.status.ToggleHelp()
			m.updateLayoutAndSize()
			return true
		case key.Matches(msg, m.keyMap.Commands):
			if cmd := m.openCommandsDialog(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			return true
		case key.Matches(msg, m.keyMap.Models):
			if cmd := m.openModelsDialog(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			return true
		case key.Matches(msg, m.keyMap.Sessions):
			if cmd := m.openSessionsDialog(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			return true
		}
		return false
	}

	if key.Matches(msg, m.keyMap.Quit) && !m.dialog.ContainsDialog(dialog.QuitID) {
		// Always handle quit keys first
		if cmd := m.openQuitDialog(); cmd != nil {
			cmds = append(cmds, cmd)
		}

		return tea.Batch(cmds...)
	}

	// Route all messages to dialog if one is open.
	if m.dialog.HasDialogs() {
		msg := m.dialog.Update(msg)
		if msg == nil {
			return tea.Batch(cmds...)
		}

		switch msg := msg.(type) {
		// Generic dialog messages
		case dialog.CloseMsg:
			m.dialog.CloseFrontDialog()

		// Session dialog messages
		case dialog.SessionSelectedMsg:
			m.dialog.CloseDialog(dialog.SessionsID)
			cmds = append(cmds, m.loadSession(msg.Session.ID))

		// Open dialog message
		case dialog.OpenDialogMsg:
			switch msg.DialogID {
			case dialog.SessionsID:
				if cmd := m.openSessionsDialog(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			case dialog.ModelsID:
				if cmd := m.openModelsDialog(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			default:
				// Unknown dialog
				break
			}

			m.dialog.CloseDialog(dialog.CommandsID)

		// Command dialog messages
		case dialog.ToggleYoloModeMsg:
			yolo := !m.com.App.Permissions.SkipRequests()
			m.com.App.Permissions.SetSkipRequests(yolo)
			m.setEditorPrompt(yolo)
			m.dialog.CloseDialog(dialog.CommandsID)
		case dialog.NewSessionsMsg:
			if m.com.App.AgentCoordinator != nil && m.com.App.AgentCoordinator.IsBusy() {
				cmds = append(cmds, uiutil.ReportWarn("Agent is busy, please wait before starting a new session..."))
				break
			}
			m.newSession()
			m.dialog.CloseDialog(dialog.CommandsID)
		case dialog.CompactMsg:
			if m.com.App.AgentCoordinator != nil && m.com.App.AgentCoordinator.IsBusy() {
				cmds = append(cmds, uiutil.ReportWarn("Agent is busy, please wait before summarizing session..."))
				break
			}
			err := m.com.App.AgentCoordinator.Summarize(context.Background(), msg.SessionID)
			if err != nil {
				cmds = append(cmds, uiutil.ReportError(err))
			}
		case dialog.ToggleHelpMsg:
			m.status.ToggleHelp()
			m.dialog.CloseDialog(dialog.CommandsID)
		case dialog.QuitMsg:
			cmds = append(cmds, tea.Quit)
		case dialog.ModelSelectedMsg:
			if m.com.App.AgentCoordinator.IsBusy() {
				cmds = append(cmds, uiutil.ReportWarn("Agent is busy, please wait..."))
				break
			}

			// TODO: Validate model API and authentication here?

			cfg := m.com.Config()
			if cfg == nil {
				cmds = append(cmds, uiutil.ReportError(errors.New("configuration not found")))
				break
			}

			if err := cfg.UpdatePreferredModel(msg.ModelType, msg.Model); err != nil {
				cmds = append(cmds, uiutil.ReportError(err))
			}

			// XXX: Should this be in a separate goroutine?
			go m.com.App.UpdateAgentModel(context.TODO())

			modelMsg := fmt.Sprintf("%s model changed to %s", msg.ModelType, msg.Model.Model)
			cmds = append(cmds, uiutil.ReportInfo(modelMsg))
			m.dialog.CloseDialog(dialog.ModelsID)
		}

		return tea.Batch(cmds...)
	}

	switch m.state {
	case uiConfigure:
		return tea.Batch(cmds...)
	case uiInitialize:
		cmds = append(cmds, m.updateInitializeView(msg)...)
		return tea.Batch(cmds...)
	case uiChat, uiLanding, uiChatCompact:
		switch m.focus {
		case uiFocusEditor:
			// Handle completions if open.
			if m.completionsOpen {
				if cmd, ok := m.completions.Update(msg); ok {
					cmds = append(cmds, cmd)
					return tea.Batch(cmds...)
				}
			}

			switch {
			case key.Matches(msg, m.keyMap.Editor.SendMessage):
				value := m.textarea.Value()
				if strings.HasSuffix(value, "\\") {
					// If the last character is a backslash, remove it and add a newline.
					m.textarea.SetValue(strings.TrimSuffix(value, "\\"))
					break
				}

				// Otherwise, send the message
				m.textarea.Reset()

				value = strings.TrimSpace(value)
				if value == "exit" || value == "quit" {
					return m.openQuitDialog()
				}

				attachments := m.attachments
				m.attachments = nil
				if len(value) == 0 {
					return nil
				}

				m.randomizePlaceholders()

				return m.sendMessage(value, attachments)
			case key.Matches(msg, m.keyMap.Chat.NewSession):
				if m.session == nil || m.session.ID == "" {
					break
				}
				if m.com.App.AgentCoordinator != nil && m.com.App.AgentCoordinator.IsBusy() {
					cmds = append(cmds, uiutil.ReportWarn("Agent is busy, please wait before starting a new session..."))
					break
				}
				m.newSession()
			case key.Matches(msg, m.keyMap.Tab):
				m.focus = uiFocusMain
				m.textarea.Blur()
				m.chat.Focus()
				m.chat.SetSelected(m.chat.Len() - 1)
			case key.Matches(msg, m.keyMap.Editor.OpenEditor):
				if m.session != nil && m.com.App.AgentCoordinator.IsSessionBusy(m.session.ID) {
					cmds = append(cmds, uiutil.ReportWarn("Agent is working, please wait..."))
					break
				}
				cmds = append(cmds, m.openEditor(m.textarea.Value()))
			case key.Matches(msg, m.keyMap.Editor.Newline):
				m.textarea.InsertRune('\n')
				m.closeCompletions()
			default:
				if handleGlobalKeys(msg) {
					// Handle global keys first before passing to textarea.
					break
				}

				// Check for @ trigger before passing to textarea.
				curValue := m.textarea.Value()
				curIdx := len(curValue)

				// Trigger completions on @.
				if msg.String() == "@" && !m.completionsOpen {
					// Only show if beginning of prompt or after whitespace.
					if curIdx == 0 || (curIdx > 0 && isWhitespace(curValue[curIdx-1])) {
						m.completionsOpen = true
						m.completionsQuery = ""
						m.completionsStartIndex = curIdx
						m.completionsPositionStart = m.completionsPosition()
						depth, limit := m.com.Config().Options.TUI.Completions.Limits()
						cmds = append(cmds, m.completions.OpenWithFiles(depth, limit))
					}
				}

				ta, cmd := m.textarea.Update(msg)
				m.textarea = ta
				cmds = append(cmds, cmd)

				// After updating textarea, check if we need to filter completions.
				// Skip filtering on the initial @ keystroke since items are loading async.
				if m.completionsOpen && msg.String() != "@" {
					newValue := m.textarea.Value()
					newIdx := len(newValue)

					// Close completions if cursor moved before start.
					if newIdx <= m.completionsStartIndex {
						m.closeCompletions()
					} else if msg.String() == "space" {
						// Close on space.
						m.closeCompletions()
					} else {
						// Extract current word and filter.
						word := m.textareaWord()
						if strings.HasPrefix(word, "@") {
							m.completionsQuery = word[1:]
							m.completions.Filter(m.completionsQuery)
						} else if m.completionsOpen {
							m.closeCompletions()
						}
					}
				}
			}
		case uiFocusMain:
			switch {
			case key.Matches(msg, m.keyMap.Tab):
				m.focus = uiFocusEditor
				cmds = append(cmds, m.textarea.Focus())
				m.chat.Blur()
			case key.Matches(msg, m.keyMap.Chat.Expand):
				m.chat.ToggleExpandedSelectedItem()
			case key.Matches(msg, m.keyMap.Chat.Up):
				if cmd := m.chat.ScrollByAndAnimate(-1); cmd != nil {
					cmds = append(cmds, cmd)
				}
				if !m.chat.SelectedItemInView() {
					m.chat.SelectPrev()
					if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			case key.Matches(msg, m.keyMap.Chat.Down):
				if cmd := m.chat.ScrollByAndAnimate(1); cmd != nil {
					cmds = append(cmds, cmd)
				}
				if !m.chat.SelectedItemInView() {
					m.chat.SelectNext()
					if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			case key.Matches(msg, m.keyMap.Chat.UpOneItem):
				m.chat.SelectPrev()
				if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			case key.Matches(msg, m.keyMap.Chat.DownOneItem):
				m.chat.SelectNext()
				if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			case key.Matches(msg, m.keyMap.Chat.HalfPageUp):
				if cmd := m.chat.ScrollByAndAnimate(-m.chat.Height() / 2); cmd != nil {
					cmds = append(cmds, cmd)
				}
				m.chat.SelectFirstInView()
			case key.Matches(msg, m.keyMap.Chat.HalfPageDown):
				if cmd := m.chat.ScrollByAndAnimate(m.chat.Height() / 2); cmd != nil {
					cmds = append(cmds, cmd)
				}
				m.chat.SelectLastInView()
			case key.Matches(msg, m.keyMap.Chat.PageUp):
				if cmd := m.chat.ScrollByAndAnimate(-m.chat.Height()); cmd != nil {
					cmds = append(cmds, cmd)
				}
				m.chat.SelectFirstInView()
			case key.Matches(msg, m.keyMap.Chat.PageDown):
				if cmd := m.chat.ScrollByAndAnimate(m.chat.Height()); cmd != nil {
					cmds = append(cmds, cmd)
				}
				m.chat.SelectLastInView()
			case key.Matches(msg, m.keyMap.Chat.Home):
				if cmd := m.chat.ScrollToTopAndAnimate(); cmd != nil {
					cmds = append(cmds, cmd)
				}
				m.chat.SelectFirst()
			case key.Matches(msg, m.keyMap.Chat.End):
				if cmd := m.chat.ScrollToBottomAndAnimate(); cmd != nil {
					cmds = append(cmds, cmd)
				}
				m.chat.SelectLast()
			default:
				handleGlobalKeys(msg)
			}
		default:
			handleGlobalKeys(msg)
		}
	default:
		handleGlobalKeys(msg)
	}

	return tea.Batch(cmds...)
}

// Draw implements [uv.Drawable] and draws the UI model.
func (m *UI) Draw(scr uv.Screen, area uv.Rectangle) {
	layout := m.generateLayout(area.Dx(), area.Dy())

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

	// Add status and help layer
	m.status.Draw(scr, layout.status)

	// Draw completions popup if open
	if m.completionsOpen && m.completions.HasItems() {
		w, h := m.completions.Size()
		x := m.completionsPositionStart.X
		y := m.completionsPositionStart.Y - h

		screenW := area.Dx()
		if x+w > screenW {
			x = screenW - w
		}
		x = max(0, x)
		y = max(0, y)

		completionsView := uv.NewStyledString(m.completions.Render())
		completionsView.Draw(scr, image.Rectangle{
			Min: image.Pt(x, y),
			Max: image.Pt(x+w, y+h),
		})
	}

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
		m.dialog.Draw(scr, area)
	}
}

// Cursor returns the cursor position and properties for the UI model. It
// returns nil if the cursor should not be shown.
func (m *UI) Cursor() *tea.Cursor {
	if m.layout.editor.Dy() <= 0 {
		// Don't show cursor if editor is not visible
		return nil
	}
	if m.dialog.HasDialogs() {
		if front := m.dialog.DialogLast(); front != nil {
			c, ok := front.(uiutil.Cursor)
			if ok {
				cur := c.Cursor()
				if cur != nil {
					pos := m.dialog.CenterPosition(m.layout.area, front.ID())
					cur.X += pos.Min.X
					cur.Y += pos.Min.Y
					return cur
				}
			}
		}
		return nil
	}
	switch m.focus {
	case uiFocusEditor:
		if m.textarea.Focused() {
			cur := m.textarea.Cursor()
			cur.X++ // Adjust for app margins
			cur.Y += m.layout.editor.Min.Y
			return cur
		}
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
	tab := k.Tab
	commands := k.Commands
	if m.focus == uiFocusEditor && m.textarea.LineCount() == 0 {
		commands.SetHelp("/ or ctrl+p", "commands")
	}

	switch m.state {
	case uiInitialize:
		binds = append(binds, k.Quit)
	case uiChat:
		if m.focus == uiFocusEditor {
			tab.SetHelp("tab", "focus chat")
		} else {
			tab.SetHelp("tab", "focus editor")
		}

		binds = append(binds,
			tab,
			commands,
			k.Models,
		)

		switch m.focus {
		case uiFocusEditor:
			binds = append(binds,
				k.Editor.Newline,
			)
		case uiFocusMain:
			binds = append(binds,
				k.Chat.UpDown,
				k.Chat.UpDownOneItem,
				k.Chat.PageUp,
				k.Chat.PageDown,
				k.Chat.Copy,
			)
		}
	default:
		// TODO: other states
		// if m.session == nil {
		// no session selected
		binds = append(binds,
			commands,
			k.Models,
			k.Editor.Newline,
		)
	}

	binds = append(binds,
		k.Quit,
		k.Help,
	)

	return binds
}

// FullHelp implements [help.KeyMap].
func (m *UI) FullHelp() [][]key.Binding {
	var binds [][]key.Binding
	k := &m.keyMap
	help := k.Help
	help.SetHelp("ctrl+g", "less")
	hasAttachments := false // TODO: implement attachments
	hasSession := m.session != nil && m.session.ID != ""
	commands := k.Commands
	if m.focus == uiFocusEditor && m.textarea.LineCount() == 0 {
		commands.SetHelp("/ or ctrl+p", "commands")
	}

	switch m.state {
	case uiInitialize:
		binds = append(binds,
			[]key.Binding{
				k.Quit,
			})
	case uiChat:
		mainBinds := []key.Binding{}
		tab := k.Tab
		if m.focus == uiFocusEditor {
			tab.SetHelp("tab", "focus chat")
		} else {
			tab.SetHelp("tab", "focus editor")
		}

		mainBinds = append(mainBinds,
			tab,
			commands,
			k.Models,
			k.Sessions,
		)
		if hasSession {
			mainBinds = append(mainBinds, k.Chat.NewSession)
		}

		binds = append(binds, mainBinds)

		switch m.focus {
		case uiFocusEditor:
			binds = append(binds,
				[]key.Binding{
					k.Editor.Newline,
					k.Editor.AddImage,
					k.Editor.MentionFile,
					k.Editor.OpenEditor,
				},
			)
			if hasAttachments {
				binds = append(binds,
					[]key.Binding{
						k.Editor.AttachmentDeleteMode,
						k.Editor.DeleteAllAttachments,
						k.Editor.Escape,
					},
				)
			}
		case uiFocusMain:
			binds = append(binds,
				[]key.Binding{
					k.Chat.UpDown,
					k.Chat.UpDownOneItem,
					k.Chat.PageUp,
					k.Chat.PageDown,
				},
				[]key.Binding{
					k.Chat.HalfPageUp,
					k.Chat.HalfPageDown,
					k.Chat.Home,
					k.Chat.End,
				},
				[]key.Binding{
					k.Chat.Copy,
					k.Chat.ClearHighlight,
				},
			)
		}
	default:
		if m.session == nil {
			// no session selected
			binds = append(binds,
				[]key.Binding{
					commands,
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
	}

	binds = append(binds,
		[]key.Binding{
			help,
			k.Quit,
		},
	)

	return binds
}

// updateLayoutAndSize updates the layout and sizes of UI components.
func (m *UI) updateLayoutAndSize() {
	m.layout = m.generateLayout(m.width, m.height)
	m.updateSize()
}

// updateSize updates the sizes of UI components based on the current layout.
func (m *UI) updateSize() {
	// Set status width
	m.status.SetWidth(m.layout.status.Dx())

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
func (m *UI) generateLayout(w, h int) layout {
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
	if m.status.ShowingAll() {
		for _, row := range helpKeyMap.FullHelp() {
			helpHeight = max(helpHeight, len(row))
		}
	}

	// Add app margins
	appRect, helpRect := uv.SplitVertical(area, uv.Fixed(area.Dy()-helpHeight))
	appRect.Min.Y += 1
	appRect.Max.Y -= 1
	helpRect.Min.Y -= 1
	appRect.Min.X += 1
	appRect.Max.X -= 1

	if slices.Contains([]uiState{uiConfigure, uiInitialize, uiLanding}, m.state) {
		// extra padding on left and right for these states
		appRect.Min.X += 1
		appRect.Max.X -= 1
	}

	layout := layout{
		area:   area,
		status: helpRect,
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

	// status is the area for the status view.
	status uv.Rectangle
}

func (m *UI) openEditor(value string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// Use platform-appropriate default editor
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "nvim"
		}
	}

	tmpfile, err := os.CreateTemp("", "msg_*.md")
	if err != nil {
		return uiutil.ReportError(err)
	}
	defer tmpfile.Close() //nolint:errcheck
	if _, err := tmpfile.WriteString(value); err != nil {
		return uiutil.ReportError(err)
	}
	cmdStr := editor + " " + tmpfile.Name()
	return uiutil.ExecShell(context.TODO(), cmdStr, func(err error) tea.Msg {
		if err != nil {
			return uiutil.ReportError(err)
		}
		content, err := os.ReadFile(tmpfile.Name())
		if err != nil {
			return uiutil.ReportError(err)
		}
		if len(content) == 0 {
			return uiutil.ReportWarn("Message is empty")
		}
		os.Remove(tmpfile.Name())
		return openEditorMsg{
			Text: strings.TrimSpace(string(content)),
		}
	})
}

// setEditorPrompt configures the textarea prompt function based on whether
// yolo mode is enabled.
func (m *UI) setEditorPrompt(yolo bool) {
	if yolo {
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
		if info.Focused {
			return "  > "
		}
		return "::: "
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

// closeCompletions closes the completions popup and resets state.
func (m *UI) closeCompletions() {
	m.completionsOpen = false
	m.completionsQuery = ""
	m.completionsStartIndex = 0
	m.completions.Close()
}

// insertFileCompletion inserts the selected file path into the textarea,
// replacing the @query, and adds the file as an attachment.
func (m *UI) insertFileCompletion(path string) {
	value := m.textarea.Value()
	word := m.textareaWord()

	// Find the @ and query to replace.
	if m.completionsStartIndex > len(value) {
		return
	}

	// Build the new value: everything before @, the path, everything after query.
	endIdx := m.completionsStartIndex + len(word)
	if endIdx > len(value) {
		endIdx = len(value)
	}

	newValue := value[:m.completionsStartIndex] + path + value[endIdx:]
	m.textarea.SetValue(newValue)
	// XXX: This will always move the cursor to the end of the textarea.
	m.textarea.MoveToEnd()

	// Add file as attachment.
	content, err := os.ReadFile(path)
	if err != nil {
		// If it fails, let the LLM handle it later.
		return
	}

	m.attachments = append(m.attachments, message.Attachment{
		FilePath: path,
		FileName: filepath.Base(path),
		MimeType: mimeOf(content),
		Content:  content,
	})
}

// completionsPosition returns the X and Y position for the completions popup.
func (m *UI) completionsPosition() image.Point {
	cur := m.textarea.Cursor()
	if cur == nil {
		return image.Point{
			X: m.layout.editor.Min.X,
			Y: m.layout.editor.Min.Y,
		}
	}
	return image.Point{
		X: cur.X + m.layout.editor.Min.X,
		Y: m.layout.editor.Min.Y + cur.Y,
	}
}

// textareaWord returns the current word at the cursor position.
func (m *UI) textareaWord() string {
	return m.textarea.Word()
}

// isWhitespace returns true if the byte is a whitespace character.
func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// mimeOf detects the MIME type of the given content.
func mimeOf(content []byte) string {
	mimeBufferSize := min(512, len(content))
	return http.DetectContentType(content[:mimeBufferSize])
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

// sendMessage sends a message with the given content and attachments.
func (m *UI) sendMessage(content string, attachments []message.Attachment) tea.Cmd {
	if m.com.App.AgentCoordinator == nil {
		return uiutil.ReportError(fmt.Errorf("coder agent is not initialized"))
	}

	var cmds []tea.Cmd
	if m.session == nil || m.session.ID == "" {
		newSession, err := m.com.App.Sessions.Create(context.Background(), "New Session")
		if err != nil {
			return uiutil.ReportError(err)
		}
		m.state = uiChat
		m.session = &newSession
		cmds = append(cmds, m.loadSession(newSession.ID))
	}

	// Capture session ID to avoid race with main goroutine updating m.session.
	sessionID := m.session.ID
	cmds = append(cmds, func() tea.Msg {
		_, err := m.com.App.AgentCoordinator.Run(context.Background(), sessionID, content, attachments...)
		if err != nil {
			isCancelErr := errors.Is(err, context.Canceled)
			isPermissionErr := errors.Is(err, permission.ErrorPermissionDenied)
			if isCancelErr || isPermissionErr {
				return nil
			}
			return uiutil.InfoMsg{
				Type: uiutil.InfoTypeError,
				Msg:  err.Error(),
			}
		}
		return nil
	})
	return tea.Batch(cmds...)
}

// openQuitDialog opens the quit confirmation dialog.
func (m *UI) openQuitDialog() tea.Cmd {
	if m.dialog.ContainsDialog(dialog.QuitID) {
		// Bring to front
		m.dialog.BringToFront(dialog.QuitID)
		return nil
	}

	quitDialog := dialog.NewQuit(m.com)
	m.dialog.OpenDialog(quitDialog)
	return nil
}

// openModelsDialog opens the models dialog.
func (m *UI) openModelsDialog() tea.Cmd {
	if m.dialog.ContainsDialog(dialog.ModelsID) {
		// Bring to front
		m.dialog.BringToFront(dialog.ModelsID)
		return nil
	}

	modelsDialog, err := dialog.NewModels(m.com)
	if err != nil {
		return uiutil.ReportError(err)
	}

	modelsDialog.SetSize(min(60, m.width-8), 30)
	m.dialog.OpenDialog(modelsDialog)

	return nil
}

// openCommandsDialog opens the commands dialog.
func (m *UI) openCommandsDialog() tea.Cmd {
	if m.dialog.ContainsDialog(dialog.CommandsID) {
		// Bring to front
		m.dialog.BringToFront(dialog.CommandsID)
		return nil
	}

	sessionID := ""
	if m.session != nil {
		sessionID = m.session.ID
	}

	commands, err := dialog.NewCommands(m.com, sessionID)
	if err != nil {
		return uiutil.ReportError(err)
	}

	// TODO: Get. Rid. Of. Magic numbers!
	commands.SetSize(min(120, m.width-8), 30)
	m.dialog.OpenDialog(commands)

	return nil
}

// openSessionsDialog opens the sessions dialog. If the dialog is already open,
// it brings it to the front. Otherwise, it will list all the sessions and open
// the dialog.
func (m *UI) openSessionsDialog() tea.Cmd {
	if m.dialog.ContainsDialog(dialog.SessionsID) {
		// Bring to front
		m.dialog.BringToFront(dialog.SessionsID)
		return nil
	}

	selectedSessionID := ""
	if m.session != nil {
		selectedSessionID = m.session.ID
	}

	dialog, err := dialog.NewSessions(m.com, selectedSessionID)
	if err != nil {
		return uiutil.ReportError(err)
	}

	// TODO: Get. Rid. Of. Magic numbers!
	dialog.SetSize(min(120, m.width-8), 30)
	m.dialog.OpenDialog(dialog)

	return nil
}

// newSession clears the current session state and prepares for a new session.
// The actual session creation happens when the user sends their first message.
func (m *UI) newSession() {
	if m.session == nil || m.session.ID == "" {
		return
	}

	m.session = nil
	m.sessionFiles = nil
	m.state = uiLanding
	m.focus = uiFocusEditor
	m.textarea.Focus()
	m.chat.Blur()
	m.chat.ClearMessages()
}

// handlePasteMsg handles a paste message.
func (m *UI) handlePasteMsg(msg tea.PasteMsg) tea.Cmd {
	if m.focus != uiFocusEditor {
		return nil
	}

	var cmd tea.Cmd
	path := strings.ReplaceAll(msg.Content, "\\ ", " ")
	// try to get an image
	path, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		m.textarea, cmd = m.textarea.Update(msg)
		return cmd
	}
	isAllowedType := false
	for _, ext := range filepicker.AllowedTypes {
		if strings.HasSuffix(path, ext) {
			isAllowedType = true
			break
		}
	}
	if !isAllowedType {
		m.textarea, cmd = m.textarea.Update(msg)
		return cmd
	}
	tooBig, _ := filepicker.IsFileTooBig(path, filepicker.MaxAttachmentSize)
	if tooBig {
		m.textarea, cmd = m.textarea.Update(msg)
		return cmd
	}

	content, err := os.ReadFile(path)
	if err != nil {
		m.textarea, cmd = m.textarea.Update(msg)
		return cmd
	}
	mimeBufferSize := min(512, len(content))
	mimeType := http.DetectContentType(content[:mimeBufferSize])
	fileName := filepath.Base(path)
	attachment := message.Attachment{FilePath: path, FileName: fileName, MimeType: mimeType, Content: content}
	return uiutil.CmdHandler(filepicker.FilePickedMsg{
		Attachment: attachment,
	})
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
