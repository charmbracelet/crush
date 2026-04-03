package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"time"
)

type Event string

const (
	EventPreToolUse         Event = "PreToolUse"
	EventPostToolUse        Event = "PostToolUse"
	EventPostToolUseFailure Event = "PostToolUseFailure"
	EventSubagentStart      Event = "SubagentStart"
	EventSubagentStop       Event = "SubagentStop"
	EventSessionStart       Event = "SessionStart"
	EventSessionEnd         Event = "SessionEnd"
	EventStop               Event = "Stop"
	EventPreCompact         Event = "PreCompact"
	EventPostCompact        Event = "PostCompact"
	EventNotification       Event = "Notification"
	EventUserPromptSubmit   Event = "UserPromptSubmit"
	EventPermissionRequest  Event = "PermissionRequest"
)

type Decision string

const (
	DecisionAllow  Decision = "allow"
	DecisionModify Decision = "modify"
	DecisionDeny   Decision = "deny"
)

type HandlerType string

const (
	HandlerTypeCommand HandlerType = "command"
	HandlerTypeHTTP    HandlerType = "http"
	HandlerTypePrompt  HandlerType = "prompt"
)

type CommandConfig struct {
	Command          string   `json:"command"`
	Args             []string `json:"args,omitempty"`
	Passthrough      bool     `json:"passthrough,omitempty"`
	PassthroughField string   `json:"passthrough_field,omitempty"`
}

type HTTPConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type PromptConfig struct {
	Prompt string `json:"prompt"`
	Model  string `json:"model,omitempty"`
}

type HookConfig struct {
	Name      string         `json:"name"`
	Enabled   *bool          `json:"enabled,omitempty"`
	Events    []Event        `json:"events"`
	Type      HandlerType    `json:"type"`
	TimeoutMs int            `json:"timeout_ms,omitempty"`
	Command   *CommandConfig `json:"command,omitempty"`
	HTTP      *HTTPConfig    `json:"http,omitempty"`
	Prompt    *PromptConfig  `json:"prompt,omitempty"`
}

func (h *HookConfig) IsEnabled() bool {
	if h.Enabled == nil {
		return true
	}
	return *h.Enabled
}

func (h *HookConfig) Timeout() time.Duration {
	if h.TimeoutMs <= 0 {
		return 5 * time.Second
	}
	return time.Duration(h.TimeoutMs) * time.Millisecond
}

