package acp

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/timeline"
	"github.com/charmbracelet/crush/internal/toolruntime"
	"github.com/charmbracelet/crush/internal/version"
)

// cancelEntry wraps a cancel function for safe concurrent prompt handling.
// It allows the deferred cleanup to verify it's still the active entry
// before deleting from the map.
type cancelEntry struct {
	cancel context.CancelFunc
}

// Handler dispatches incoming ACP requests to the correct methods.
type Handler struct {
	app    App
	server *Server // set after server is constructed (circular reference resolved via setter)

	mu         sync.RWMutex
	cancels    map[string]*cancelEntry
	sessionCWD map[string]string
}

// App is the subset of app.App the ACP handler needs.
type App interface {
	GetSessions() session.Service
	GetMessages() message.Service
	GetCoordinator() agent.Coordinator
	GetConfig() *config.ConfigStore
	GetPermissions() permission.Service
	GetToolRuntime() toolruntime.Service
	GetTimeline() timeline.Service
}

// NewHandler constructs a Handler backed by the given App.
func NewHandler(app App) *Handler {
	return &Handler{
		app:        app,
		cancels:    make(map[string]*cancelEntry),
		sessionCWD: make(map[string]string),
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
	case "session/list":
		return h.handleSessionList(ctx, req)
	case "session/prompt":
		return h.handleSessionPrompt(ctx, req)
	case "session/cancel":
		return h.handleSessionCancel(ctx, req)
	case "session/set_config_option":
		return h.handleSetConfigOption(ctx, req)
	case "session/set_mode":
		return h.handleSetMode(ctx, req)
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
			SessionCapabilities: &SessionCapabilities{
				List: &struct{}{},
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

	requestedCWD := normalizeOptionalSessionCWD(params.CWD)
	sess, err = h.persistSessionCWD(ctx, sess, requestedCWD)
	if err != nil {
		return nil, &RPCError{Code: CodeInternalError, Message: fmt.Sprintf("failed to persist session cwd: %v", err)}
	}

	// Use the internal session ID as the ACP session ID for simplicity.
	slog.Info("ACP: created session", "session_id", sess.ID)
	return SessionNewResult{
		SessionID:     sess.ID,
		ConfigOptions: h.buildConfigOptions(sess.ID),
		Modes:         h.buildModes(sess.ID),
	}, nil
}

// handleSessionLoad loads an existing session.
func (h *Handler) handleSessionLoad(ctx context.Context, req *Request) (any, *RPCError) {
	var params SessionLoadParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
	}

	sess, err := h.app.GetSessions().Get(ctx, params.SessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &RPCError{Code: CodeResourceNotFound, Message: fmt.Sprintf("session not found: %s", params.SessionID)}
		}
		return nil, &RPCError{Code: CodeInternalError, Message: fmt.Sprintf("failed to get session: %v", err)}
	}

	// Replay history as session/update notifications before responding so
	// clients can deterministically rebuild transcript state during load.
	requestedCWD := normalizeOptionalSessionCWD(params.CWD)
	sess, err = h.persistSessionCWD(ctx, sess, requestedCWD)
	if err != nil {
		return nil, &RPCError{Code: CodeInternalError, Message: fmt.Sprintf("failed to persist session cwd: %v", err)}
	}

	h.replayHistory(ctx, sess.ID)

	slog.Info("ACP: loaded session", "session_id", sess.ID)
	return SessionLoadResult{
		ConfigOptions: h.buildConfigOptions(sess.ID),
		Modes:         h.buildModes(sess.ID),
	}, nil
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
				h.sendUpdateWithContext(ctx, sessionID, SessionUpdate{
					SessionUpdate: SessionUpdateUserMessageChunk,
					Content:       TextBlock(content),
				})
			}
		case message.Tool:
			for _, tr := range msg.ToolResults() {
				h.sendUpdateWithContext(ctx, sessionID, h.sessionUpdateFromToolResult(tr))
			}
		case message.Assistant:
			content := msg.Content().Text
			if content != "" {
				h.sendUpdateWithContext(ctx, sessionID, SessionUpdate{
					SessionUpdate: SessionUpdateAgentMessageChunk,
					Content:       TextBlock(content),
				})
			}
			for _, tc := range msg.ToolCalls() {
				h.sendUpdateWithContext(ctx, sessionID, SessionUpdate{
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

	// Subscribe to messages and sessions before running to capture streaming updates.
	// Use a dedicated child context so the subscription is cleaned up when
	// this function returns, preventing a slow memory leak.
	subCtx, subCancel := context.WithCancel(ctx)
	defer subCancel()
	msgSub := h.app.GetMessages().Subscribe(subCtx)
	sessionSub := h.app.GetSessions().Subscribe(subCtx)
	runtimeSub := h.app.GetToolRuntime().Subscribe(subCtx)
	timelineSub := h.app.GetTimeline().Subscribe(subCtx)

	// Wrap context with cancellation so session/cancel can stop the run.
	runCtx, cancel := context.WithCancel(ctx)
	entry := &cancelEntry{cancel: cancel}
	h.mu.Lock()
	// Cancel any previous prompt for this session before overwriting.
	if oldEntry, ok := h.cancels[params.SessionID]; ok {
		oldEntry.cancel()
	}
	h.cancels[params.SessionID] = entry
	h.mu.Unlock()
	defer func() {
		cancel()
		h.mu.Lock()
		// Only delete if this entry is still the active one.
		// This prevents a finishing prompt from removing another prompt's
		// cancel entry when multiple prompts run concurrently for the same session.
		if h.cancels[params.SessionID] == entry {
			delete(h.cancels, params.SessionID)
		}
		h.mu.Unlock()
	}()

	// Track the last-known text length per message for streaming.
	readBytes := make(map[string]int)
	runtimeSnapshotHashes := make(map[string][32]byte)
	trackedSessionIDs := map[string]struct{}{params.SessionID: {}}

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
			// Drain any remaining subscription events before returning so that
			// trailing stream chunks are not lost.
			for {
				drained := false
				select {
				case event := <-msgSub:
					drained = true
					if h.shouldForwardSessionEvent(subCtx, params.SessionID, event.Payload.SessionID, trackedSessionIDs) {
						h.handleMessageEvent(params.SessionID, event.Payload, readBytes)
					}
				default:
				}

				select {
				case event := <-sessionSub:
					drained = true
					if event.Payload.ID == params.SessionID {
						h.handleSessionEvent(params.SessionID, event)
						continue
					}
					if event.Payload.ParentSessionID == params.SessionID {
						trackedSessionIDs[event.Payload.ID] = struct{}{}
					}
				default:
				}

				select {
				case event := <-runtimeSub:
					drained = true
					if h.shouldForwardSessionEvent(subCtx, params.SessionID, event.Payload.SessionID, trackedSessionIDs) {
						h.handleToolRuntimeEvent(params.SessionID, event, runtimeSnapshotHashes)
					}
				default:
				}

				select {
				case event := <-timelineSub:
					drained = true
					if h.shouldForwardSessionEvent(subCtx, params.SessionID, event.Payload.SessionID, trackedSessionIDs) {
						h.handleTimelineEvent(params.SessionID, event)
					}
				default:
				}

				if !drained {
					break loop
				}
			}

		case event := <-msgSub:
			msg := event.Payload
			if !h.shouldForwardSessionEvent(subCtx, params.SessionID, msg.SessionID, trackedSessionIDs) {
				continue
			}
			h.handleMessageEvent(params.SessionID, msg, readBytes)

		case event := <-sessionSub:
			if event.Payload.ID == params.SessionID {
				h.handleSessionEvent(params.SessionID, event)
				continue
			}
			if event.Payload.ParentSessionID == params.SessionID {
				trackedSessionIDs[event.Payload.ID] = struct{}{}
			}

		case event := <-runtimeSub:
			if !h.shouldForwardSessionEvent(subCtx, params.SessionID, event.Payload.SessionID, trackedSessionIDs) {
				continue
			}
			h.handleToolRuntimeEvent(params.SessionID, event, runtimeSnapshotHashes)

		case event := <-timelineSub:
			if !h.shouldForwardSessionEvent(subCtx, params.SessionID, event.Payload.SessionID, trackedSessionIDs) {
				continue
			}
			h.handleTimelineEvent(params.SessionID, event)

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
				Content:       TextBlock(chunk),
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
				Content:       TextBlock(chunk),
			})
		}

		// Emit tool call events.
		for _, tc := range msg.ToolCalls() {
			key := msg.ID + ":tc:" + tc.ID
			if _, seen := readBytes[key]; !seen {
				readBytes[key] = 1
				status := ToolCallStatusInProgress
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
	case message.Tool:
		for _, tr := range msg.ToolResults() {
			key := msg.ID + ":tr:" + tr.ToolCallID
			if _, seen := readBytes[key]; !seen {
				readBytes[key] = 1
				h.sendUpdate(sessionID, h.sessionUpdateFromToolResult(tr))
			}
		}
	}
}

