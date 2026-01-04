package acp

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/coder/acp-go-sdk"
)

// Sink receives events from Crush's pubsub system and translates them to ACP
// session updates.
type Sink struct {
	ctx       context.Context
	cancel    context.CancelFunc
	conn      *acp.AgentSideConnection
	sessionID string

	// Track text deltas per message to avoid re-sending content.
	textOffsets      map[string]int
	reasoningOffsets map[string]int
}

// NewSink creates a new event sink for the given session.
func NewSink(ctx context.Context, conn *acp.AgentSideConnection, sessionID string) *Sink {
	sinkCtx, cancel := context.WithCancel(ctx)
	return &Sink{
		ctx:              sinkCtx,
		cancel:           cancel,
		conn:             conn,
		sessionID:        sessionID,
		textOffsets:      make(map[string]int),
		reasoningOffsets: make(map[string]int),
	}
}

// Start subscribes to messages and permissions, forwarding events to ACP.
func (s *Sink) Start(messages message.Service, permissions permission.Service) {
	// Subscribe to message events.
	go func() {
		msgCh := messages.Subscribe(s.ctx)
		for {
			select {
			case event, ok := <-msgCh:
				if !ok {
					return
				}
				s.HandleMessage(event)
			case <-s.ctx.Done():
				return
			}
		}
	}()

	// Subscribe to permission events.
	go func() {
		permCh := permissions.Subscribe(s.ctx)
		for {
			select {
			case event, ok := <-permCh:
				if !ok {
					return
				}
				s.HandlePermission(event.Payload, permissions)
			case <-s.ctx.Done():
				return
			}
		}
	}()
}

// Stop cancels the sink's subscriptions.
func (s *Sink) Stop() {
	s.cancel()
}

// HandleMessage translates a Crush message event to ACP session updates.
func (s *Sink) HandleMessage(event pubsub.Event[message.Message]) {
	msg := event.Payload

	// Only handle messages for our session.
	if msg.SessionID != s.sessionID {
		return
	}

	for _, part := range msg.Parts {
		update := s.translatePart(msg.ID, msg.Role, part)
		if update == nil {
			continue
		}

		if err := s.conn.SessionUpdate(s.ctx, acp.SessionNotification{
			SessionId: acp.SessionId(s.sessionID),
			Update:    *update,
		}); err != nil {
			slog.Error("Failed to send session update", "error", err)
		}
	}
}

// HandlePermission translates a permission request to an ACP permission request.
func (s *Sink) HandlePermission(req permission.PermissionRequest, permissions permission.Service) {
	// Only handle permissions for our session.
	if req.SessionID != s.sessionID {
		return
	}

	slog.Debug("ACP permission request", "tool", req.ToolName, "action", req.Action)

	// Build the tool call for the permission request.
	toolCall := acp.RequestPermissionToolCall{
		ToolCallId: acp.ToolCallId(req.ToolCallID),
		Title:      acp.Ptr(req.Description),
		Kind:       acp.Ptr(acp.ToolKindEdit),
		Status:     acp.Ptr(acp.ToolCallStatusPending),
		Locations:  []acp.ToolCallLocation{{Path: req.Path}},
		RawInput:   req.Params,
	}

	// For edit tools, include diff content so the client can show the proposed
	// changes.
	if meta := extractEditParams(req.Params); meta != nil && meta.FilePath != "" {
		toolCall.Content = []acp.ToolCallContent{
			acp.ToolDiffContent(meta.FilePath, meta.NewContent, meta.OldContent),
		}
	}

	resp, err := s.conn.RequestPermission(s.ctx, acp.RequestPermissionRequest{
		SessionId: acp.SessionId(s.sessionID),
		ToolCall:  toolCall,
		Options: []acp.PermissionOption{
			{Kind: acp.PermissionOptionKindAllowOnce, Name: "Allow", OptionId: "allow"},
			{Kind: acp.PermissionOptionKindAllowAlways, Name: "Allow always", OptionId: "allow_always"},
			{Kind: acp.PermissionOptionKindRejectOnce, Name: "Deny", OptionId: "deny"},
		},
	})
	if err != nil {
		slog.Error("Failed to request permission", "error", err)
		permissions.Deny(req)
		return
	}

	if resp.Outcome.Cancelled != nil {
		permissions.Deny(req)
		return
	}

	if resp.Outcome.Selected != nil {
		switch string(resp.Outcome.Selected.OptionId) {
		case "allow":
			permissions.Grant(req)
		case "allow_always":
			permissions.GrantPersistent(req)
		default:
			permissions.Deny(req)
		}
	}
}

