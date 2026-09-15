// Package acp provides the ACP event bridge that translates Crush pubsub
// events into ACP session/update notifications.
package acp

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	notify "github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	acp "github.com/coder/acp-go-sdk"
)

// eventBridge translates Crush agent events into ACP session updates.
type eventBridge struct {
	conn   *acp.AgentSideConnection
	app    *app.App
	log    *slog.Logger
	cancel context.CancelFunc

	// mu guards the per-message tracking maps below. Message events carry a
	// whole-message snapshot rather than a delta, so the difference against the
	// last snapshot is the new content.
	mu             sync.Mutex
	streamed       map[string]int             // msgID -> text bytes already sent
	thoughts       map[string]int             // msgID -> reasoning bytes already sent
	toolCalls      map[string]map[string]bool // msgID -> toolCallID -> finished
	toolResultSent map[string]bool            // toolCallID -> content already sent
}

// toolKindFor maps a Crush tool name to the closest ACP tool kind.
func toolKindFor(name string) acp.ToolKind {
	switch strings.ToLower(name) {
	case "read", "view", "glob", "grep", "ls", "cat":
		return acp.ToolKindRead
	case "edit", "write", "str_replace":
		return acp.ToolKindEdit
	case "delete", "rm":
		return acp.ToolKindDelete
	case "move", "rename", "mv":
		return acp.ToolKindMove
	case "search":
		return acp.ToolKindSearch
	case "bash", "shell", "execute":
		return acp.ToolKindExecute
	case "fetch", "web_fetch":
		return acp.ToolKindFetch
	case "think":
		return acp.ToolKindThink
	case "switch_mode":
		return acp.ToolKindSwitchMode
	default:
		return acp.ToolKindOther
	}
}

// newEventBridge creates a bridge that listens for agent events and
// forwards them as ACP session/update notifications.
func newEventBridge(conn *acp.AgentSideConnection, app *app.App, log *slog.Logger) *eventBridge {
	return &eventBridge{
		conn:           conn,
		app:            app,
		log:            log,
		streamed:       make(map[string]int),
		thoughts:       make(map[string]int),
		toolCalls:      make(map[string]map[string]bool),
		toolResultSent: make(map[string]bool),
	}
}

// Start begins listening for agent events. It returns when ctx is cancelled.
func (b *eventBridge) Start(ctx context.Context) error {
	ctx, b.cancel = context.WithCancel(ctx)

	// Subscribe to agent notifications (errors, auth events, etc.)
	if b.app.AgentNotifications() != nil {
		go b.subscribeNotifications(ctx, b.app.AgentNotifications())
	}
	// Subscribe to run completions (turn-end signals)
	if b.app.RunCompletions() != nil {
		go b.subscribeRunCompletions(ctx, b.app.RunCompletions())
	}
	// Subscribe to message updates, which is where streaming assistant text
	// actually lives. Without this a successful turn sends the client no
	// session/update at all, and an editor that waits for the first
	// agent_message_chunk shows a loading indicator forever.
	if b.app.Messages != nil {
		go b.subscribeMessages(ctx)
	}

	<-ctx.Done()
	return ctx.Err()
}

// Shutdown stops the event bridge.
func (b *eventBridge) Shutdown() {
	if b.cancel != nil {
		b.cancel()
	}
}

// subscribeMessages forwards assistant text as it streams. Message events hold
// a full snapshot of the message, so what has not yet been sent is the tail
// beyond the offset recorded for that message.
func (b *eventBridge) subscribeMessages(ctx context.Context) {
	ch := b.app.Messages.Subscribe(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ch:
			b.handleMessage(ctx, ev.Payload)
		}
	}
}

func (b *eventBridge) handleMessage(ctx context.Context, m message.Message) {
	b.streamToolResults(ctx, m)
	if m.Role != message.Assistant || m.SessionID == "" {
		return
	}
	b.streamText(ctx, m)
	b.streamThoughts(ctx, m)
	b.streamToolCalls(ctx, m)
}