func (h *Handler) handleToolRuntimeEvent(sessionID string, event pubsub.Event[toolruntime.State], snapshotHashes map[string][32]byte) {
	if event.Type == pubsub.DeletedEvent {
		return
	}

	state := event.Payload
	if background, _ := state.ClientMetadata["background"].(bool); background {
		return
	}

	if state.Status == toolruntime.StatusRunning {
		snapshot := strings.TrimSpace(state.SnapshotText)
		if snapshot == "" {
			return
		}

		hash := sha256.Sum256([]byte(snapshot))
		if prev, ok := snapshotHashes[state.ToolCallID]; ok && prev == hash {
			return
		}
		snapshotHashes[state.ToolCallID] = hash

		h.sendUpdate(sessionID, SessionUpdate{
			SessionUpdate: SessionUpdateToolCallUpdate,
			ToolCallID:    state.ToolCallID,
			Title:         state.ToolName,
			Kind:          "tool",
			Status:        ToolCallStatusInProgress,
			Content:       TextBlock(snapshot),
		})
		return
	}

	if state.Status != toolruntime.StatusCompleted && state.Status != toolruntime.StatusFailed && state.Status != toolruntime.StatusCanceled {
		return
	}
	delete(snapshotHashes, state.ToolCallID)

	status := ToolCallStatusCompleted
	if state.Status == toolruntime.StatusFailed {
		status = ToolCallStatusFailed
	} else if state.Status == toolruntime.StatusCanceled {
		status = ToolCallStatusCanceled
	}

	h.sendUpdate(sessionID, SessionUpdate{
		SessionUpdate: SessionUpdateToolCallUpdate,
		ToolCallID:    state.ToolCallID,
		Title:         state.ToolName,
		Kind:          "tool",
		Status:        status,
	})
}

