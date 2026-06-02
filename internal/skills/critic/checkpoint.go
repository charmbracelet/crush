package critic

import "context"

// CheckpointType identifies the kind of work being reviewed.
type CheckpointType string

const (
	CheckpointEdit    CheckpointType = "edit"
	CheckpointMessage CheckpointType = "message"
)

// GateDecision represents the action to take after critic review.
type GateDecision int

const (
	GateApprove GateDecision = iota
	GateRevise
	GateHalt
)

// CriticFeedback is the structured response from the critic agent.
type CriticFeedback struct {
	Verdict    string          `json:"verdict"`
	Confidence float64         `json:"confidence"`
	Concerns   []CriticConcern `json:"concerns"`
	Summary    string          `json:"summary"`
}

// CriticConcern is a single issue raised by the critic.
type CriticConcern struct {
	Severity   string `json:"severity"`
	Dimension  string `json:"dimension"`
	Summary    string `json:"summary"`
	Suggestion string `json:"suggestion"`
}

// ToolCallSnapshot captures a tool invocation for review.
type ToolCallSnapshot struct {
	Name      string `json:"name"`
	Input     string `json:"input"`
	Output    string `json:"output"`
	ErrString string `json:"err_string,omitempty"`
}

// Checkpoint represents a slice of primary-agent work submitted for critique.
type Checkpoint struct {
	Type           CheckpointType
	UserPrompt     string
	PrimaryPlan    string
	PrimaryDiff    string
	MessageContent string
	ToolCalls      []ToolCallSnapshot
	LSPDiagnostics []DiagnosticSnapshot
	Iteration      int
	CoachSummary   string
}

// CoachSummaryProvider is implemented by middleware that can supply a coaching
// summary for the current session turn. The critic uses this to enrich its
// review with real-time tool usage observations.
type CoachSummaryProvider interface {
	GetCoachSummary(sessionID string) string
}

// Gate evaluates feedback and decides whether to proceed, revise, or halt.
func Gate(feedback *CriticFeedback) GateDecision {
	if feedback == nil {
		return GateRevise
	}

	switch feedback.Verdict {
	case "halt":
		return GateHalt
	case "revise":
		return GateRevise
	default:
		return GateApprove
	}
}

// CheckpointEmitter is the callback type used by the middleware to request a
// review. It is defined here so that the middleware and service share the same
// signature.
type CheckpointEmitter func(ctx context.Context, cp Checkpoint) (*CriticFeedback, error)