// streamToolResults surfaces executed tool output on its tool_call_update so
// the client can observe tool results as they complete (O9). Without this a
// tool-heavy turn is silent between the first tool batch and the final
// assistant message, which reads as a stall. Output is attached via the ACP
// tool_call_update raw-output/content fields rather than the agent message
// stream, so it does not pollute the assistant text. A per-toolCallID guard
// prevents re-emitting the same content when a tool message is re-snapshotted.
func (b *eventBridge) streamToolResults(ctx context.Context, m message.Message) {
	if m.Role != message.Tool || m.SessionID == "" {
		return
	}
	for _, part := range m.Parts {
		tr, ok := part.(message.ToolResult)
		if !ok || tr.ToolCallID == "" {
			continue
		}
		b.mu.Lock()
		if b.toolResultSent[tr.ToolCallID] {
			b.mu.Unlock()
			continue
		}
		b.toolResultSent[tr.ToolCallID] = true
		b.mu.Unlock()

		text := tr.Content
		if tr.IsError {
			text = "Error: " + text
		}
		if text == "" {
			continue
		}
		update := acp.UpdateToolCall(
			acp.ToolCallId(tr.ToolCallID),
			acp.WithUpdateContent([]acp.ToolCallContent{
				acp.ToolContent(acp.TextBlock(text)),
			}),
		)
		notification := acp.SessionNotification{
			SessionId: acp.SessionId(m.SessionID),
			Update:    update,
		}
		if err := b.conn.SessionUpdate(ctx, notification); err != nil {
			b.log.Warn("Failed to send tool result update", "error", err)
		}
	}
}

// streamText streams the new tail of the assistant message text.
func (b *eventBridge) streamText(ctx context.Context, m message.Message) {
	text := m.Content().Text

	b.mu.Lock()
	sent := b.streamed[m.ID]
	if len(text) < sent {
		// The message was replaced or rewritten; resend from the start.
		sent = 0
	}
	b.streamed[m.ID] = len(text)
	b.mu.Unlock()

	if len(text) <= sent {
		return
	}
	delta := text[sent:]
	update := acp.SessionNotification{
		SessionId: acp.SessionId(m.SessionID),
		Update:    acp.UpdateAgentMessageText(delta),
	}
	if err := b.conn.SessionUpdate(ctx, update); err != nil {
		b.log.Warn("Failed to send streamed session update", "error", err)
	}
}

// streamThoughts streams reasoning deltas as agent_thought_chunk updates (S3).
func (b *eventBridge) streamThoughts(ctx context.Context, m message.Message) {
	thinking := m.ReasoningContent().Thinking

	b.mu.Lock()
	sent := b.thoughts[m.ID]
	if len(thinking) < sent {
		sent = 0
	}
	b.thoughts[m.ID] = len(thinking)
	b.mu.Unlock()

	if len(thinking) <= sent {
		return
	}
	delta := thinking[sent:]
	update := acp.SessionNotification{
		SessionId: acp.SessionId(m.SessionID),
		Update:    acp.UpdateAgentThoughtText(delta),
	}
	if err := b.conn.SessionUpdate(ctx, update); err != nil {
		b.log.Warn("Failed to send thought update", "error", err)
	}
}

// streamToolCalls diffs the tool calls in a message snapshot and emits
// tool_call / tool_call_update notifications as tools start and finish (S1).
func (b *eventBridge) streamToolCalls(ctx context.Context, m message.Message) {
	type action struct {
		id       string
		name     string
		input    string
		finished bool
		started  bool
	}
	var starts, updates []action

	b.mu.Lock()
	seen := b.toolCalls[m.ID]
	if seen == nil {
		seen = make(map[string]bool)
		b.toolCalls[m.ID] = seen
	}
	for _, tc := range m.ToolCalls() {
		prev, ok := seen[tc.ID]
		if !ok {
			seen[tc.ID] = tc.Finished
			starts = append(starts, action{id: tc.ID, name: tc.Name, input: tc.Input, finished: tc.Finished})
		} else if prev != tc.Finished {
			seen[tc.ID] = tc.Finished
			updates = append(updates, action{id: tc.ID, finished: tc.Finished})
		}
	}
	b.mu.Unlock()

	sessionID := acp.SessionId(m.SessionID)
	for _, a := range starts {
		status := acp.ToolCallStatusInProgress
		if a.finished {
			status = acp.ToolCallStatusCompleted
		}
		opts := []acp.ToolCallStartOpt{
			acp.WithStartStatus(status),
			acp.WithStartKind(toolKindFor(a.name)),
		}
		if a.input != "" {
			opts = append(opts, acp.WithStartContent([]acp.ToolCallContent{
				acp.ToolContent(acp.TextBlock(a.input)),
			}))
		}
		update := acp.SessionNotification{
			SessionId: sessionID,
			Update:    acp.StartToolCall(acp.ToolCallId(a.id), a.name, opts...),
		}
		if err := b.conn.SessionUpdate(ctx, update); err != nil {
			b.log.Warn("Failed to send tool call update", "error", err)
		}
	}
	for _, a := range updates {
		status := acp.ToolCallStatusInProgress
		if a.finished {
			status = acp.ToolCallStatusCompleted
		}
		update := acp.SessionNotification{
			SessionId: sessionID,
			Update:    acp.UpdateToolCall(acp.ToolCallId(a.id), acp.WithUpdateStatus(status)),
		}
		if err := b.conn.SessionUpdate(ctx, update); err != nil {
			b.log.Warn("Failed to send tool call update", "error", err)
		}
	}
}