// handleSessionEvent converts a session update into session/update notifications.
func (h *Handler) handleSessionEvent(sessionID string, event pubsub.Event[session.Session]) {
	sess := event.Payload
	h.sendUpdate(sessionID, SessionUpdate{
		SessionUpdate: SessionUpdateSessionInfoUpdate,
		Title:         sess.Title,
		UpdatedAt:     time.Unix(sess.UpdatedAt, 0).UTC().Format(time.RFC3339),
	})
}

func (h *Handler) handleTimelineEvent(sessionID string, event pubsub.Event[timeline.Event]) {
	if event.Type == pubsub.DeletedEvent {
		return
	}
	h.sendUpdate(sessionID, SessionUpdate{
		SessionUpdate: SessionUpdateTimelineEvent,
		TimelineEvent: timelineEventPayload(event.Payload),
	})
}

func (h *Handler) sessionUpdateFromToolResult(tr message.ToolResult) SessionUpdate {
	status := ToolCallStatusCompleted
	subtaskResult, hasSubtaskResult := tr.SubtaskResult()
	if hasSubtaskResult {
		switch subtaskResult.Status {
		case message.ToolResultSubtaskStatusFailed:
			status = ToolCallStatusFailed
		case message.ToolResultSubtaskStatusCanceled:
			status = ToolCallStatusCanceled
		}
	} else if tr.IsError {
		status = ToolCallStatusFailed
	}

	update := SessionUpdate{
		SessionUpdate: SessionUpdateToolCallUpdate,
		ToolCallID:    tr.ToolCallID,
		Title:         tr.Name,
		Kind:          "tool",
		Status:        status,
		RawOutput:     tr,
	}
	if hasSubtaskResult {
		update.ChildSessionID = subtaskResult.ChildSessionID
		update.ParentToolCallID = subtaskResult.ParentToolCallID
		update.SubtaskResult = &SubtaskResult{Status: string(subtaskResult.Status)}
	}
	return update
}

