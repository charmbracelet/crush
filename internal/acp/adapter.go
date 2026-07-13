package acp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	acp "github.com/coder/acp-go-sdk"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
)

func truePtr() *bool { v := true; return &v }

type Adapter struct {
	coordinator agent.Coordinator
	sessions    session.Service
	messages    message.Service
	conn        *acp.AgentSideConnection
	logger      *slog.Logger

	toolKinds map[string]acp.ToolKind
	toolLocs  map[string][]acp.ToolCallLocation
	toolNames map[string]string
}

func NewAdapter(c agent.Coordinator, s session.Service, m message.Service) *Adapter {
	return &Adapter{
		coordinator: c,
		sessions:    s,
		messages:    m,
		logger:      slog.Default(),
		toolKinds:   make(map[string]acp.ToolKind),
		toolLocs:    make(map[string][]acp.ToolCallLocation),
		toolNames:   make(map[string]string),
	}
}

func (a *Adapter) SetLogger(l *slog.Logger)                    { a.logger = l }
func (a *Adapter) SetConnection(conn *acp.AgentSideConnection) { a.conn = conn }
func (a *Adapter) Connection() *acp.AgentSideConnection        { return a.conn }
func (a *Adapter) SetObserver() {
	if a.coordinator != nil {
		a.coordinator.SetObserver(a)
	}
}

func (a *Adapter) OnToolInputStart(sessionID, toolCallID, toolName string) {
	kind := toolKindFromName(toolName)
	a.toolNames[toolCallID] = toolName
	start := acp.StartToolCall(
		acp.ToolCallId(toolCallID), toolName,
		acp.WithStartKind(kind),
		acp.WithStartStatus(acp.ToolCallStatusPending),
	)
	// Note: _zed_visual_command meta is NOT attached here because we
	// don't have the tool input yet (no file paths to extract).
	// Meta is attached in OnToolCall when the input is available.
	_ = kind
	a.sessionUpdate(acp.SessionId(sessionID), start)
}

func (a *Adapter) OnTextDelta(sessionID, messageID, text string) {
	a.sessionUpdate(acp.SessionId(sessionID), acp.UpdateAgentMessageText(text))
}

func (a *Adapter) OnReasoningDelta(sessionID, messageID, text string) {
	a.sessionUpdate(acp.SessionId(sessionID), acp.UpdateAgentThoughtText(text))
}

func (a *Adapter) OnToolCall(sessionID, toolCallID, toolName, input string) {
	kind := toolKindFromName(toolName)
	locs := toolLocationsFromInput(input)
	a.toolKinds[toolCallID] = kind
	a.toolLocs[toolCallID] = locs
	a.toolNames[toolCallID] = toolName

	// Send intermediate in_progress update so ACP clients can animate tool calls.
	update := acp.UpdateToolCall(acp.ToolCallId(toolCallID),
		acp.WithUpdateStatus(acp.ToolCallStatusInProgress),
	)
	meta := a.buildVisualMeta(kind, locs, toolName, input)
	if len(meta) > 0 && update.ToolCallUpdate != nil {
		update.ToolCallUpdate.Meta = meta
	}
	a.sessionUpdate(acp.SessionId(sessionID), update)
}

func (a *Adapter) OnToolResult(sessionID, toolCallID string, output string) {
	kind := a.toolKinds[toolCallID]
	locs := a.toolLocs[toolCallID]
	delete(a.toolKinds, toolCallID)
	delete(a.toolLocs, toolCallID)
	delete(a.toolNames, toolCallID)

	opts := []acp.ToolCallUpdateOpt{acp.WithUpdateStatus(acp.ToolCallStatusCompleted)}
	if kind != "" {
		opts = append(opts, acp.WithUpdateKind(kind))
	}
	if len(locs) > 0 {
		opts = append(opts, acp.WithUpdateLocations(locs))
	}
	if output != "" {
		content := output
		if len(content) > 2000 {
			content = content[:2000] + "..."
		}
		opts = append(opts, acp.WithUpdateContent([]acp.ToolCallContent{
			acp.ToolContent(acp.TextBlock(content)),
		}))
	}
	update := acp.UpdateToolCall(acp.ToolCallId(toolCallID), opts...)
	// Note: _zed_visual_command meta is already attached to the in_progress
	// update in OnToolCall. Skip meta here to avoid duplicate dispatch.
	a.sessionUpdate(acp.SessionId(sessionID), update)
}

