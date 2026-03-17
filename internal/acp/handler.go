package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/version"
)

// Handler dispatches incoming ACP requests to the correct methods.
type Handler struct {
	app    App
	server *Server // set after server is constructed (circular reference resolved via setter)

	mu      sync.RWMutex
	cancels map[string]context.CancelFunc
}

// App is the subset of app.App the ACP handler needs.
type App interface {
	GetSessions() session.Service
	GetMessages() message.Service
	GetCoordinator() agent.Coordinator
}

// NewHandler constructs a Handler backed by the given App.
func NewHandler(app App) *Handler {
	return &Handler{
		app:     app,
		cancels: make(map[string]context.CancelFunc),
	}
}

// SetServer wires the server reference so the handler can send notifications
// and outgoing calls.
func (h *Handler) SetServer(s *Server) {
	h.server = s
}

// Handle dispatches an incoming request.
func (h *Handler) Handle(ctx context.Context, req *Request) (any, *RPCError) {
	switch req.Method {
	case "initialize":
		return h.handleInitialize(ctx, req)
	case "session/new":
		return h.handleSessionNew(ctx, req)
	case "session/load":
		return h.handleSessionLoad(ctx, req)
	case "session/prompt":
		return h.handleSessionPrompt(ctx, req)
	case "session/cancel":
		return h.handleSessionCancel(ctx, req)
	default:
		return nil, &RPCError{Code: CodeMethodNotFound, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}
}

// handleInitialize processes the initialize handshake.
func (h *Handler) handleInitialize(_ context.Context, req *Request) (any, *RPCError) {
	var params InitializeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
	}

	slog.Info("ACP: client connected", "client", params.ClientInfo.Name, "version", params.ClientInfo.Version)

	return InitializeResult{
		ProtocolVersion: ProtocolVersion,
		AgentCapabilities: AgentCapabilities{
			LoadSession: true,
			PromptCapabilities: &PromptCapabilities{
				Image:           true,
				EmbeddedContext: true,
			},
			MCP: &MCPCapabilities{
				HTTP: true,
				SSE:  true,
			},
		},
		AgentInfo: AgentInfo{
			Name:    "crush",
			Title:   "Crush",
			Version: version.Version,
		},
		AuthMethods: []string{},
	}, nil
}

// handleSessionNew creates a new session.
func (h *Handler) handleSessionNew(ctx context.Context, req *Request) (any, *RPCError) {
	var params SessionNewParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
		}
	}

	sess, err := h.app.GetSessions().Create(ctx, agent.DefaultSessionName)
	if err != nil {
		return nil, &RPCError{Code: CodeInternalError, Message: fmt.Sprintf("failed to create session: %v", err)}
	}

	// Use the internal session ID as the ACP session ID for simplicity.
	slog.Info("ACP: created session", "session_id", sess.ID)
	return SessionNewResult{SessionID: sess.ID}, nil
}

// handleSessionLoad loads an existing session.
func (h *Handler) handleSessionLoad(ctx context.Context, req *Request) (any, *RPCError) {
	var params SessionLoadParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
	}

	sess, err := h.app.GetSessions().Get(ctx, params.SessionID)
	if err != nil {
		return nil, &RPCError{Code: CodeInternalError, Message: fmt.Sprintf("session not found: %v", err)}
	}

	// Replay history as session/update notifications.
	go h.replayHistory(ctx, sess.ID)

	slog.Info("ACP: loaded session", "session_id", sess.ID)
	return SessionNewResult{SessionID: sess.ID}, nil
}

// replayHistory replays all messages as session/update notifications.
func (h *Handler) replayHistory(ctx context.Context, sessionID string) {
	msgs, err := h.app.GetMessages().List(ctx, sessionID)
	if err != nil {
		slog.Warn("ACP: failed to list messages for replay", "session_id", sessionID, "err", err)
		return
	}
	for _, msg := range msgs {
		switch msg.Role {
		case message.User:
			content := msg.Content().Text
			if content != "" {
				h.sendUpdate(sessionID, SessionUpdate{
					SessionUpdate: SessionUpdateUserMessageChunk,
					Content:       content,
				})
			}
		case message.Assistant:
			content := msg.Content().Text
			if content != "" {
				h.sendUpdate(sessionID, SessionUpdate{
					SessionUpdate: SessionUpdateAgentMessageChunk,
					Content:       content,
				})
			}
			for _, tc := range msg.ToolCalls() {
				h.sendUpdate(sessionID, SessionUpdate{
					SessionUpdate: SessionUpdateToolCall,
					ToolCallID:    tc.ID,
					Title:         tc.Name,
					Kind:          "tool",
					Status:        ToolCallStatusCompleted,
					RawInput:      tc.Input,
				})
			}
		}
	}
}

