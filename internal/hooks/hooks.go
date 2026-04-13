// Package hooks runs user-defined shell commands that fire on hook events
// (e.g. PreToolUse), returning decisions that control agent behavior.
package hooks

import "strings"

// Hook event name constants.
const (
	EventPreToolUse = "PreToolUse"
)

// HookMetadata is embedded in tool response metadata so the UI can
// display a hook indicator.
type HookMetadata struct {
	HookCount    int        `json:"hook_count"`
	Decision     string     `json:"decision"`
	Reason       string     `json:"reason,omitempty"`
	InputRewrite bool       `json:"input_rewrite,omitempty"`
	Hooks        []HookInfo `json:"hooks,omitempty"`
}

// HookInfo identifies a single hook that ran and its individual result.
type HookInfo struct {
	Name         string `json:"name"`
	Matcher      string `json:"matcher,omitempty"`
	Decision     string `json:"decision"`
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
	Reason       string
	Context      string
	UpdatedInput string // Replacement tool input JSON (opaque string).
}

// AggregateResult holds the combined outcome of all hooks for an event.
type AggregateResult struct {
	Decision     Decision
	HookCount    int        // Number of hooks that ran.
	Hooks        []HookInfo // Info about each hook that ran.
	Reason       string     // Concatenated deny reasons (newline-separated).
	Context      string     // Concatenated context from all hooks.
	UpdatedInput string     // Last non-empty updated_input wins.
}

// aggregate merges multiple HookResults into a single AggregateResult.
// Deny wins over allow, allow wins over none. All deny reasons and all
// context strings are concatenated.
func aggregate(results []HookResult) AggregateResult {
	var (
		decision     Decision
		reasons      []string
		contexts     []string
		updatedInput string
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
		if r.Context != "" {
			contexts = append(contexts, r.Context)
		}
		if r.UpdatedInput != "" {
			updatedInput = r.UpdatedInput
		}
	}

	agg := AggregateResult{Decision: decision, HookCount: len(results), UpdatedInput: updatedInput}
	if len(reasons) > 0 {
		agg.Reason = strings.Join(reasons, "\n")
	}
	if len(contexts) > 0 {
		agg.Context = strings.Join(contexts, "\n")
	}
	return agg
}