// buildVisualMeta constructs the _zed_visual_command meta map based on the tool
// kind, file locations, and original tool input.
func (a *Adapter) buildVisualMeta(kind acp.ToolKind, locs []acp.ToolCallLocation, toolName string, input string) map[string]any {
	var command string
	switch kind {
	case acp.ToolKindRead, acp.ToolKindEdit:
		command = "open_file"
	case acp.ToolKindExecute:
		command = "run_in_terminal"
	default:
		return nil
	}
	if command == "" {
		return nil
	}
	params := map[string]any{}
	for _, loc := range locs {
		if loc.Path != "" {
			params["path"] = loc.Path
			if loc.Line != nil && *loc.Line > 0 {
				params["line"] = *loc.Line
			}
			break // Only use the first location
		}
	}
	if command == "run_in_terminal" && params["path"] == nil {
		if cmd := toolCommandFromInput(input); cmd != "" {
			params["command"] = cmd
		} else if toolName != "" {
			params["command"] = toolName
		}
	}
	if len(params) == 0 {
		return nil
	}
	return map[string]any{
		"_zed_visual_command": map[string]any{
			"command": command,
			"params":  params,
		},
	}
}

func (a *Adapter) OnPlanUpdate(sessionID string, entries []agent.PlanEntry) {
	acpEntries := make([]acp.PlanEntry, len(entries))
	for i, e := range entries {
		acpEntries[i] = acp.PlanEntry{
			Content:  e.Content,
			Priority: planPriority(e.Priority),
			Status:   planStatus(e.Status),
		}
	}
	a.sessionUpdate(acp.SessionId(sessionID), acp.UpdatePlan(acpEntries...))
}

func planPriority(p agent.PlanPriority) acp.PlanEntryPriority {
	switch p {
	case agent.PlanPriorityHigh:
		return acp.PlanEntryPriorityHigh
	case agent.PlanPriorityMedium:
		return acp.PlanEntryPriorityMedium
	case agent.PlanPriorityLow:
		return acp.PlanEntryPriorityLow
	default:
		return acp.PlanEntryPriorityMedium
	}
}

func planStatus(s agent.PlanStatus) acp.PlanEntryStatus {
	switch s {
	case agent.PlanStatusPending:
		return acp.PlanEntryStatusPending
	case agent.PlanStatusInProgress:
		return acp.PlanEntryStatusInProgress
	case agent.PlanStatusCompleted:
		return acp.PlanEntryStatusCompleted
	default:
		return acp.PlanEntryStatusPending
	}
}

func (a *Adapter) OnUsageUpdate(sessionID string, usage agent.StepUsage) {
	a.sessionUpdate(acp.SessionId(sessionID), acp.SessionUpdate{
		UsageUpdate: &acp.SessionUsageUpdate{
			SessionUpdate: "usage_update",
			Used:          usage.Used,
			Size:          usage.Size,
			Cost:          usageCost(usage.Cost),
		},
	})
}

func (a *Adapter) Initialize(ctx context.Context, params acp.InitializeRequest) (acp.InitializeResponse, error) {
	arch := runtime.GOARCH
	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession: true,
			PromptCapabilities: acp.PromptCapabilities{
				Image:           true,
				Audio:           false,
				EmbeddedContext: true,
			},
			SessionCapabilities: acp.SessionCapabilities{
				List:   &acp.SessionListCapabilities{Meta: map[string]any{}},
				Resume: &acp.SessionResumeCapabilities{Meta: map[string]any{"arch": arch}},
				Close:  &acp.SessionCloseCapabilities{Meta: map[string]any{}},
			},
		},
		AgentInfo: &acp.Implementation{
			Name:    "Crush-ACP",
			Version: "acp-0.1.0",
		},
	}, nil
}

func (a *Adapter) NewSession(ctx context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	title := "ACP Session " + time.Now().Format("2006-01-02 15:04")
	if params.Cwd != "" {
		title = "ACP - " + filepath.Base(params.Cwd)
	}
	sess, err := a.sessions.Create(ctx, title)
	if err != nil {
		return acp.NewSessionResponse{}, fmt.Errorf("create session: %w", err)
	}
	titleStr := sess.Title
	a.sessionUpdate(acp.SessionId(sess.ID), acp.SessionUpdate{
		SessionInfoUpdate: &acp.SessionSessionInfoUpdate{
			SessionUpdate: "session_info_update",
			Title:         &titleStr,
		},
	})
	// Send available commands so ACP clients can populate command UI.
	a.sendAvailableCommands(acp.SessionId(sess.ID))
	return acp.NewSessionResponse{SessionId: acp.SessionId(sess.ID)}, nil
}

