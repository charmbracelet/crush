package acp

import (
	"context"
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
	modes      map[string]string
	sessionCWD map[string]string
}

// App is the subset of app.App the ACP handler needs.
type App interface {
	GetSessions() session.Service
	GetMessages() message.Service
	GetCoordinator() agent.Coordinator
	GetConfig() *config.ConfigStore
	GetPermissions() permission.Service
}

// NewHandler constructs a Handler backed by the given App.
func NewHandler(app App) *Handler {
	return &Handler{
		app:        app,
		cancels:    make(map[string]*cancelEntry),
		modes:      make(map[string]string),
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
	h.setSessionCWD(sess.ID, normalizeSessionCWD(params.CWD))

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
		return nil, &RPCError{Code: CodeInternalError, Message: fmt.Sprintf("session not found: %v", err)}
	}

	// Replay history as session/update notifications before responding so
	// clients can deterministically rebuild transcript state during load.
	h.setSessionCWD(sess.ID, normalizeSessionCWD(params.CWD))
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
				h.sendUpdate(sessionID, SessionUpdate{
					SessionUpdate: SessionUpdateUserMessageChunk,
					Content:       TextBlock(content),
				})
			}
		case message.Assistant:
			content := msg.Content().Text
			if content != "" {
				h.sendUpdate(sessionID, SessionUpdate{
					SessionUpdate: SessionUpdateAgentMessageChunk,
					Content:       TextBlock(content),
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

	// Subscribe to messages and sessions before running to capture streaming updates.
	// Use a dedicated child context so the subscription is cleaned up when
	// this function returns, preventing a slow memory leak.
	subCtx, subCancel := context.WithCancel(ctx)
	defer subCancel()
	msgSub := h.app.GetMessages().Subscribe(subCtx)
	sessionSub := h.app.GetSessions().Subscribe(subCtx)

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

		case event := <-sessionSub:
			if event.Payload.ID != params.SessionID {
				continue
			}
			h.handleSessionEvent(params.SessionID, event)

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
	}
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

func (h *Handler) setModeForSession(sessionID, mode string) {
	h.mu.Lock()
	h.modes[sessionID] = mode
	h.mu.Unlock()
}

func (h *Handler) currentModeForSession(sessionID string) string {
	h.mu.RLock()
	mode := h.modes[sessionID]
	h.mu.RUnlock()
	if mode == "" {
		return "default"
	}
	return mode
}

func (h *Handler) setSessionCWD(sessionID, cwd string) {
	h.mu.Lock()
	h.sessionCWD[sessionID] = cwd
	h.mu.Unlock()
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

func (h *Handler) sessionCWDForSession(sessionID, fallbackCWD string) string {
	h.mu.RLock()
	cwd := h.sessionCWD[sessionID]
	h.mu.RUnlock()
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
	if params.ModeID != "default" && params.ModeID != "yolo" {
		return nil, &RPCError{Code: CodeInvalidParams, Message: fmt.Sprintf("invalid modeId: %s", params.ModeID)}
	}
	h.setModeForSession(params.SessionID, params.ModeID)
	h.app.GetPermissions().SetSessionAutoApprove(params.SessionID, params.ModeID == "yolo")
	h.sendUpdate(params.SessionID, SessionUpdate{
		SessionUpdate: SessionUpdateCurrentModeUpdate,
		CurrentModeID: params.ModeID,
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
			CWD:       h.sessionCWDForSession(s.ID, params.CWD),
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
		if params.Value != "default" && params.Value != "yolo" {
			return nil, &RPCError{Code: CodeInvalidParams, Message: fmt.Sprintf("invalid mode option value: %s", params.Value)}
		}
		h.setModeForSession(params.SessionID, params.Value)
		enableYolo := params.Value == "yolo"
		h.app.GetPermissions().SetSessionAutoApprove(params.SessionID, enableYolo)

		updated := h.buildConfigOptions(params.SessionID)
		h.sendUpdate(params.SessionID, SessionUpdate{
			SessionUpdate: SessionUpdateConfigOptionUpdate,
			ConfigOptions: updated,
		})
		h.sendUpdate(params.SessionID, SessionUpdate{
			SessionUpdate: SessionUpdateCurrentModeUpdate,
			CurrentModeID: params.Value,
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