func (b *eventBridge) subscribeNotifications(ctx context.Context, broker *pubsub.Broker[notify.Notification]) {
	ch := broker.Subscribe(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ch:
			b.handleNotification(ctx, ev.Payload)
		}
	}
}

func (b *eventBridge) subscribeRunCompletions(ctx context.Context, broker *pubsub.Broker[notify.RunComplete]) {
	ch := broker.Subscribe(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ch:
			b.handleRunComplete(ctx, ev.Payload)
		}
	}
}

// handleNotification converts agent notifications to ACP session updates.
func (b *eventBridge) handleNotification(ctx context.Context, n notify.Notification) {
	sessionID := acp.SessionId(n.SessionID)
	switch n.Type {
	case notify.TypeAgentError:
		b.log.Info("ACP: agent error", "sessionId", n.SessionID, "message", n.Message)
		b.sendText(ctx, sessionID, "[Error] "+n.Message)
	case notify.TypeAgentFinished:
		b.log.Info("ACP: agent finished", "sessionId", n.SessionID)
		// No explicit update needed - prompt response handles completion
	case notify.TypeReAuthenticate:
		b.log.Info("ACP: re-auth required", "sessionId", n.SessionID, "provider", n.ProviderID)
		b.sendText(ctx, sessionID, "[Auth Required] Please re-authenticate with "+n.ProviderID)
	default:
		b.log.Debug("ACP: unknown notification type", "type", n.Type)
	}
}

// handleRunComplete sends a final update when a turn completes. On error or
// cancellation it surfaces a short message; on success the prompt response
// already signalled completion. It always emits a usage_update (O13) so the
// client can show token usage and cost for the session.
func (b *eventBridge) handleRunComplete(ctx context.Context, rc notify.RunComplete) {
	sessionID := acp.SessionId(rc.SessionID)
	b.log.Info("ACP: run complete", "sessionId", rc.SessionID, "cancelled", rc.Cancelled)

	switch {
	case rc.Cancelled:
		b.sendText(ctx, sessionID, "[Cancelled] The operation was cancelled.")
	case rc.Error != "":
		b.sendText(ctx, sessionID, "[Error] "+rc.Error)
	}

	b.usageUpdate(ctx, rc.SessionID)
}

// sendText emits an agent_message_chunk carrying the given text.
func (b *eventBridge) sendText(ctx context.Context, sessionID acp.SessionId, text string) {
	update := acp.SessionNotification{
		SessionId: sessionID,
		Update:    acp.UpdateAgentMessageText(text),
	}
	if err := b.conn.SessionUpdate(ctx, update); err != nil {
		b.log.Warn("Failed to send session update", "error", err)
	}
}

// usageUpdate emits a SessionUsageUpdate reporting the session's cumulative
// token usage against the active model's context window, plus cumulative cost
// (O13). It is a best-effort signal: missing usage data is simply omitted.
func (b *eventBridge) usageUpdate(ctx context.Context, sessionID string) {
	session, err := b.app.Sessions.Get(ctx, sessionID)
	if err != nil {
		return
	}

	used := int(session.PromptTokens + session.CompletionTokens)
	cost := &acp.Cost{Amount: session.Cost, Currency: "USD"}

	size := 0
	if store := b.app.Store(); store != nil {
		if model := store.Config().GetModelByType(config.SelectedModelType(config.AgentCoder)); model != nil {
			size = int(model.ContextWindow)
		}
	}

	update := acp.SessionNotification{
		SessionId: acp.SessionId(sessionID),
		Update: acp.SessionUpdate{
			UsageUpdate: &acp.SessionUsageUpdate{
				SessionUpdate: "usage_update",
				Size:          size,
				Used:          used,
				Cost:          cost,
			},
		},
	}
	if err := b.conn.SessionUpdate(ctx, update); err != nil {
		b.log.Warn("Failed to send usage update", "error", err)
	}
}
