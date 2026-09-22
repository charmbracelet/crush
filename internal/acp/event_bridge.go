// Package acp provides the ACP event bridge that translates Crush pubsub
// events into ACP session/update notifications.
package acp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"

	notify "github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/x/ansi"
	acp "github.com/coder/acp-go-sdk"
)

// chunkDeliveryDelay is the delay between streamed ACP chunk
// notifications. Since the message broker delivers debounced
// full-turn snapshots rather than per-token deltas, spacing chunks
// out makes output appear progressive rather than bursty.
const chunkDeliveryDelay = 50 * time.Millisecond

// eventBridge translates Crush agent events into ACP session updates.
type eventBridge struct {
	conn   *acp.AgentSideConnection
	app    *app.App
	log    *slog.Logger
	cancel context.CancelFunc

	// mu guards the per-message tracking maps below. Message events carry a
	// whole-message snapshot rather than a delta, so the difference against the
	// last snapshot is the new content.
	mu              sync.Mutex
	streamed        map[string]int             // msgID -> text bytes already sent
	thoughts        map[string]int             // msgID -> reasoning bytes already sent
	toolCalls       map[string]map[string]bool // msgID -> toolCallID -> finished
	toolResultSent  map[string]bool            // toolCallID -> content already sent
	finished        map[string]bool            // toolCallID -> terminal status already reported
	attachmentsSent map[string]int             // msgID -> binary content parts already sent
	promptMeta      map[string]map[string]any  // sessionID -> _meta from session/prompt

	// pendingText tracks messages where thinking has started but not finished.
	// Text for these messages is held back until thinking completes (FinishedAt > 0)
	// to avoid interleaving agent_thought_chunk and agent_message_chunk.
	pendingText map[string]bool
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
		conn:            conn,
		app:             app,
		log:             log,
		streamed:        make(map[string]int),
		thoughts:        make(map[string]int),
		toolCalls:       make(map[string]map[string]bool),
		toolResultSent:  make(map[string]bool),
		finished:        make(map[string]bool),
		attachmentsSent: make(map[string]int),
		promptMeta:      make(map[string]map[string]any),
		pendingText:     make(map[string]bool),
	}
}

// SetPromptMeta records the _meta object received on the session/prompt request
// so it is echoed on every outgoing session/update for that turn.
func (b *eventBridge) SetPromptMeta(sessionID string, meta map[string]any) {
	if len(meta) == 0 {
		return
	}
	b.mu.Lock()
	b.promptMeta[sessionID] = meta
	b.mu.Unlock()
}

// note builds a session notification, attaching any prompt-scoped _meta
// captured for the current turn of that session.
func (b *eventBridge) note(sessionID acp.SessionId, update acp.SessionUpdate) acp.SessionNotification {
	note := acp.SessionNotification{SessionId: sessionID, Update: update}
	b.mu.Lock()
	meta := b.promptMeta[string(sessionID)]
	b.mu.Unlock()
	if len(meta) > 0 {
		note.Meta = meta
	}
	return note
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

	b.streamThoughts(ctx, m)

	thinking := m.ReasoningContent()
	hasThinking := len(thinking.Thinking) > 0
	if hasThinking && thinking.FinishedAt == 0 {
		b.mu.Lock()
		b.pendingText[m.ID] = true
		b.mu.Unlock()
		return
	}

	b.mu.Lock()
	delete(b.pendingText, m.ID)
	b.mu.Unlock()

	b.streamText(ctx, m)
	b.streamToolCalls(ctx, m)
	b.streamAttachments(ctx, m)
}

// streamAttachments emits binary content parts (images/audio produced by the
// assistant, e.g. provider-executed tools) as agent_message_chunk content
// blocks (O15). Part counts are tracked per message so re-snapshots do not
// re-emit.
func (b *eventBridge) streamAttachments(ctx context.Context, m message.Message) {
	if m.Role != message.Assistant || m.SessionID == "" {
		return
	}
	var bins []message.BinaryContent
	for _, part := range m.Parts {
		if bc, ok := part.(message.BinaryContent); ok && len(bc.Data) > 0 {
			bins = append(bins, bc)
		}
	}
	if len(bins) == 0 {
		return
	}

	b.mu.Lock()
	sent := b.attachmentsSent[m.ID]
	if len(bins) < sent {
		sent = 0
	}
	b.attachmentsSent[m.ID] = len(bins)
	b.mu.Unlock()

	sessionID := acp.SessionId(m.SessionID)
	for _, bc := range bins[sent:] {
		encoded := base64.StdEncoding.EncodeToString(bc.Data)
		var block acp.ContentBlock
		if strings.HasPrefix(bc.MIMEType, "audio/") {
			block = acp.AudioBlock(encoded, bc.MIMEType)
		} else {
			block = acp.ImageBlock(encoded, bc.MIMEType)
		}
		if err := b.conn.SessionUpdate(ctx, b.note(sessionID, acp.UpdateAgentMessage(block))); err != nil {
			b.log.Warn("Failed to send attachment update", "error", err)
		}
	}
}

