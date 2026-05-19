package critic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGate_NilFeedback(t *testing.T) {
	t.Parallel()
	require.Equal(t, GateRevise, Gate(nil))
}

func TestGate_Approve(t *testing.T) {
	t.Parallel()
	fb := &CriticFeedback{Verdict: "approve", Confidence: 0.95}
	require.Equal(t, GateApprove, Gate(fb))
}

func TestGate_Revise(t *testing.T) {
	t.Parallel()
	fb := &CriticFeedback{Verdict: "revise", Confidence: 0.72}
	require.Equal(t, GateRevise, Gate(fb))
}

func TestGate_Halt(t *testing.T) {
	t.Parallel()
	fb := &CriticFeedback{Verdict: "halt", Confidence: 1.0}
	require.Equal(t, GateHalt, Gate(fb))
}

func TestGate_UnknownVerdict(t *testing.T) {
	t.Parallel()
	fb := &CriticFeedback{Verdict: "something_weird", Confidence: 0.5}
	require.Equal(t, GateApprove, Gate(fb))
}

func TestCheckpointType_Constants(t *testing.T) {
	t.Parallel()
	require.Equal(t, CheckpointType("edit"), CheckpointEdit)
}
