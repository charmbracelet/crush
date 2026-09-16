package agent

import (
	"os"
	"strconv"

	"charm.land/fantasy"
)

// SessionTelemetry is the per-session counter snapshot the eval
// harness records into run records — the stub-track and
// notebook-recall splits that a flat count can't express.
type SessionTelemetry struct {
	StubInvalidations int   `json:"invalidations"`
	StubResults       int   `json:"results"`
	StubSavedBytes    int64 `json:"saved_bytes"`
	BoundaryAdvances  int   `json:"boundary_advances"`
	// StubKinds splits StubResults by stub kind, keyed by the kind's
	// telemetry label — the empty-string superseded kind surfaces as
	// "superseded", never "".
	StubKinds     map[string]int `json:"kinds,omitempty"`
	ResultRecalls int            `json:"result_recalls"`
	EntryRecalls  int            `json:"entry_recalls"`
	EmptyRecalls  int            `json:"empty_recalls"`
	CrossRecalls  int            `json:"cross_recalls"`
	// Checkpoint telemetry: written counts committed checkpoint
	// entries; rendered counts prefix renders that included one.
	CheckpointsWritten int `json:"checkpoints_written"`
	CheckpointRenders  int `json:"checkpoint_renders"`
}

// SessionTelemetry returns the coordinator's per-session counters.
// Deliberately not on the Coordinator interface — the eval harness
// type-asserts for it so test stubs needn't implement it. Zero value
// when the agent is absent or the session has no accumulated counters.
func (c *coordinator) SessionTelemetry(sessionID string) SessionTelemetry {
	sa, ok := c.currentAgent.(*sessionAgent)
	if !ok || sa == nil {
		return SessionTelemetry{}
	}
	var t SessionTelemetry
	if s, ok := sa.stubStats.Get(sessionID); ok {
		t.StubInvalidations = s.Invalidations
		t.StubResults = s.Results
		t.StubSavedBytes = s.SavedBytes
		t.BoundaryAdvances = s.BoundaryAdvances
		if len(s.Kinds) > 0 {
			t.StubKinds = make(map[string]int, len(s.Kinds))
			for kind, n := range s.Kinds {
				t.StubKinds[kind.String()] += n
			}
		}
	}
	if n, ok := sa.nbStats.Get(sessionID); ok {
		t.ResultRecalls = n.ResultRecalls
		t.EntryRecalls = n.EntryRecalls
		t.EmptyRecalls = n.EmptyRecalls
		t.CrossRecalls = n.CrossRecalls
		t.CheckpointsWritten = n.CheckpointsWritten
		t.CheckpointRenders = n.CheckpointRenders
	}
	return t
}

// EvalMaxStepsEnvVar caps a single run's steps when the eval harness
// is driving — the run-side enforcement of the trajectory-wide
// max_steps budget. The driver passes the remaining budget (plus one)
// per turn; kept in sync with internal/eval.EvalMaxStepsEnvVar.
const EvalMaxStepsEnvVar = "CRUSH_EVAL_MAX_STEPS"

// evalStepCaps returns the eval step cap as a StopCondition, or nil
// when the harness isn't driving this process.
func evalStepCaps() []fantasy.StopCondition {
	n, err := strconv.Atoi(os.Getenv(EvalMaxStepsEnvVar))
	if err != nil || n <= 0 {
		return nil
	}
	return []fantasy.StopCondition{fantasy.StepCountIs(n)}
}