// handleSessionPrompt runs a prompt turn and streams updates back.
func (h *Handler) handleSessionPrompt(ctx context.Context, req *Request) (any, *RPCError) {
	var params PromptParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
	}

	if params.SessionID == "" {
		return nil, &RPCError{Code: CodeInvalidParams, Message: "sessionId is required"}
	}

	// Build prompt text from content blocks.
	promptText := extractText(params.Prompt)
	if promptText == "" {
		return nil, &RPCError{Code: CodeInvalidParams, Message: "prompt is empty"}
	}

	// Subscribe to messages before running to capture streaming updates.
	msgSub := h.app.GetMessages().Subscribe(ctx)

	// Wrap context with cancellation so session/cancel can stop the run.
	runCtx, cancel := context.WithCancel(ctx)
	h.mu.Lock()
	h.cancels[params.SessionID] = cancel
	h.mu.Unlock()
	defer func() {
		cancel()
		h.mu.Lock()
		delete(h.cancels, params.SessionID)
		h.mu.Unlock()
	}()

	// Track the last-known text length per message for streaming.
	readBytes := make(map[string]int)

	// Run the agent in a goroutine and stream message events.
	type runResult struct {
		result *fantasy.AgentResult
		err    error
	}
	done := make(chan runResult, 1)

	go func() {
		result, err := h.app.GetCoordinator().Run(runCtx, params.SessionID, promptText)
		done <- runResult{result, err}
	}()

	stopReason := StopReasonEndTurn

loop:
	for {
		select {
		case r := <-done:
			if r.err != nil {
				if isContextError(r.err) {
					stopReason = StopReasonCancelled
				} else {
					return nil, &RPCError{Code: CodeInternalError, Message: r.err.Error()}
				}
			}
			// Drain any remaining message events before returning so that
			// trailing stream chunks are not lost.
			for {
				select {
				case event := <-msgSub:
					if event.Payload.SessionID == params.SessionID {
						h.handleMessageEvent(params.SessionID, event.Payload, readBytes)
					}
				default:
					break loop
				}
			}

		case event := <-msgSub:
			msg := event.Payload
			if msg.SessionID != params.SessionID {
				continue
			}
			h.handleMessageEvent(params.SessionID, msg, readBytes)

		case <-ctx.Done():
			stopReason = StopReasonCancelled
			break loop
		}
	}
	return PromptResult{StopReason: stopReason}, nil
}

// handleMessageEvent converts a message update into session/update notifications.
func (h *Handler) handleMessageEvent(sessionID string, msg message.Message, readBytes map[string]int) {
	switch msg.Role {
	case message.Assistant:
		// Stream text content as agent_message_chunk.
		content := msg.Content().Text
		prev := readBytes[msg.ID]
		if len(content) > prev {
			chunk := content[prev:]
			readBytes[msg.ID] = len(content)
			h.sendUpdate(sessionID, SessionUpdate{
				SessionUpdate: SessionUpdateAgentMessageChunk,
				Content:       chunk,
			})
		}

		// Stream reasoning/thinking.
		thinking := msg.ReasoningContent().Thinking
		prevThink := readBytes[msg.ID+":think"]
		if len(thinking) > prevThink {
			chunk := thinking[prevThink:]
			readBytes[msg.ID+":think"] = len(thinking)
			h.sendUpdate(sessionID, SessionUpdate{
				SessionUpdate: SessionUpdateAgentThoughtChunk,
				Content:       chunk,
			})
		}

		// Emit tool call events.
		for _, tc := range msg.ToolCalls() {
			key := msg.ID + ":tc:" + tc.ID
			if _, seen := readBytes[key]; !seen {
				readBytes[key] = 1
				status := ToolCallStatusRunning
				if tc.Finished {
					status = ToolCallStatusCompleted
				}
				h.sendUpdate(sessionID, SessionUpdate{
					SessionUpdate: SessionUpdateToolCall,
					ToolCallID:    tc.ID,
					Title:         tc.Name,
					Kind:          "tool",
					Status:        status,
					RawInput:      tc.Input,
				})
			} else if tc.Finished {
				finishedKey := msg.ID + ":tc:" + tc.ID + ":done"
				if _, done := readBytes[finishedKey]; !done {
					readBytes[finishedKey] = 1
					h.sendUpdate(sessionID, SessionUpdate{
						SessionUpdate: SessionUpdateToolCallUpdate,
						ToolCallID:    tc.ID,
						Title:         tc.Name,
						Kind:          "tool",
						Status:        ToolCallStatusCompleted,
						RawInput:      tc.Input,
					})
				}
			}
		}
	}
}

// handleSessionCancel cancels a running prompt turn.
func (h *Handler) handleSessionCancel(_ context.Context, req *Request) (any, *RPCError) {
	var params SessionCancelParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
	}

	h.mu.RLock()
	cancel, ok := h.cancels[params.SessionID]
	h.mu.RUnlock()

	if ok {
		cancel()
		slog.Info("ACP: cancelled session", "session_id", params.SessionID)
	}
	return struct{}{}, nil
}

// sendUpdate dispatches a session/update notification to the connected client.
func (h *Handler) sendUpdate(sessionID string, update SessionUpdate) {
	if h.server == nil {
		return
	}
	h.server.Notify(context.Background(), "session/update", SessionUpdateNotification{
		SessionID: sessionID,
		Update:    update,
	})
}

// extractText joins all text-type ContentBlocks into a single string.
func extractText(blocks []ContentBlock) string {
	var sb strings.Builder
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(b.Text)
		}
	}
	return sb.String()
}

// isContextError returns true if the error is a context cancellation, deadline,
// or the agent's own request-cancelled sentinel.
func isContextError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, agent.ErrRequestCancelled) {
		return true
	}
	return false
}

// Compile-time check that pubsub.Event[message.Message] is the event type used
// in the message subscription loop.
var _ pubsub.Event[message.Message]
