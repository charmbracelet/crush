package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
)

// Transcript tuning: how much conversation the hooks see, in messages
// and in characters per entry. The excerpt is intentionally bounded so
// classifier hooks stay cheap.
const (
	maxTranscriptMessages = 60
	maxTranscriptEntryLen = 2000
	transcriptHeadRatio   = 0.65
)

// transcriptProvider returns a reasoning-blind transcript excerpt for
// the given session: user messages and tool calls only. Assistant prose,
// tool outputs, system prompts, and reasoning are stripped so hook
// classifiers judge the conversation, not the model's own argumentation
// or possibly-injected tool results. The excerpt is capped at
// maxTranscriptMessages entries with each entry middle-truncated.
func transcriptProvider(messages message.Service) hooks.TranscriptProvider {
	return func(ctx context.Context, sessionID string) string {
		if messages == nil || sessionID == "" {
			return ""
		}
		all, err := messages.List(ctx, sessionID)
		if err != nil {
			slog.Warn("Transcript provider failed to load session messages", "error", err)
			return ""
		}
		var lines []string
		for _, msg := range all {
			if msg.Role == message.User {
				if text := strings.TrimSpace(msg.Content().Text); text != "" {
					lines = append(lines, "User: "+middleTruncate(text, maxTranscriptEntryLen))
				}
			}
			for _, part := range msg.Parts {
				if tc, ok := part.(message.ToolCall); ok {
					input := strings.TrimSpace(tc.Input)
					if input == "" || input == "{}" {
						input = ""
					} else {
						input = " " + middleTruncate(input, maxTranscriptEntryLen)
					}
					lines = append(lines, "Action: "+tc.Name+input)
				}
			}
		}
		if len(lines) > maxTranscriptMessages {
			lines = lines[len(lines)-maxTranscriptMessages:]
		}
		return strings.Join(lines, "\n")
	}
}

// middleTruncate shortens s to at most max chars by keeping
// transcriptHeadRatio of the budget from the head and the remainder from
// the tail, joined by an ellipsis. Runeware: never splits a rune.
func middleTruncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	head := int(float64(max) * transcriptHeadRatio)
	tail := max - head - 3
	if tail < 1 {
		tail = 1
		head = max - head - 3
	}
	if head < 1 || head+tail >= len(runes) {
		return string(runes[:max-3]) + "..."
	}
	return string(runes[:head]) + "..." + string(runes[len(runes)-tail:])
}

// permissionHookDispatcher bridges the permission service's optional
// policy hooks (PrePermission, PermissionDenied) to the hooks runner.
// It reads hook configs fresh on every dispatch so config reloads apply
// immediately, and delegates transcript context to the shared
// transcriptProvider.
type permissionHookDispatcher struct {
	cfg      *config.ConfigStore
	messages message.Service
}

func newPermissionHookDispatcher(cfg *config.ConfigStore, messages message.Service) *permissionHookDispatcher {
	return &permissionHookDispatcher{cfg: cfg, messages: messages}
}

// PrePermission implements permission.PermissionHooks.
func (d *permissionHookDispatcher) PrePermission(ctx context.Context, req permission.PermissionRequest) permission.PreHookResult {
	cfgs := d.cfg.Config().Hooks[hooks.EventPrePermission]
	if len(cfgs) == 0 {
		return permission.PreHookResult{Decision: permission.HookDecisionNone}
	}
	res, err := d.run(ctx, hooks.EventPrePermission, cfgs, req)
	if err != nil {
		slog.Warn("PrePermission hook execution error, falling through to prompt",
			"tool", req.ToolName, "error", err)
		return permission.PreHookResult{Decision: permission.HookDecisionNone}
	}
	switch res.Decision {
	case hooks.DecisionAllow:
		return permission.PreHookResult{Decision: permission.HookDecisionAllow}
	case hooks.DecisionDeny:
		return permission.PreHookResult{Decision: permission.HookDecisionDeny, Reason: res.Reason}
	default:
		return permission.PreHookResult{Decision: permission.HookDecisionNone}
	}
}

// PermissionDenied implements permission.PermissionHooks. Decisions from
// these hooks are ignored; the event is fire-and-forget observability.
func (d *permissionHookDispatcher) PermissionDenied(ctx context.Context, req permission.PermissionRequest) {
	cfgs := d.cfg.Config().Hooks[hooks.EventPermissionDenied]
	if len(cfgs) == 0 {
		return
	}
	_, err := d.run(ctx, hooks.EventPermissionDenied, cfgs, req)
	if err != nil {
		slog.Warn("PermissionDenied hook execution error", "tool", req.ToolName, "error", err)
	}
}

// run executes the matching hooks for a single permission-related event
// and returns the aggregated result.
func (d *permissionHookDispatcher) run(ctx context.Context, event string, cfgs []config.HookConfig, req permission.PermissionRequest) (hooks.AggregateResult, error) {
	inputJSON := marshalHookParams(req.Params)
	runner := hooks.NewRunner(cfgs, d.cfg.WorkingDir(), d.cfg.WorkingDir())
	if needsTranscript(cfgs) {
		runner = runner.WithTranscriptProvider(transcriptProvider(d.messages))
	}
	return runner.Run(ctx, event, req.SessionID, req.ToolName, inputJSON)
}

// marshalHookParams renders a permission request's Params as the
// tool_input JSON in the hook payload. Non-object values degrade to "{}".
func marshalHookParams(params any) string {
	if params == nil {
		return "{}"
	}
	data, err := json.Marshal(params)
	if err != nil {
		return "{}"
	}
	if len(data) == 0 {
		return "{}"
	}
	if data[0] != '{' {
		return fmt.Sprintf(`{"value":%s}`, data)
	}
	return string(data)
}

// needsTranscript reports whether any hook config opted into transcript
// context.
func needsTranscript(cfgs []config.HookConfig) bool {
	for _, h := range cfgs {
		if h.IncludeTranscript {
			return true
		}
	}
	return false
}