// handleSessionCancel cancels a running prompt turn.
func (h *Handler) handleSessionCancel(_ context.Context, req *Request) (any, *RPCError) {
	var params SessionCancelParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
	}

	h.mu.RLock()
	entry, ok := h.cancels[params.SessionID]
	h.mu.RUnlock()

	if ok {
		entry.cancel()
		slog.Info("ACP: cancelled session", "session_id", params.SessionID)
	}
	return struct{}{}, nil
}

// sendUpdate dispatches a session/update notification to the connected client.
func (h *Handler) sendUpdate(sessionID string, update SessionUpdate) {
	h.sendUpdateWithContext(context.Background(), sessionID, update)
}

func (h *Handler) sendUpdateWithContext(ctx context.Context, sessionID string, update SessionUpdate) {
	if h.server == nil {
		return
	}
	h.server.Notify(ctx, "session/update", SessionUpdateNotification{
		SessionID: sessionID,
		Update:    update,
	})
}

func (h *Handler) sendUpdateSyncWithContext(ctx context.Context, sessionID string, update SessionUpdate) {
	if h.server == nil {
		return
	}
	if err := h.server.NotifySync(ctx, "session/update", SessionUpdateNotification{
		SessionID: sessionID,
		Update:    update,
	}); err != nil {
		slog.Warn("ACP: failed to write session update", "session_id", sessionID, "err", err)
	}
}

