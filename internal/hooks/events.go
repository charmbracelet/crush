package hooks

import "strings"

// Hook event names.
const (
	// EventPreToolUse fires before a tool runs. Hooks can allow, deny,
	// halt, rewrite the input, or add context.
	EventPreToolUse = "PreToolUse"
	// EventPostToolUse fires after a tool succeeds. Hooks can replace
	// the output, add context, or halt.
	EventPostToolUse = "PostToolUse"
	// EventPostToolUseFailure fires after a tool fails. Hooks can add
	// context or halt.
	EventPostToolUseFailure = "PostToolUseFailure"
	// EventPermissionRequest fires when a permission prompt is shown.
	EventPermissionRequest = "PermissionRequest"
	// EventPrePermission fires right before the permission service is
	// about to prompt the user for a tool call. Returning allow or deny
	// from these hooks short-circuits the prompt entirely; silence falls
	// through to the normal prompt.
	EventPrePermission = "PrePermission"
	// EventPermissionDenied fires after a permission request has been
	// denied, so hosts can react programmatically (notify, log, disable).
	// It carries the permission request payload. Fire-and-forget: the
	// hook does not block the denial.
	EventPermissionDenied = "PermissionDenied"
	// EventPermissionResult fires when a permission prompt is resolved
	// (granted, denied, or cancelled).
	EventPermissionResult = "PermissionResult"
	// EventSessionStart fires when a session is created or resumed.
	EventSessionStart = "SessionStart"
	// EventSessionEnd fires for the active session when the app exits.
	EventSessionEnd = "SessionEnd"
	// EventTurnStart fires when the agent begins processing a prompt.
	EventTurnStart = "TurnStart"
	// EventTurnEnd fires when the agent finishes a turn.
	EventTurnEnd = "TurnEnd"
	// EventStopFailure fires when a turn ends with a provider error.
	EventStopFailure = "StopFailure"
	// EventInterrupt fires when the user cancels a turn.
	EventInterrupt = "Interrupt"
	// EventPreCompact fires before context compaction. Hooks can block it.
	EventPreCompact = "PreCompact"
	// EventPostCompact fires after context compaction completes.
	EventPostCompact = "PostCompact"
)

// events maps each event name to whether it carries a tool call that a
// matcher can be tested against.
var events = map[string]bool{
	EventPreToolUse:         true,
	EventPostToolUse:        true,
	EventPostToolUseFailure: true,
	EventPermissionDenied:   true,
	EventPermissionRequest:  true,
	EventPrePermission:      true,
	EventPermissionResult:   true,
	EventSessionStart:       false,
	EventSessionEnd:         false,
	EventTurnStart:          false,
	EventTurnEnd:            false,
	EventStopFailure:        false,
	EventInterrupt:          false,
	EventPreCompact:         false,
	EventPostCompact:        false,
}

// eventByKey maps a folded event name (lowercase, underscores removed) to
// its canonical form.
var eventByKey = func() map[string]string {
	m := make(map[string]string, len(events))
	for name := range events {
		m[foldEvent(name)] = name
	}
	return m
}()

func foldEvent(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "_", ""))
}

// NormalizeEvent maps a user-provided event name to its canonical form.
// Matching is case-insensitive and accepts snake_case, so "pre_tool_use"
// and "PRETOOLUSE" both resolve to "PreToolUse". The second result is
// false when the name is not a known event.
func NormalizeEvent(name string) (string, bool) {
	canonical, ok := eventByKey[foldEvent(name)]
	return canonical, ok
}

// EventHasTool reports whether the event carries a tool call, and so
// whether its hooks can use a matcher.
func EventHasTool(name string) bool {
	return events[name]
}

// Event is one hook invocation. Name selects which configured hooks run;
// the remaining fields describe the occasion, and only the ones that are
// set appear in the payload piped to the hook.
type Event struct {
	Name      string
	SessionID string

	// Tool is the call a tool event is about; nil for lifecycle events.
	Tool *ToolCall

	// ToolResponse is the tool's output (PostToolUse).
	ToolResponse *string
	// Error describes the failure (PostToolUseFailure, StopFailure).
	Error string
	// LastAssistantMessage is the assistant's text for the turn (TurnEnd).
	LastAssistantMessage *string
	// Decision is the permission outcome (PermissionResult).
	Decision string
	// Source is why the session started (SessionStart).
	Source string
	// Reason is why the session ended (SessionEnd).
	Reason string
	// Trigger is why compaction ran (PreCompact, PostCompact).
	Trigger string
	// Transcript is a reasoning-blind excerpt of the recent conversation,
	// filled in by the runner for hooks that opt in via
	// include_transcript. Callers do not set it.
	Transcript string
}

// ToolCall identifies the tool call a tool event is about. Input is raw
// JSON.
type ToolCall struct {
	ID    string
	Name  string
	Input string
}

// toolName returns the event's tool name, or "" for lifecycle events.
func (ev Event) toolName() string {
	if ev.Tool == nil {
		return ""
	}
	return ev.Tool.Name
}