// sendAvailableCommands sends the list of available slash commands to ACP clients.
// These are built from the registered ACP tools.
func (a *Adapter) sendAvailableCommands(sid acp.SessionId) {
	// Collect available commands from the coordinator's tool registry.
	// The coordinator exposes ACP tools via the ACPConnector.
	commands := a.availableToolCommands()
	if len(commands) == 0 {
		return
	}
	a.sessionUpdate(sid, acp.SessionUpdate{
		AvailableCommandsUpdate: &acp.SessionAvailableCommandsUpdate{
			SessionUpdate:     "available_commands_update",
			AvailableCommands: commands,
		},
	})
}

// availableToolCommands builds the list of AvailableCommand from the
// coordinator's ACP tools. Falls back to static list if coordinator doesn't
// expose tool names.
func (a *Adapter) availableToolCommands() []acp.AvailableCommand {
	if a.coordinator == nil {
		return a.fallbackCommands()
	}
	// Try to get ACP tool names from the coordinator via interface assertion.
	type toolLister interface {
		ACPtoolNames() []string
	}
	if lister, ok := a.coordinator.(toolLister); ok {
		names := lister.ACPtoolNames()
		cmds := make([]acp.AvailableCommand, 0, len(names))
		for _, name := range names {
			cmds = append(cmds, acp.AvailableCommand{
				Name:        name,
				Description: acpToolDescription(name),
			})
		}
		return cmds
	}
	return a.fallbackCommands()
}

// fallbackCommands returns a static list of common Crush commands for when
// the coordinator doesn't expose a tool lister interface.
func (a *Adapter) fallbackCommands() []acp.AvailableCommand {
	return []acp.AvailableCommand{
		{Name: "/submit", Description: "Submit the current prompt to the agent"},
		{Name: "/new", Description: "Start a new session"},
		{Name: "/abort", Description: "Abort the current agent turn"},
		{Name: "/help", Description: "Show available commands"},
	}
}

func acpToolDescription(name string) string {
	switch name {
	case "view":
		return "Read a file or directory"
	case "edit":
		return "Edit a file"
	case "write":
		return "Write a new file"
	case "bash":
		return "Run a shell command"
	case "glob":
		return "Search for files by pattern"
	case "grep":
		return "Search file contents"
	case "todos":
		return "Manage task list"
	default:
		return fmt.Sprintf("Run the %s tool", name)
	}
}

func (a *Adapter) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	sessionID := string(params.SessionId)
	var textParts []string
	var attachments []message.Attachment
	for _, block := range params.Prompt {
		switch {
		case block.Text != nil:
			textParts = append(textParts, block.Text.Text)
		case block.Image != nil:
			data, _ := base64.StdEncoding.DecodeString(block.Image.Data)
			attachments = append(attachments, message.Attachment{Content: data, MimeType: block.Image.MimeType})
		case block.Audio != nil:
			data, _ := base64.StdEncoding.DecodeString(block.Audio.Data)
			attachments = append(attachments, message.Attachment{Content: data, MimeType: block.Audio.MimeType})
		case block.ResourceLink != nil:
			textParts = append(textParts, fmt.Sprintf("[resource: %s (%s)]", block.ResourceLink.Name, block.ResourceLink.Uri))
		case block.Resource != nil:
			if block.Resource.Resource.TextResourceContents != nil {
				textParts = append(textParts, fmt.Sprintf("--- %s ---\n%s",
					block.Resource.Resource.TextResourceContents.Uri,
					block.Resource.Resource.TextResourceContents.Text,
				))
			}
		}
	}
	promptText := strings.Join(textParts, "\n")
	a.sessionUpdate(params.SessionId, acp.UpdateAgentMessageText(""))
	_, err := a.coordinator.Run(ctx, sessionID, promptText, attachments...)
	if err != nil {
		a.sessionUpdate(params.SessionId, acp.UpdateAgentMessageText("Error: "+err.Error()))
		// Still flush orphaned tool calls on error — the model may have
		// started tool calls before the error occurred.
		a.flushOrphanedToolCalls(params.SessionId)
		return acp.PromptResponse{}, err
	}

	// Flush any orphaned tool calls that received OnToolInputStart but no
	// matching OnToolResult (e.g. model was cancelled mid-tool-call).
	a.flushOrphanedToolCalls(params.SessionId)

	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

