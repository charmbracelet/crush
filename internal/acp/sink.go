package acp

import (
	"context"
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

	resp, err := s.conn.RequestPermission(s.ctx, acp.RequestPermissionRequest{
		SessionId: acp.SessionId(s.sessionID),
		ToolCall: acp.RequestPermissionToolCall{
			ToolCallId: acp.ToolCallId(req.ToolCallID),
			Title:      acp.Ptr(req.Description),
			Kind:       acp.Ptr(acp.ToolKindEdit),
			Status:     acp.Ptr(acp.ToolCallStatusPending),
			Locations:  []acp.ToolCallLocation{{Path: req.Path}},
			RawInput:   req.Params,
		},
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
		update := acp.StartToolCall(
			acp.ToolCallId(tc.ID),
			tc.Name,
			acp.WithStartStatus(acp.ToolCallStatusPending),
		)
		return &update
	}

	update := acp.UpdateToolCall(
		acp.ToolCallId(tc.ID),
		acp.WithUpdateStatus(acp.ToolCallStatusInProgress),
	)
	return &update
}

func (s *Sink) translateToolResult(tr message.ToolResult) *acp.SessionUpdate {
	status := acp.ToolCallStatusCompleted
	if tr.IsError {
		status = acp.ToolCallStatusFailed
	}

	update := acp.UpdateToolCall(
		acp.ToolCallId(tr.ToolCallID),
		acp.WithUpdateStatus(status),
		acp.WithUpdateContent([]acp.ToolCallContent{
			acp.ToolContent(acp.TextBlock(tr.Content)),
		}),
	)
	return &update
}
