// Package hooks runs user-defined shell commands that fire on hook events
// (e.g. PreToolUse), returning decisions that control agent behavior.
package hooks

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/tidwall/sjson"
)

// Hook event name constants.
const (
	// EventPreToolUse fires before each tool execution. Hooks can
	// allow, deny, halt, or rewrite the tool input.
	EventPreToolUse = "PreToolUse"

	// EventSessionStart fires when a new session is created.
	// Carries the session ID for restore/resume support.
	EventSessionStart = "SessionStart"

	// EventTurnStart fires when the agent begins processing a user
	// prompt. Marks the transition to working state.
	EventTurnStart = "TurnStart"

	// EventPermissionRequest fires when a permission prompt is shown
	// to the user. Marks the transition to blocked state.
	EventPermissionRequest = "PermissionRequest"

	// EventPermissionResult fires after a permission decision is made
	// (granted, denied, or cancelled). Marks the transition out of
	// blocked state.
	EventPermissionResult = "PermissionResult"

	// EventTurnEnd fires when the agent finishes a turn and returns
	// to idle state.
	EventTurnEnd = "TurnEnd"

	// EventInterrupt fires when the user cancels or interrupts an
	// active turn (e.g. ctrl-c, escape).
	EventInterrupt = "Interrupt"

	// EventPostToolUse fires after a tool call succeeds. Carries
	// tool name and input for observability. Cannot block.
	EventPostToolUse = "PostToolUse"

	// EventSessionEnd fires when a session ends (app shutdown or
	// explicit session teardown). Carries session ID.
	EventSessionEnd = "SessionEnd"

	// EventStopFailure fires when a turn ends due to an API or
	// provider error (not user cancellation). Carries session ID.
	EventStopFailure = "StopFailure"

	// EventPreCompact fires before context compaction begins.
	// Can halt to prevent compaction.
	EventPreCompact = "PreCompact"

	// EventPostCompact fires after context compaction completes.
	EventPostCompact = "PostCompact"
)

// AllEvents lists every supported hook event name. Used for config
// validation and documentation.
var AllEvents = []string{
	EventPreToolUse,
	EventPostToolUse,
	EventSessionStart,
	EventSessionEnd,
	EventTurnStart,
	EventPermissionRequest,
	EventPermissionResult,
	EventTurnEnd,
	EventInterrupt,
	EventStopFailure,
	EventPreCompact,
	EventPostCompact,
}

// eventNormMap maps lowercased-no-underscores event names to their
// canonical form. Derived from AllEvents so adding a new event only
// requires updating one list.
var eventNormMap = func() map[string]string {
	m := make(map[string]string, len(AllEvents))
	for _, e := range AllEvents {
		m[strings.ToLower(strings.ReplaceAll(e, "_", ""))] = e
	}
	return m
}()

// NormalizeEventName maps user-provided event names to their canonical
// form. Matching is case-insensitive and accepts snake_case variants.
// Returns the input unchanged if no match is found.
func NormalizeEventName(name string) string {
	if canonical, ok := eventNormMap[strings.ToLower(strings.ReplaceAll(name, "_", ""))]; ok {
		return canonical
	}
	return name
}

// IsToolEvent reports whether the event is handled via the tool
// execution path (Runner.Run with matcher filtering). Lifecycle events
// return false.
func IsToolEvent(event string) bool {
	return event == EventPreToolUse || event == EventPostToolUse
}

// HaltExitCode is the exit code that halts the whole turn. 2 blocks the
// current tool call; 49 sits in the no-man's-land between the
// generic-error range (1-30), the sysexits range (64-78), and the
// killed-by-signal range (128+) so it can't be hit by accident.
const HaltExitCode = 49

// HookMetadata is embedded in tool response metadata so the UI can
// display a hook indicator.
type HookMetadata struct {
	HookCount    int        `json:"hook_count"`
	Decision     string     `json:"decision"`
	Halt         bool       `json:"halt,omitempty"`
	Reason       string     `json:"reason,omitempty"`
	InputRewrite bool       `json:"input_rewrite,omitempty"`
	Hooks        []HookInfo `json:"hooks,omitempty"`
}

// HookInfo identifies a single hook that ran and its individual result.
type HookInfo struct {
	Name         string `json:"name"`
	Matcher      string `json:"matcher,omitempty"`
	Decision     string `json:"decision"`
	Halt         bool   `json:"halt,omitempty"`
	Reason       string `json:"reason,omitempty"`
	InputRewrite bool   `json:"input_rewrite,omitempty"`
}