type HookInput struct {
	HookEventName string         `json:"hook_event_name"`
	ToolName      string         `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
	ToolResult    string         `json:"tool_result,omitempty"`
	SessionID     string         `json:"session_id,omitempty"`
	AgentID       string         `json:"agent_id,omitempty"`
	AgentType     string         `json:"agent_type,omitempty"`
}

type HookOutput struct {
	Decision        Decision       `json:"decision"`
	ModifiedInput   map[string]any `json:"modified_input,omitempty"`
	Reason          string         `json:"reason,omitempty"`
	FallbackOnError bool           `json:"fallback_on_error,omitempty"`
	FallbackInput   map[string]any `json:"fallback_input,omitempty"`
}

type Handler interface {
	Execute(ctx context.Context, input HookInput) (*HookOutput, error)
}

type registeredHook struct {
	config  HookConfig
	handler Handler
}

type PromptHandler interface {
	RunPrompt(ctx context.Context, prompt string, hookInput HookInput) (*HookOutput, error)
}

type ManagerOption func(*Manager)

func WithPromptHandler(ph PromptHandler) ManagerOption {
	return func(m *Manager) {
		m.promptHandler = ph
	}
}

type Manager struct {
	hooks         map[Event][]*registeredHook
	promptHandler PromptHandler
}

func NewManager(configs []HookConfig, opts ...ManagerOption) (*Manager, error) {
	m := &Manager{
		hooks: make(map[Event][]*registeredHook),
	}
	for _, opt := range opts {
		opt(m)
	}
	for _, cfg := range configs {
		if !cfg.IsEnabled() {
			continue
		}
		handler, err := m.newHandler(cfg)
		if err != nil {
			return nil, fmt.Errorf("hook %q: %w", cfg.Name, err)
		}
		rh := &registeredHook{config: cfg, handler: handler}
		for _, event := range cfg.Events {
			m.hooks[event] = append(m.hooks[event], rh)
		}
	}
	return m, nil
}

func (m *Manager) RunPreToolUse(ctx context.Context, toolName string, toolInput map[string]any, sessionID string) (*HookOutput, error) {
	return m.run(ctx, EventPreToolUse, HookInput{
		HookEventName: string(EventPreToolUse),
		ToolName:      toolName,
		ToolInput:     toolInput,
		SessionID:     sessionID,
	})
}

func (m *Manager) run(ctx context.Context, event Event, input HookInput) (*HookOutput, error) {
	hooks := m.hooks[event]
	if len(hooks) == 0 {
		slog.Debug("No hooks registered for event", "event", event)
		return &HookOutput{Decision: DecisionAllow}, nil
	}

	slog.Debug("Running hooks for event", "event", event, "count", len(hooks))

	currentInput := make(map[string]any, len(input.ToolInput))
	maps.Copy(currentInput, input.ToolInput)
	current := input
	current.ToolInput = currentInput

	anyModified := false
	var fallbackInput map[string]any
	fallbackProtectedKeys := make(map[string]struct{})
	for _, rh := range hooks {
		timeoutCtx, cancel := context.WithTimeout(ctx, rh.config.Timeout())
		output, err := rh.handler.Execute(timeoutCtx, current)
		cancel()

		if err != nil {
			slog.Warn("Hook execution failed, continuing (fail-open)", "hook", rh.config.Name, "error", err)
			continue
		}
		if output == nil {
			continue
		}

		switch output.Decision {
		case DecisionDeny:
			return output, nil
		case DecisionModify:
			anyModified = true
			maps.Copy(current.ToolInput, output.ModifiedInput)
			if fallbackInput != nil {
				for key, value := range output.ModifiedInput {
					if _, protected := fallbackProtectedKeys[key]; protected {
						continue
					}
					fallbackInput[key] = value
				}
			}
			if output.FallbackOnError {
				if fallbackInput == nil {
					fallbackInput = make(map[string]any, len(current.ToolInput))
					maps.Copy(fallbackInput, current.ToolInput)
				}
				for key, value := range output.FallbackInput {
					fallbackInput[key] = value
					fallbackProtectedKeys[key] = struct{}{}
				}
			}
		}
	}

	if anyModified {
		return &HookOutput{
			Decision:        DecisionModify,
			ModifiedInput:   current.ToolInput,
			FallbackOnError: fallbackInput != nil,
			FallbackInput:   fallbackInput,
		}, nil
	}
	return &HookOutput{Decision: DecisionAllow}, nil
}

func (m *Manager) RunPostToolUse(ctx context.Context, toolName string, toolInput map[string]any, toolResult string, sessionID string) {
	m.runNotify(ctx, EventPostToolUse, HookInput{
		HookEventName: string(EventPostToolUse),
		ToolName:      toolName,
		ToolInput:     toolInput,
		ToolResult:    toolResult,
		SessionID:     sessionID,
	})
}

func (m *Manager) RunPostToolUseFailure(ctx context.Context, toolName string, toolInput map[string]any, errMsg string, sessionID string) {
	m.runNotify(ctx, EventPostToolUseFailure, HookInput{
		HookEventName: string(EventPostToolUseFailure),
		ToolName:      toolName,
		ToolInput:     toolInput,
		ToolResult:    errMsg,
		SessionID:     sessionID,
	})
}

func (m *Manager) RunSubagentStart(ctx context.Context, agentID, agentType, sessionID string) {
	m.runNotify(ctx, EventSubagentStart, HookInput{
		HookEventName: string(EventSubagentStart),
		AgentID:       agentID,
		AgentType:     agentType,
		SessionID:     sessionID,
	})
}

func (m *Manager) RunSubagentStop(ctx context.Context, agentID, agentType, sessionID string) {
	m.runNotify(ctx, EventSubagentStop, HookInput{
		HookEventName: string(EventSubagentStop),
		AgentID:       agentID,
		AgentType:     agentType,
		SessionID:     sessionID,
	})
}

func (m *Manager) RunSessionStart(ctx context.Context, sessionID string) {
	m.runNotify(ctx, EventSessionStart, HookInput{
		HookEventName: string(EventSessionStart),
		SessionID:     sessionID,
	})
}

func (m *Manager) RunSessionEnd(ctx context.Context, sessionID string) {
	m.runNotify(ctx, EventSessionEnd, HookInput{
		HookEventName: string(EventSessionEnd),
		SessionID:     sessionID,
	})
}

func (m *Manager) RunPreCompact(ctx context.Context, sessionID string) {
	m.runNotify(ctx, EventPreCompact, HookInput{
		HookEventName: string(EventPreCompact),
		SessionID:     sessionID,
	})
}

func (m *Manager) RunPostCompact(ctx context.Context, sessionID string) {
	m.runNotify(ctx, EventPostCompact, HookInput{
		HookEventName: string(EventPostCompact),
		SessionID:     sessionID,
	})
}

func (m *Manager) RunStop(ctx context.Context, sessionID string) {
	m.runNotify(ctx, EventStop, HookInput{
		HookEventName: string(EventStop),
		SessionID:     sessionID,
	})
}

func (m *Manager) RunNotification(ctx context.Context, sessionID, notificationType, notificationMessage string) {
	m.runNotify(ctx, EventNotification, HookInput{
		HookEventName: string(EventNotification),
		SessionID:     sessionID,
		ToolName:      notificationType,
		ToolResult:    notificationMessage,
	})
}

func (m *Manager) RunUserPromptSubmit(ctx context.Context, sessionID, prompt string) {
	m.runNotify(ctx, EventUserPromptSubmit, HookInput{
		HookEventName: string(EventUserPromptSubmit),
		SessionID:     sessionID,
		ToolResult:    prompt,
	})
}

func (m *Manager) RunPermissionRequest(ctx context.Context, sessionID, toolName string, toolInput map[string]any) {
	m.runNotify(ctx, EventPermissionRequest, HookInput{
		HookEventName: string(EventPermissionRequest),
		SessionID:     sessionID,
		ToolName:      toolName,
		ToolInput:     toolInput,
	})
}

func (m *Manager) runNotify(ctx context.Context, event Event, input HookInput) {
	hooks := m.hooks[event]
	if len(hooks) == 0 {
		return
	}
	slog.Debug("Running notify hooks for event", "event", event, "count", len(hooks))
	for _, rh := range hooks {
		timeoutCtx, cancel := context.WithTimeout(ctx, rh.config.Timeout())
		_, err := rh.handler.Execute(timeoutCtx, input)
		cancel()
		if err != nil {
			slog.Warn("Notify hook execution failed", "hook", rh.config.Name, "event", event, "error", err)
		}
	}
}

func (m *Manager) newHandler(cfg HookConfig) (Handler, error) {
	switch cfg.Type {
	case HandlerTypeCommand:
		if cfg.Command == nil {
			return nil, fmt.Errorf("command config is required for command handler type")
		}
		return newCommandHandler(cfg), nil
	case HandlerTypeHTTP:
		if cfg.HTTP == nil {
			return nil, fmt.Errorf("http config is required for http handler type")
		}
		return newHTTPHandler(cfg), nil
	case HandlerTypePrompt:
		return newPromptHandler(cfg, m.promptHandler)
	default:
		return nil, fmt.Errorf("unknown handler type: %q", cfg.Type)
	}
}