// editParams holds fields needed for diff content in permission requests.
type editParams struct {
	FilePath   string `json:"file_path"`
	OldContent string `json:"old_content"`
	NewContent string `json:"new_content"`
}

// extractEditParams attempts to extract edit parameters from permission params.
func extractEditParams(params any) *editParams {
	if params == nil {
		return nil
	}

	// Try JSON round-trip to extract fields.
	data, err := json.Marshal(params)
	if err != nil {
		return nil
	}

	var ep editParams
	if err := json.Unmarshal(data, &ep); err != nil {
		return nil
	}

	return &ep
}

// translatePart converts a message part to an ACP session update.
func (s *Sink) translatePart(msgID string, role message.MessageRole, part message.ContentPart) *acp.SessionUpdate {
	switch p := part.(type) {
	case message.TextContent:
		return s.translateText(msgID, role, p)

	case message.ReasoningContent:
		return s.translateReasoning(msgID, p)

	case message.ToolCall:
		return s.translateToolCall(p)

	case message.ToolResult:
		return s.translateToolResult(p)

	case message.Finish:
		// Reset offsets on message finish.
		delete(s.textOffsets, msgID)
		delete(s.reasoningOffsets, msgID)
		return nil

	default:
		return nil
	}
}

func (s *Sink) translateText(msgID string, role message.MessageRole, text message.TextContent) *acp.SessionUpdate {
	// Skip user messages - the client already knows what it sent via the
	// prompt request.
	if role != message.Assistant {
		return nil
	}

	offset := s.textOffsets[msgID]
	if len(text.Text) <= offset {
		return nil
	}

	delta := text.Text[offset:]
	s.textOffsets[msgID] = len(text.Text)

	if delta == "" {
		return nil
	}

	update := acp.UpdateAgentMessageText(delta)
	return &update
}

func (s *Sink) translateReasoning(msgID string, reasoning message.ReasoningContent) *acp.SessionUpdate {
	offset := s.reasoningOffsets[msgID]
	if len(reasoning.Thinking) <= offset {
		return nil
	}

	delta := reasoning.Thinking[offset:]
	s.reasoningOffsets[msgID] = len(reasoning.Thinking)

	if delta == "" {
		return nil
	}

	update := acp.UpdateAgentThoughtText(delta)
	return &update
}

func (s *Sink) translateToolCall(tc message.ToolCall) *acp.SessionUpdate {
	if !tc.Finished {
		opts := []acp.ToolCallStartOpt{
			acp.WithStartStatus(acp.ToolCallStatusPending),
			acp.WithStartKind(toolKind(tc.Name)),
		}

		// Parse input to extract path, title, and raw input.
		title := tc.Name
		if input := parseToolInput(tc.Input); input != nil {
			if input.Path != "" {
				opts = append(opts, acp.WithStartLocations([]acp.ToolCallLocation{{Path: input.Path}}))
			}
			if input.Title != "" {
				title = input.Title
			}
			opts = append(opts, acp.WithStartRawInput(input.Raw))
		}

		update := acp.StartToolCall(acp.ToolCallId(tc.ID), title, opts...)
		return &update
	}

	// Tool finished streaming - update with title and input now available.
	opts := []acp.ToolCallUpdateOpt{
		acp.WithUpdateStatus(acp.ToolCallStatusInProgress),
	}
	if input := parseToolInput(tc.Input); input != nil {
		if input.Title != "" {
			opts = append(opts, acp.WithUpdateTitle(input.Title))
		}
		if input.Path != "" {
			opts = append(opts, acp.WithUpdateLocations([]acp.ToolCallLocation{{Path: input.Path}}))
		}
		opts = append(opts, acp.WithUpdateRawInput(input.Raw))
	}

	update := acp.UpdateToolCall(acp.ToolCallId(tc.ID), opts...)
	return &update
}