func (h *Handler) shouldForwardSessionEvent(ctx context.Context, parentSessionID string, candidateSessionID string, trackedSessionIDs map[string]struct{}) bool {
	if candidateSessionID == parentSessionID {
		trackedSessionIDs[candidateSessionID] = struct{}{}
		return true
	}
	if _, ok := trackedSessionIDs[candidateSessionID]; ok {
		return true
	}
	if candidateSessionID == "" {
		return false
	}
	candidate, err := h.app.GetSessions().Get(ctx, candidateSessionID)
	if err != nil {
		return false
	}
	if candidate.ParentSessionID != parentSessionID {
		return false
	}
	trackedSessionIDs[candidateSessionID] = struct{}{}
	return true
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

func (h *Handler) currentModeForSession(sessionID string) string {
	if strings.TrimSpace(sessionID) == "" {
		return "default"
	}
	sess, err := h.app.GetSessions().Get(context.Background(), sessionID)
	if err != nil {
		return "default"
	}
	return session.ModeStateFromSession(sess).CurrentModeID()
}

func (h *Handler) setSessionCWD(sessionID, cwd string) {
	h.mu.Lock()
	h.sessionCWD[sessionID] = cwd
	h.mu.Unlock()
}

func normalizeOptionalSessionCWD(cwd string) string {
	if strings.TrimSpace(cwd) == "" {
		return ""
	}
	return normalizeSessionCWD(cwd)
}

func normalizeSessionCWD(cwd string) string {
	if cwd == "" {
		cwd = "."
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return cwd
	}
	return abs
}

func (h *Handler) persistSessionCWD(ctx context.Context, sess session.Session, cwd string) (session.Session, error) {
	if cwd != "" && cwd != sess.WorkspaceCWD {
		sess.WorkspaceCWD = cwd
		saved, err := h.app.GetSessions().Save(ctx, sess)
		if err != nil {
			return session.Session{}, err
		}
		sess = saved
	}
	h.setSessionCWD(sess.ID, normalizeOptionalSessionCWD(sess.WorkspaceCWD))
	return sess, nil
}

func (h *Handler) sessionCWDForSession(sess session.Session, fallbackCWD string) string {
	h.mu.RLock()
	cwd := h.sessionCWD[sess.ID]
	h.mu.RUnlock()
	if stored := normalizeOptionalSessionCWD(sess.WorkspaceCWD); stored != "" {
		return stored
	}
	if cwd != "" {
		return cwd
	}
	return normalizeSessionCWD(fallbackCWD)
}

func (h *Handler) buildModes(sessionID string) *SessionModeState {
	return &SessionModeState{
		CurrentModeID: h.currentModeForSession(sessionID),
		AvailableModes: []SessionMode{
			{
				ID:          "default",
				Name:        "Default",
				Description: "Ask for permission when required",
			},
			{
				ID:          "auto",
				Name:        "Auto",
				Description: "Guarded autonomy with manual fallback",
			},
			{
				ID:          "yolo",
				Name:        "YOLO",
				Description: "Auto-approve tool permissions in this session",
			},
		},
	}
}

func (h *Handler) handleSetMode(_ context.Context, req *Request) (any, *RPCError) {
	var params SetModeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
	}
	if params.ModeID != "default" && params.ModeID != "auto" && params.ModeID != "yolo" {
		return nil, &RPCError{Code: CodeInvalidParams, Message: fmt.Sprintf("invalid modeId: %s", params.ModeID)}
	}
	if params.ModeID == "auto" {
		h.app.GetPermissions().ClearPersistentPermissions(params.SessionID)
	}
	current, err := h.app.GetSessions().Get(context.Background(), params.SessionID)
	if err != nil {
		return nil, &RPCError{Code: CodeInternalError, Message: err.Error()}
	}
	transition := session.NewPermissionModeTransition(current, session.NormalizePermissionMode(params.ModeID))
	if _, err := h.app.GetSessions().UpdatePermissionMode(context.Background(), params.SessionID, transition.Current.PermissionMode); err != nil {
		return nil, &RPCError{Code: CodeInternalError, Message: err.Error()}
	}
	if transition.ExitedAutoMode() {
		if _, err := h.app.GetMessages().Create(context.Background(), params.SessionID, message.NewAutoModePromptMessage(message.AutoModePromptTypeExit)); err != nil {
			return nil, &RPCError{Code: CodeInternalError, Message: err.Error()}
		}
	}
	h.sendUpdate(params.SessionID, SessionUpdate{
		SessionUpdate: SessionUpdateCurrentModeUpdate,
		CurrentModeID: transition.Current.CurrentModeID(),
	})
	h.sendUpdate(params.SessionID, SessionUpdate{
		SessionUpdate: SessionUpdateConfigOptionUpdate,
		ConfigOptions: h.buildConfigOptions(params.SessionID),
	})
	return struct{}{}, nil
}

// buildConfigOptions builds the config options for the session.
// It returns available models that the user can select from.
func (h *Handler) buildConfigOptions(sessionID string) []ConfigOption {
	cfg := h.app.GetConfig()
	if cfg == nil {
		return nil
	}

	var options []ConfigOptionVariant
	seenModels := make(map[string]bool)

	// Add enabled providers' models.
	for _, provider := range cfg.Config().EnabledProviders() {
		for _, model := range provider.Models {
			key := provider.ID + ":" + model.ID
			if seenModels[key] {
				continue
			}
			seenModels[key] = true

			name := model.Name
			if name == "" {
				name = model.ID
			}
			options = append(options, ConfigOptionVariant{
				Value:       key,
				Name:        name,
				Description: provider.Name,
			})
		}
	}

	// Determine current models.
	currentLarge := cfg.Config().Models[config.SelectedModelTypeLarge]
	currentValue := ""
	if currentLarge.Provider != "" && currentLarge.Model != "" {
		currentValue = currentLarge.Provider + ":" + currentLarge.Model
	}

	currentSmall := cfg.Config().Models[config.SelectedModelTypeSmall]
	currentSmallValue := ""
	if currentSmall.Provider != "" && currentSmall.Model != "" {
		currentSmallValue = currentSmall.Provider + ":" + currentSmall.Model
	}

	if len(options) == 0 {
		return nil
	}

	return []ConfigOption{
		{
			ID:           "model_large",
			Name:         "Large Model",
			Category:     "model",
			Type:         "select",
			CurrentValue: currentValue,
			Options:      options,
		},
		{
			ID:           "model_small",
			Name:         "Small Model",
			Category:     "model",
			Type:         "select",
			CurrentValue: currentSmallValue,
			Options:      options,
		},
		{
			ID:           "mode",
			Name:         "Permission Mode",
			Category:     "mode",
			Type:         "select",
			CurrentValue: h.currentModeForSession(sessionID),
			Options: []ConfigOptionVariant{
				{Value: "default", Name: "Default", Description: "Ask for permission when required"},
				{Value: "auto", Name: "Auto", Description: "Guarded autonomy with manual fallback"},
				{Value: "yolo", Name: "YOLO", Description: "Auto-approve tool permissions in this session"},
			},
		},
	}
}