// flushOrphanedToolCalls sends completed updates for any tool calls that
// were started but never resolved during the prompt turn.
func (a *Adapter) flushOrphanedToolCalls(sessionID acp.SessionId) {
	for toolCallID, toolName := range a.toolNames {
		slog.Warn("Flushing orphaned tool call", "session", sessionID, "tool_call_id", toolCallID, "tool_name", toolName)
		kind := a.toolKinds[toolCallID]
		locs := a.toolLocs[toolCallID]
		opts := []acp.ToolCallUpdateOpt{
			acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
		}
		if kind != "" {
			opts = append(opts, acp.WithUpdateKind(kind))
		}
		if len(locs) > 0 {
			opts = append(opts, acp.WithUpdateLocations(locs))
		}
		opts = append(opts, acp.WithUpdateContent([]acp.ToolCallContent{
			acp.ToolContent(acp.TextBlock("Tool call was interrupted")),
		}))
		update := acp.UpdateToolCall(acp.ToolCallId(toolCallID), opts...)
		a.sessionUpdate(sessionID, update)
	}
	a.toolKinds = make(map[string]acp.ToolKind)
	a.toolLocs = make(map[string][]acp.ToolCallLocation)
	a.toolNames = make(map[string]string)
}

func (a *Adapter) Cancel(ctx context.Context, params acp.CancelNotification) error {
	a.coordinator.Cancel(string(params.SessionId))
	return nil
}

func (a *Adapter) CloseSession(ctx context.Context, params acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	return acp.CloseSessionResponse{}, a.sessions.Delete(ctx, string(params.SessionId))
}

func (a *Adapter) ListSessions(ctx context.Context, _ acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	sessions, err := a.sessions.List(ctx)
	if err != nil {
		return acp.ListSessionsResponse{}, err
	}
	infos := make([]acp.SessionInfo, len(sessions))
	for i, s := range sessions {
		title := s.Title
		infos[i] = acp.SessionInfo{
			SessionId: acp.SessionId(s.ID),
			Title:     &title,
		}
	}
	return acp.ListSessionsResponse{Sessions: infos}, nil
}

func (a *Adapter) ResumeSession(ctx context.Context, params acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	_, err := a.sessions.Get(ctx, string(params.SessionId))
	return acp.ResumeSessionResponse{}, err
}

func (a *Adapter) UnstableForkSession(ctx context.Context, params acp.UnstableForkSessionRequest) (acp.UnstableForkSessionResponse, error) {
	// Load the parent session to get its working directory and state.
	parentSessionID := string(params.SessionId)
	parentSession, err := a.sessions.Get(ctx, parentSessionID)
	if err != nil {
		return acp.UnstableForkSessionResponse{}, fmt.Errorf("get parent session: %w", err)
	}

	// Create a new session with the same title prefix.
	title := "Fork of " + parentSession.Title
	newSession, err := a.sessions.Create(ctx, title)
	if err != nil {
		return acp.UnstableForkSessionResponse{}, fmt.Errorf("create fork session: %w", err)
	}

	// Copy messages from parent to fork.
	msgs, err := a.messages.List(ctx, parentSessionID)
	if err != nil {
		slog.Warn("Failed to list parent messages for fork", "parent", parentSessionID, "err", err)
	} else {
		for _, msg := range msgs {
			if _, err := a.messages.Create(ctx, newSession.ID, message.CreateMessageParams{
				Role:     msg.Role,
				Parts:    msg.Parts,
				Model:    msg.Model,
				Provider: msg.Provider,
			}); err != nil {
				slog.Warn("Failed to copy message to fork", "msg_id", msg.ID, "err", err)
			}
		}
	}

	titleStr := newSession.Title
	sessionID := acp.SessionId(newSession.ID)
	a.sessionUpdate(sessionID, acp.SessionUpdate{
		SessionInfoUpdate: &acp.SessionSessionInfoUpdate{
			SessionUpdate: "session_info_update",
			Title:         &titleStr,
		},
	})
	a.sendAvailableCommands(sessionID)

	return acp.UnstableForkSessionResponse{SessionId: sessionID}, nil
}

