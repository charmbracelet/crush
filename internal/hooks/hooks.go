// Package hooks runs user-defined shell commands that fire on hook events
// (e.g. PreToolUse), returning decisions that control agent behavior.
package hooks

// Hook event name constants.
const (
	EventPreToolUse = "PreToolUse"
)

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
	Decision Decision
	Reason   string
	Context  string
}

// AggregateResult holds the combined outcome of all hooks for an event.
type AggregateResult struct {
	Decision Decision
	Reason   string // Concatenated deny reasons (newline-separated).
	Context  string // Concatenated context from all hooks.
}

// aggregate merges multiple HookResults into a single AggregateResult.
// Deny wins over allow, allow wins over none. All deny reasons and all
// context strings are concatenated.
func aggregate(results []HookResult) AggregateResult {
	var (
		decision Decision
		reasons  []string
		contexts []string
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
	}

	agg := AggregateResult{Decision: decision}
	if len(reasons) > 0 {
		for i, r := range reasons {
			if i > 0 {
				agg.Reason += "\n"
			}
			agg.Reason += r
		}
	}
	if len(contexts) > 0 {
		for i, c := range contexts {
			if i > 0 {
				agg.Context += "\n"
			}
			agg.Context += c
		}
	}
	return agg
}