// streamToolResults surfaces executed tool output on its tool_call_update so
// the client can observe tool results as they complete (O9). Without this a
// tool-heavy turn is silent between the first tool batch and the final
// assistant message, which reads as a stall. The terminal status is reported
// here too: failed for tool errors, completed otherwise (S10). A per-toolCallID
// guard prevents re-emitting the same result when a tool message is
// re-snapshotted.
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

		status := acp.ToolCallStatusCompleted
		text := tr.Content
		if tr.IsError {
			status = acp.ToolCallStatusFailed
			text = "Error: " + text
		}
		opts := []acp.ToolCallUpdateOpt{acp.WithUpdateStatus(status)}
		if text != "" {
			opts = append(opts,
				acp.WithUpdateContent([]acp.ToolCallContent{
					acp.ToolContent(acp.TextBlock(text)),
				}),
				acp.WithUpdateRawOutput(tr.Content),
			)
		}
		note := b.note(acp.SessionId(m.SessionID), acp.UpdateToolCall(acp.ToolCallId(tr.ToolCallID), opts...))
		if err := b.conn.SessionUpdate(ctx, note); err != nil {
			b.log.Warn("Failed to send tool result update", "error", err)
		}

		b.mu.Lock()
		b.finished[tr.ToolCallID] = true
		b.mu.Unlock()
	}
}

// splitDelta splits a streaming delta into ~4-word chunks so the ACP
// client sees smooth, token-sized updates rather than large paragraph
// jumps. Original whitespace is preserved exactly so spaces at chunk
// and delta boundaries are not lost.
func splitDelta(delta string) []string {
	if delta == "" {
		return nil
	}
	var chunks []string
	const wordsPerChunk = 4
	s := delta
	for len(s) > 0 {
		pos := 0
		wordCount := 0
		for wordCount < wordsPerChunk && pos < len(s) {
			for pos < len(s) && isSpace(s[pos]) {
				pos++
			}
			if pos >= len(s) {
				break
			}
			wordCount++
			for pos < len(s) && !isSpace(s[pos]) {
				pos++
			}
		}
		if wordCount < wordsPerChunk {
			chunks = append(chunks, s)
			break
		}
		chunks = append(chunks, s[:pos])
		s = s[pos:]
	}
	return chunks
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\n' || c == '\t'
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
	// Split large streaming deltas into small pieces (~4 words) so Zed
	// sees smooth, token-sized updates rather than paragraph-sized jumps.
	chunks := splitDelta(delta)
	for i, chunk := range chunks {
		update := acp.SessionNotification{
			SessionId: acp.SessionId(m.SessionID),
			Update:    acp.UpdateAgentMessageText(chunk),
		}
		if err := b.conn.SessionUpdate(ctx, update); err != nil {
			b.log.Warn("Failed to send streamed session update", "error", err)
		}
		// Throttle chunk delivery so the client perceives progressive
		// output instead of a single burst. The message broker delivers
		// debounced full-turn snapshots (not per-token deltas), so we space
		// the split chunks out to approximate streaming cadence.
		if i < len(chunks)-1 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(chunkDeliveryDelay):
			}
		}
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
	chunks := splitDelta(delta)
	for i, chunk := range chunks {
		update := acp.SessionNotification{
			SessionId: acp.SessionId(m.SessionID),
			Update:    acp.UpdateAgentThoughtText(chunk),
		}
		if err := b.conn.SessionUpdate(ctx, update); err != nil {
			b.log.Warn("Failed to send streamed session update", "error", err)
		}
		if i < len(chunks)-1 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(chunkDeliveryDelay):
			}
		}
	}
}