func (a *Adapter) SetSessionConfigOption(ctx context.Context, _ acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, errors.New("not supported")
}

// UnstableSetSessionModel handles model switching requests from the client.
func (a *Adapter) UnstableSetSessionModel(ctx context.Context, params acp.UnstableSetSessionModelRequest) (acp.UnstableSetSessionModelResponse, error) {
	sessionID := string(params.SessionId)
	if sessionID == "" {
		return acp.UnstableSetSessionModelResponse{}, errors.New("session ID required")
	}

	// Rebuild models for the session.
	if err := a.coordinator.UpdateModels(ctx); err != nil {
		slog.Warn("Failed to update models for session", "session", sessionID, "err", err)
		return acp.UnstableSetSessionModelResponse{}, fmt.Errorf("update models: %w", err)
	}

	return acp.UnstableSetSessionModelResponse{}, nil
}

func (a *Adapter) SetSessionMode(ctx context.Context, _ acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

func (a *Adapter) Authenticate(ctx context.Context, _ acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (a *Adapter) LoadSession(ctx context.Context, params acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	sessionID := string(params.SessionId)

	// Verify the session exists.
	session, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return acp.LoadSessionResponse{}, err
	}

	// Flush any pending debounced message state so reads are consistent.
	if err := a.messages.FlushAll(ctx); err != nil {
		return acp.LoadSessionResponse{}, fmt.Errorf("flush messages: %w", err)
	}

	// Send session info update so ACP clients display the correct title.
	sessTitle := session.Title
	a.sessionUpdate(acp.SessionId(sessionID), acp.SessionUpdate{
		SessionInfoUpdate: &acp.SessionSessionInfoUpdate{
			SessionUpdate: "session_info_update",
			Title:         &sessTitle,
		},
	})

	// Send available commands so ACP clients can populate command UI.
	a.sendAvailableCommands(acp.SessionId(sessionID))

	// Replay conversation history from SQLite as SessionUpdate notifications.
	// Must be sent before the LoadSessionResponse so the client can
	// reconstruct the full conversation view.
	msgs, err := a.messages.List(ctx, sessionID)
	if err != nil {
		return acp.LoadSessionResponse{}, fmt.Errorf("list messages: %w", err)
	}
	for _, msg := range msgs {
		a.replayMessage(acp.SessionId(sessionID), msg)
	}

	return acp.LoadSessionResponse{}, nil
}

// replayMessage emits SessionUpdate notifications for a single message,
// reconstructing the conversation as the client would have seen it live.
func (a *Adapter) replayMessage(sid acp.SessionId, msg message.Message) {
	parts := msg.Parts
	if len(parts) == 0 {
		return
	}

	switch msg.Role {
	case message.User:
		// Reconstruct user message as a UserMessageChunk with text content.
		var text strings.Builder
		for _, part := range parts {
			switch p := part.(type) {
			case message.TextContent:
				text.WriteString(p.Text)
			case message.ImageURLContent:
				fmt.Fprintf(&text, "\n[image: %s]", p.URL)
			case message.BinaryContent:
				fmt.Fprintf(&text, "\n[image: binary %s]", p.MIMEType)
			}
		}
		if text.Len() > 0 {
			a.sessionUpdate(sid, acp.UpdateUserMessageText(text.String()))
		}

	case message.Assistant:
		// Replay assistant parts as they arrived during the original turn.
		for _, part := range parts {
			switch p := part.(type) {
			case message.TextContent:
				if p.Text != "" {
					a.sessionUpdate(sid, acp.UpdateAgentMessageText(p.Text))
				}
			case message.ReasoningContent:
				if p.Thinking != "" {
					a.sessionUpdate(sid, acp.UpdateAgentThoughtText(p.Thinking))
				}
			case message.ToolCall:
				if p.ID == "" {
					continue
				}
				kind := toolKindFromName(p.Name)
				locs := toolLocationsFromInput(p.Input)
				// Replay as a 2-step lifecycle: StartToolCall(pending) +
				// ToolCallUpdate(completed). Skip in_progress since the
				// tool already completed and we want the final state.
				startOpts := []acp.ToolCallStartOpt{
					acp.WithStartKind(kind),
					acp.WithStartStatus(acp.ToolCallStatusPending),
				}
				if len(locs) > 0 {
					startOpts = append(startOpts, acp.WithStartLocations(locs))
				}
				start := acp.StartToolCall(
					acp.ToolCallId(p.ID), p.Name, startOpts...,
				)
				// Attach visual command meta to the start notification.
				meta := a.buildVisualMeta(kind, locs, p.Name, p.Input)
				if len(meta) > 0 && start.ToolCall != nil {
					start.ToolCall.Meta = meta
				}
				a.sessionUpdate(sid, start)
			}
		}
		// Replay tool results after all tool calls for this message.
		for _, part := range parts {
			if tr, ok := part.(message.ToolResult); ok && tr.ToolCallID != "" {
				a.replayToolResult(sid, tr)
			}
		}

	case message.Tool:
		// Standalone tool result messages (after-tool-call rounds).
		for _, part := range parts {
			if tr, ok := part.(message.ToolResult); ok && tr.ToolCallID != "" {
				a.replayToolResult(sid, tr)
			}
		}
	}
}

// replayToolResult emits a completed ToolCallUpdate for a tool result.
func (a *Adapter) replayToolResult(sid acp.SessionId, tr message.ToolResult) {
	opts := []acp.ToolCallUpdateOpt{
		acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
	}
	if tr.Content != "" {
		content := tr.Content
		if len(content) > 2000 {
			content = content[:2000] + "..."
		}
		opts = append(opts, acp.WithUpdateContent([]acp.ToolCallContent{
			acp.ToolContent(acp.TextBlock(content)),
		}))
	}
	a.sessionUpdate(sid, acp.UpdateToolCall(acp.ToolCallId(tr.ToolCallID), opts...))
}

// UpdateSessionInfo sends a session_info_update to the connected client.
// Called when the session title changes (e.g. auto-generated from first prompt).
func (a *Adapter) UpdateSessionInfo(sessionID, title string) {
	titleStr := title
	a.sessionUpdate(acp.SessionId(sessionID), acp.SessionUpdate{
		SessionInfoUpdate: &acp.SessionSessionInfoUpdate{
			SessionUpdate: "session_info_update",
			Title:         &titleStr,
		},
	})
}

// --- helpers ---

func (a *Adapter) sessionUpdate(sid acp.SessionId, update acp.SessionUpdate) {
	if a.conn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{SessionId: sid, Update: update}); err != nil && a.logger != nil {
		a.logger.Warn("session update failed", "session", sid, "err", err)
	}
}

