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
	EventPreToolUse Event = "PreToolUse"
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

type HookConfig struct {
	Name    string       `json:"name"`
	Enabled *bool        `json:"enabled,omitempty"`
	Events  []Event      `json:"events"`
	Type    HandlerType  `json:"type"`
	TimeoutMs int        `json:"timeout_ms,omitempty"`
	Command *CommandConfig `json:"command,omitempty"`
	HTTP    *HTTPConfig    `json:"http,omitempty"`
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
	SessionID     string         `json:"session_id,omitempty"`
}

type HookOutput struct {
	Decision      Decision       `json:"decision"`
	ModifiedInput map[string]any `json:"modified_input,omitempty"`
	Reason        string         `json:"reason,omitempty"`
}

type Handler interface {
	Execute(ctx context.Context, input HookInput) (*HookOutput, error)
}

type registeredHook struct {
	config  HookConfig
	handler Handler
}

type Manager struct {
	hooks map[Event][]*registeredHook
}

func NewManager(configs []HookConfig) (*Manager, error) {
	m := &Manager{
		hooks: make(map[Event][]*registeredHook),
	}
	for _, cfg := range configs {
		if !cfg.IsEnabled() {
			continue
		}
		handler, err := newHandler(cfg)
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
		return &HookOutput{Decision: DecisionAllow}, nil
	}

	currentInput := make(map[string]any, len(input.ToolInput))
	maps.Copy(currentInput, input.ToolInput)
	current := input
	current.ToolInput = currentInput

	anyModified := false
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
		}
	}

	if anyModified {
		return &HookOutput{
			Decision:      DecisionModify,
			ModifiedInput: current.ToolInput,
		}, nil
	}
	return &HookOutput{Decision: DecisionAllow}, nil
}

func newHandler(cfg HookConfig) (Handler, error) {
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
	default:
		return nil, fmt.Errorf("unknown handler type: %q", cfg.Type)
	}
}