// streamToolCalls diffs the tool calls in a message snapshot and emits
// tool_call / tool_call_update notifications as tools start and finish (S1).
// Starts carry a human-readable title, tool kind, file locations, and rawInput
// (S10). The pending status while a tool waits for permission approval is
// emitted by the permission bridge (see permission.go).
func (b *eventBridge) streamToolCalls(ctx context.Context, m message.Message) {
	type action struct {
		id       string
		name     string
		input    string
		finished bool
	}
	var starts, updates []action

	b.mu.Lock()
	seen := b.toolCalls[m.ID]
	if seen == nil {
		seen = make(map[string]bool)
		b.toolCalls[m.ID] = seen
	}
	for _, tc := range m.ToolCalls() {
		// A terminal status reported via the tool-result path wins; skip the
		// redundant completed update for this call.
		if b.finished[tc.ID] {
			seen[tc.ID] = true
			continue
		}
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
			acp.WithStartRawInput(toolRawInput(a.input)),
		}
		if locs := toolLocations(a.input); len(locs) > 0 {
			opts = append(opts, acp.WithStartLocations(locs))
		}
		if a.input != "" {
			opts = append(opts, acp.WithStartContent([]acp.ToolCallContent{
				acp.ToolContent(acp.TextBlock(a.input)),
			}))
		}
		note := b.note(sessionID, acp.StartToolCall(acp.ToolCallId(a.id), toolTitle(a.name, a.input), opts...))
		if err := b.conn.SessionUpdate(ctx, note); err != nil {
			b.log.Warn("Failed to send tool call update", "error", err)
		}
	}
	for _, a := range updates {
		update := acp.SessionNotification{
			SessionId: sessionID,
			Update:    acp.UpdateToolCall(acp.ToolCallId(a.id), acp.WithUpdateStatus(acp.ToolCallStatusCompleted)),
		}
		if err := b.conn.SessionUpdate(ctx, update); err != nil {
			b.log.Warn("Failed to send tool call update", "error", err)
		}
	}
}

// toolTitle derives a human-readable tool_call title from the tool name and
// its JSON input. The first meaningful argument (path, command, query, ...) is
// appended so clients can show what the tool is acting on.
func toolTitle(name, input string) string {
	arg := toolPrimaryArg(input)
	if arg == "" {
		return capitalize(name)
	}
	arg = ansi.Truncate(arg, 80, "...")
	return capitalize(name) + ": " + arg
}

// toolPrimaryArg extracts the most descriptive string field from a tool input
// JSON object for display in the tool title.
func toolPrimaryArg(input string) string {
	var obj map[string]any
	if err := json.Unmarshal([]byte(input), &obj); err != nil {
		return ""
	}
	for _, key := range []string{"command", "path", "file_path", "pattern", "query", "url", "description"} {
		if v, ok := obj[key].(string); ok && v != "" {
			return strings.ReplaceAll(v, "\n", " ")
		}
	}
	return ""
}

// toolLocations extracts file paths from a tool input JSON object as ACP
// tool-call locations so clients can follow the agent across files.
func toolLocations(input string) []acp.ToolCallLocation {
	var obj map[string]any
	if err := json.Unmarshal([]byte(input), &obj); err != nil {
		return nil
	}
	var locs []acp.ToolCallLocation
	for _, key := range []string{"path", "file_path", "notebook_path"} {
		if v, ok := obj[key].(string); ok && v != "" {
			locs = append(locs, acp.ToolCallLocation{Path: v})
		}
	}
	return locs
}

// toolRawInput decodes a tool input JSON string into a raw JSON value for the
// tool_call rawInput field. Non-JSON input is passed through as a string.
func toolRawInput(input string) any {
	if input == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return input
	}
	return v
}

// capitalize uppercases the first rune of s.
func capitalize(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	return string(unicode.ToUpper(r[0])) + string(r[1:])
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

	b.mu.Lock()
	b.pendingText = make(map[string]bool)
	delete(b.promptMeta, rc.SessionID)
	b.mu.Unlock()
}

// sendText emits an agent_message_chunk carrying the given text.
func (b *eventBridge) sendText(ctx context.Context, sessionID acp.SessionId, text string) {
	if err := b.conn.SessionUpdate(ctx, b.note(sessionID, acp.UpdateAgentMessageText(text))); err != nil {
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
		cfg := store.Config()
		if model := cfg.GetModelByType(cfg.Agents[config.AgentCoder].Model); model != nil {
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