func toolKindFromName(name string) acp.ToolKind {
	switch name {
	case "view", "zed_view", "ls", "glob", "grep", "rg", "search", "sourcegraph", "lsp_diagnostics", "lsp_references":
		return acp.ToolKindRead
	case "edit", "write", "multiedit", "zed_write":
		return acp.ToolKindEdit
	case "bash", "zed_bash", "job_output", "job_kill":
		return acp.ToolKindExecute
	case "fetch", "download", "web_fetch", "web_search", "agentic_fetch":
		return acp.ToolKindFetch
	case "todos", "list_mcp_resources", "read_mcp_resource", "crush_info", "crush_logs", "lsp_restart":
		return acp.ToolKindThink
	case "agent":
		return acp.ToolKindSwitchMode
	default:
		return acp.ToolKindOther
	}
}

func toolLocationsFromInput(input string) []acp.ToolCallLocation {
	if input == "" {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return nil
	}
	if fp, ok := raw["file_path"].(string); ok && fp != "" {
		line := 0
		if l, ok := raw["offset"].(float64); ok {
			line = int(l)
		}
		return []acp.ToolCallLocation{{Path: fp, Line: &line}}
	}
	if p, ok := raw["path"].(string); ok && p != "" {
		return []acp.ToolCallLocation{{Path: p}}
	}
	return nil
}

func toolCommandFromInput(input string) string {
	if input == "" {
		return ""
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return ""
	}
	if cmd, ok := raw["command"].(string); ok {
		return cmd
	}
	return ""
}

func usageCost(cost float64) *acp.Cost {
	if cost == 0 {
		return nil
	}
	return &acp.Cost{
		Amount:   cost,
		Currency: "USD",
	}
}

func ptr[T any](v T) *T { return &v }

var _ acp.Agent = (*Adapter)(nil)
var _ acp.AgentLoader = (*Adapter)(nil)