// Decision represents the outcome of a single hook execution.
type Decision int

const (
	// DecisionNone means the hook expressed no opinion.
	DecisionNone Decision = iota
	// DecisionAllow means the hook explicitly allowed the action.
	DecisionAllow
	// DecisionDeny means the hook blocked the action.
	DecisionDeny
)

func (d Decision) String() string {
	switch d {
	case DecisionAllow:
		return "allow"
	case DecisionDeny:
		return "deny"
	default:
		return "none"
	}
}

// HookResult holds the parsed output of a single hook execution.
type HookResult struct {
	Decision     Decision
	Halt         bool   // If true, halt the whole turn.
	Reason       string // Deny or halt reason (same field, different audience).
	Context      string
	UpdatedInput string // Shallow-merge patch against tool_input (opaque JSON).
}

// AggregateResult holds the combined outcome of all hooks for an event.
type AggregateResult struct {
	Decision     Decision
	Halt         bool       // Any hook requested halt.
	HookCount    int        // Number of hooks that ran.
	Hooks        []HookInfo // Info about each hook that ran (config order).
	Reason       string     // Concatenated deny/halt reasons (newline-separated).
	Context      string     // Concatenated context from all hooks.
	UpdatedInput string     // Merged tool_input JSON (empty if no patches).
}

// aggregate merges multiple HookResults into a single AggregateResult.
// Results are processed in config order (the order of the slice). Deny
// wins over allow, allow wins over none. Halt is sticky. Reasons and
// context concatenate in order.
//
// When replace is true, updated_input uses last-writer-wins replacement
// semantics (for PostToolUse output replacement). When false,
// updated_input patches shallow-merge against origToolInput (for
// PreToolUse input rewriting).
func aggregate(results []HookResult, origToolInput string, replace bool) AggregateResult {
	var (
		decision Decision
		halt     bool
		reasons  []string
		contexts []string
		merged   = origToolInput
		anyPatch = false
	)
	for _, r := range results {
		switch r.Decision {
		case DecisionDeny:
			decision = DecisionDeny
			if r.Reason != "" {
				reasons = append(reasons, r.Reason)
			}
		case DecisionAllow:
			if decision != DecisionDeny {
				decision = DecisionAllow
			}
		case DecisionNone:
			// No change.
		}
		if r.Halt {
			halt = true
			if r.Reason != "" && r.Decision != DecisionDeny {
				// A halting hook that didn't also deny still contributes
				// its reason so the user sees it.
				reasons = append(reasons, r.Reason)
			}
		}
		if r.Context != "" {
			contexts = append(contexts, r.Context)
		}
		if r.UpdatedInput != "" {
			if replace {
				// Last-writer-wins: no merge against base.
				merged = r.UpdatedInput
			} else {
				next, err := shallowMerge(merged, r.UpdatedInput)
				if err != nil {
					slog.Warn(
						"Hook updated_input patch rejected; ignoring",
						"error", err,
						"patch", r.UpdatedInput,
					)
					continue
				}
				merged = next
			}
			anyPatch = true
		}
	}

	agg := AggregateResult{
		Decision:  decision,
		Halt:      halt,
		HookCount: len(results),
	}
	if anyPatch {
		agg.UpdatedInput = merged
	}
	if len(reasons) > 0 {
		agg.Reason = strings.Join(reasons, "\n")
	}
	if len(contexts) > 0 {
		agg.Context = strings.Join(contexts, "\n")
	}
	return agg
}

// shallowMerge applies a top-level-keys patch to base (both JSON
// objects). Keys in patch overwrite keys in base; keys absent from the
// patch are preserved. Returns an error if either value is not a valid
// JSON object.
func shallowMerge(base, patch string) (string, error) {
	if base == "" {
		base = "{}"
	}
	// Ensure base is an object so sjson has somewhere to write.
	var baseAny any
	if err := json.Unmarshal([]byte(base), &baseAny); err != nil {
		return "", err
	}
	if _, ok := baseAny.(map[string]any); !ok {
		return "", errNotObject("tool_input")
	}
	var patchMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(patch), &patchMap); err != nil {
		return "", errNotObject("updated_input")
	}
	out := base
	for k, v := range patchMap {
		next, err := sjson.SetRawBytes([]byte(out), k, v)
		if err != nil {
			return "", err
		}
		out = string(next)
	}
	return out, nil
}

type errNotObject string

func (e errNotObject) Error() string { return string(e) + " is not a JSON object" }