// handleSessionList lists existing sessions.
func (h *Handler) handleSessionList(ctx context.Context, req *Request) (any, *RPCError) {
	var params SessionListParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
		}
	}

	sessions, err := h.app.GetSessions().List(ctx)
	if err != nil {
		return nil, &RPCError{Code: CodeInternalError, Message: fmt.Sprintf("failed to list sessions: %v", err)}
	}

	entries := make([]SessionListEntry, 0, len(sessions))
	for _, s := range sessions {
		// Skip task/sub-agent sessions (those with a parent).
		if s.ParentSessionID != "" {
			continue
		}
		entry := SessionListEntry{
			SessionID: s.ID,
			CWD:       h.sessionCWDForSession(s, params.CWD),
			Title:     s.Title,
		}
		if s.UpdatedAt != 0 {
			entry.UpdatedAt = time.Unix(s.UpdatedAt, 0).UTC().Format(time.RFC3339)
		}
		entries = append(entries, entry)
	}

	return SessionListResult{Sessions: entries}, nil
}

// handleSetConfigOption handles the session/set_config_option request.
func (h *Handler) handleSetConfigOption(ctx context.Context, req *Request) (any, *RPCError) {
	var params SetConfigOptionParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
	}

	if params.ConfigID != "model_large" && params.ConfigID != "model_small" && params.ConfigID != "mode" {
		return nil, &RPCError{Code: CodeInvalidParams, Message: fmt.Sprintf("unknown config option: %s", params.ConfigID)}
	}

	cfg := h.app.GetConfig()
	if cfg == nil {
		return nil, &RPCError{Code: CodeInternalError, Message: "config not available"}
	}

	if params.ConfigID == "mode" {
		if params.Value != "default" && params.Value != "auto" && params.Value != "yolo" {
			return nil, &RPCError{Code: CodeInvalidParams, Message: fmt.Sprintf("invalid mode option value: %s", params.Value)}
		}
		if params.Value == "auto" {
			h.app.GetPermissions().ClearPersistentPermissions(params.SessionID)
		}
		current, err := h.app.GetSessions().Get(ctx, params.SessionID)
		if err != nil {
			return nil, &RPCError{Code: CodeInternalError, Message: err.Error()}
		}
		transition := session.NewPermissionModeTransition(current, session.NormalizePermissionMode(params.Value))
		if _, err := h.app.GetSessions().UpdatePermissionMode(ctx, params.SessionID, transition.Current.PermissionMode); err != nil {
			return nil, &RPCError{Code: CodeInternalError, Message: err.Error()}
		}
		if transition.ExitedAutoMode() {
			if _, err := h.app.GetMessages().Create(ctx, params.SessionID, message.NewAutoModePromptMessage(message.AutoModePromptTypeExit)); err != nil {
				return nil, &RPCError{Code: CodeInternalError, Message: err.Error()}
			}
		}

		updated := h.buildConfigOptions(params.SessionID)
		h.sendUpdate(params.SessionID, SessionUpdate{
			SessionUpdate: SessionUpdateConfigOptionUpdate,
			ConfigOptions: updated,
		})
		h.sendUpdate(params.SessionID, SessionUpdate{
			SessionUpdate: SessionUpdateCurrentModeUpdate,
			CurrentModeID: transition.Current.CurrentModeID(),
		})
		return SetConfigOptionResult{ConfigOptions: updated}, nil
	}

	// Validate requested value against advertised model options.
	valid := false
	for _, opt := range h.buildConfigOptions(params.SessionID) {
		if opt.ID != params.ConfigID {
			continue
		}
		for _, candidate := range opt.Options {
			if candidate.Value == params.Value {
				valid = true
				break
			}
		}
		break
	}
	if !valid {
		return nil, &RPCError{Code: CodeInvalidParams, Message: fmt.Sprintf("invalid model option value: %s", params.Value)}
	}

	// Parse the value (format: "provider:model").
	parts := strings.SplitN(params.Value, ":", 2)
	if len(parts) != 2 {
		return nil, &RPCError{Code: CodeInvalidParams, Message: "invalid model value format, expected 'provider:model'"}
	}
	providerID, modelID := parts[0], parts[1]
	selectedModel := config.SelectedModel{Provider: providerID, Model: modelID}

	var modelType config.SelectedModelType
	if params.ConfigID == "model_large" {
		modelType = config.SelectedModelTypeLarge
	} else {
		modelType = config.SelectedModelTypeSmall
	}

	if err := h.app.GetCoordinator().PrepareModelSwitch(ctx, params.SessionID, modelType, selectedModel); err != nil {
		return nil, &RPCError{Code: CodeInternalError, Message: err.Error()}
	}

	if params.ConfigID == "model_large" {
		largeCurrent := cfg.Config().Models[config.SelectedModelTypeLarge]
		newLarge := config.SelectedModel{
			Provider:         selectedModel.Provider,
			Model:            selectedModel.Model,
			MaxTokens:        largeCurrent.MaxTokens,
			Temperature:      largeCurrent.Temperature,
			TopP:             largeCurrent.TopP,
			TopK:             largeCurrent.TopK,
			FrequencyPenalty: largeCurrent.FrequencyPenalty,
			PresencePenalty:  largeCurrent.PresencePenalty,
		}
		if err := cfg.UpdatePreferredModel(config.ScopeWorkspace, config.SelectedModelTypeLarge, newLarge); err != nil {
			return nil, &RPCError{Code: CodeInternalError, Message: fmt.Sprintf("failed to update large model: %v", err)}
		}
	} else {
		smallCurrent := cfg.Config().Models[config.SelectedModelTypeSmall]
		newSmall := config.SelectedModel{
			Provider:         selectedModel.Provider,
			Model:            selectedModel.Model,
			MaxTokens:        smallCurrent.MaxTokens,
			Temperature:      smallCurrent.Temperature,
			TopP:             smallCurrent.TopP,
			TopK:             smallCurrent.TopK,
			FrequencyPenalty: smallCurrent.FrequencyPenalty,
			PresencePenalty:  smallCurrent.PresencePenalty,
		}
		if err := cfg.UpdatePreferredModel(config.ScopeWorkspace, config.SelectedModelTypeSmall, newSmall); err != nil {
			return nil, &RPCError{Code: CodeInternalError, Message: fmt.Sprintf("failed to update small model: %v", err)}
		}
	}

	// Refresh the agent's model.
	if err := h.app.GetCoordinator().UpdateModels(ctx); err != nil {
		return nil, &RPCError{Code: CodeInternalError, Message: fmt.Sprintf("failed to refresh agent models: %v", err)}
	}

	updatedConfigOptions := h.buildConfigOptions(params.SessionID)
	// Also notify the client in case its UI only listens to session/update.
	h.sendUpdate(params.SessionID, SessionUpdate{
		SessionUpdate: SessionUpdateConfigOptionUpdate,
		ConfigOptions: updatedConfigOptions,
	})

	return SetConfigOptionResult{
		ConfigOptions: updatedConfigOptions,
	}, nil
}

// Compile-time check that pubsub.Event[message.Message] is the event type used
// in the message subscription loop.
var _ pubsub.Event[message.Message]