// toolInput holds parsed tool call input.
type toolInput struct {
	Path  string
	Title string
	Raw   map[string]any
}

// parseToolInput extracts path and raw input from JSON tool input.
func parseToolInput(input string) *toolInput {
	if input == "" {
		return nil
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return nil
	}

	ti := &toolInput{Raw: raw}

	// Extract path from common field names.
	if path, ok := raw["file_path"].(string); ok {
		ti.Path = path
	} else if path, ok := raw["path"].(string); ok {
		ti.Path = path
	}

	// Extract title/description for display.
	if desc, ok := raw["description"].(string); ok {
		ti.Title = desc
	}

	return ti
}

// toolKind maps Crush tool names to ACP tool kinds.
func toolKind(name string) acp.ToolKind {
	switch name {
	case "view", "ls", "job_output", "lsp_diagnostics":
		return acp.ToolKindRead
	case "edit", "multiedit", "write":
		return acp.ToolKindEdit
	case "bash", "job_kill":
		return acp.ToolKindExecute
	case "grep", "glob", "lsp_references", "sourcegraph", "web_search":
		return acp.ToolKindSearch
	case "fetch", "agentic_fetch", "web_fetch", "download":
		return acp.ToolKindFetch
	default:
		return acp.ToolKindOther
	}
}

// diffMetadata holds fields common to edit tool response metadata.
type diffMetadata struct {
	FilePath   string `json:"file_path"`
	OldContent string `json:"old_content"`
	NewContent string `json:"new_content"`
}

func (s *Sink) translateToolResult(tr message.ToolResult) *acp.SessionUpdate {
	status := acp.ToolCallStatusCompleted
	if tr.IsError {
		status = acp.ToolCallStatusFailed
	}

	// For edit tools with metadata, emit diff content.
	content := []acp.ToolCallContent{acp.ToolContent(acp.TextBlock(tr.Content))}
	var locations []acp.ToolCallLocation

	if !tr.IsError && tr.Metadata != "" {
		switch tr.Name {
		case "edit", "multiedit", "write":
			var meta diffMetadata
			if err := json.Unmarshal([]byte(tr.Metadata), &meta); err == nil && meta.FilePath != "" {
				content = []acp.ToolCallContent{
					acp.ToolDiffContent(meta.FilePath, meta.NewContent, meta.OldContent),
				}
			}
		case "view":
			var meta struct {
				FilePath string `json:"file_path"`
			}
			if err := json.Unmarshal([]byte(tr.Metadata), &meta); err == nil && meta.FilePath != "" {
				locations = []acp.ToolCallLocation{{Path: meta.FilePath}}
			}
		case "ls":
			var meta struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal([]byte(tr.Metadata), &meta); err == nil && meta.Path != "" {
				locations = []acp.ToolCallLocation{{Path: meta.Path}}
			}
		}
	}

	opts := []acp.ToolCallUpdateOpt{
		acp.WithUpdateStatus(status),
		acp.WithUpdateContent(content),
	}
	if len(locations) > 0 {
		opts = append(opts, acp.WithUpdateLocations(locations))
	}

	update := acp.UpdateToolCall(acp.ToolCallId(tr.ToolCallID), opts...)
	return &update
}
