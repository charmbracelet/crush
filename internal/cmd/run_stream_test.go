package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// TestRunStream_ToolUseDoesNotTerminate is the regression test for
// the original bug: a tool-call assistant message has a Finish part
// with reason=tool_use and used to terminate `crush run` early via
// the discarded `msg.IsFinished()` exit condition. With the new
// RunComplete-driven loop, tool_use finishes must keep the stream
// alive so the post-tool final text still reaches stdout.
func TestRunStream_ToolUseDoesNotTerminate(t *testing.T) {
	t.Parallel()

	s := &runStream{sessionID: "S", out: &bytes.Buffer{}, read: map[string]int{}}

	toolUse := proto.Message{
		ID:        "m1",
		SessionID: "S",
		Role:      proto.Assistant,
		Parts: []proto.ContentPart{
			proto.TextContent{Text: ""},
			proto.Finish{Reason: proto.FinishReasonToolUse, Time: time.Now().Unix()},
		},
	}
	done, err := s.handle(pubsub.Event[proto.Message]{Payload: toolUse}, nil)
	require.NoError(t, err)
	require.False(t, done, "tool_use finish must NOT terminate the run loop")
}

// TestRunStream_RunCompleteExits verifies the happy path: streaming
// assistant text then RunComplete terminates with the full final
// text on stdout. Together with the tool_use test above this
// nails down the "tool use + final text" sequence that the original
// bug truncated.
func TestRunStream_RunCompleteExits(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &runStream{sessionID: "S", out: buf, read: map[string]int{}}

	// Tool-use step.
	done, err := s.handle(pubsub.Event[proto.Message]{Payload: proto.Message{
		ID: "m1", SessionID: "S", Role: proto.Assistant,
		Parts: []proto.ContentPart{
			proto.TextContent{Text: ""},
			proto.Finish{Reason: proto.FinishReasonToolUse},
		},
	}}, nil)
	require.NoError(t, err)
	require.False(t, done)

	// Final assistant message stream.
	done, err = s.handle(pubsub.Event[proto.Message]{Payload: proto.Message{
		ID: "m2", SessionID: "S", Role: proto.Assistant,
		Parts: []proto.ContentPart{
			proto.TextContent{Text: "VERDICT: APPROVED"},
			proto.Finish{Reason: proto.FinishReasonEndTurn},
		},
	}}, nil)
	require.NoError(t, err)
	require.False(t, done, "message finish (even end_turn) must not exit; RunComplete is the only terminal signal")

	// RunComplete.
	done, err = s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: "S",
		MessageID: "m2",
		Text:      "VERDICT: APPROVED",
	}}, nil)
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, "VERDICT: APPROVED", buf.String())
}

// TestRunStream_ReconcilesOnOutOfOrderRunComplete is the worst-case
// ordering scenario: RunComplete reaches the client BEFORE any of
// the streaming assistant message events for the turn (the pubsub
// fan-in across upstream brokers does not preserve cross-broker
// ordering). The embedded Text field must rescue stdout so the
// caller still sees the complete final text.
func TestRunStream_ReconcilesOnOutOfOrderRunComplete(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &runStream{sessionID: "S", out: buf, read: map[string]int{}}

	done, err := s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: "S",
		MessageID: "m2",
		Text:      "VERDICT: APPROVED",
	}}, nil)
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, "VERDICT: APPROVED", buf.String(),
		"RunComplete must reconcile stdout when message events did not arrive in time")
}

// TestRunStream_ReconcilesPartialStream covers the realistic case
// where some streaming output reached stdout before RunComplete
// arrived: the reconciliation pass must append only the unread tail,
// never duplicate the prefix.
func TestRunStream_ReconcilesPartialStream(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &runStream{sessionID: "S", out: buf, read: map[string]int{}}

	_, err := s.handle(pubsub.Event[proto.Message]{Payload: proto.Message{
		ID: "m2", SessionID: "S", Role: proto.Assistant,
		Parts: []proto.ContentPart{proto.TextContent{Text: "VERDICT: "}},
	}}, nil)
	require.NoError(t, err)

	_, err = s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: "S",
		MessageID: "m2",
		Text:      "VERDICT: APPROVED",
	}}, nil)
	require.NoError(t, err)
	require.Equal(t, "VERDICT: APPROVED", buf.String())
}

// TestRunStream_IgnoresOtherSessions ensures multi-session
// subscribers (e.g. a TUI watching workspace events while `crush
// run` is in flight against the same workspace) do not cause
// premature exit on RunComplete for a different session.
func TestRunStream_IgnoresOtherSessions(t *testing.T) {
	t.Parallel()

	s := &runStream{sessionID: "S", out: &bytes.Buffer{}, read: map[string]int{}}
	done, err := s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: "OTHER",
		MessageID: "x",
		Text:      "noise",
	}}, nil)
	require.NoError(t, err)
	require.False(t, done)
}

// TestRunStream_ErrorRunComplete surfaces a failing run as a
// non-nil error from `crush run` so shells and CI catch it via
// exit status.
func TestRunStream_ErrorRunComplete(t *testing.T) {
	t.Parallel()

	s := &runStream{sessionID: "S", out: &bytes.Buffer{}, read: map[string]int{}}
	done, err := s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: "S",
		Error:     "model temporarily unavailable",
	}}, nil)
	require.True(t, done)
	require.Error(t, err)
	require.Contains(t, err.Error(), "model temporarily unavailable")
}

// TestRunStream_CancelledRunCompleteIsClean ensures a cancelled
// run (e.g. Ctrl+C while `crush run` waits) exits cleanly rather
// than reporting the cancellation as a failure.
func TestRunStream_CancelledRunCompleteIsClean(t *testing.T) {
	t.Parallel()

	s := &runStream{sessionID: "S", out: &bytes.Buffer{}, read: map[string]int{}}
	done, err := s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: "S",
		Error:     "context canceled",
		Cancelled: true,
	}}, nil)
	require.True(t, done)
	require.NoError(t, err)
}

// TestRunStream_LeadingWhitespaceTrimmedOnce mirrors the
// pre-existing trim of leading whitespace on the first byte of
// stdout: the trim must happen exactly once even when stdout is
// first produced by the RunComplete reconciliation path rather
// than the live stream.
func TestRunStream_LeadingWhitespaceTrimmedOnce(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &runStream{sessionID: "S", out: buf, read: map[string]int{}}

	done, err := s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: "S",
		MessageID: "m2",
		Text:      "  \tactual output",
	}}, nil)
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, "actual output", buf.String())
}

// TestRunStream_StopSpinnerInvokedOnFirstOutput verifies the
// spinner is stopped exactly when meaningful output starts (either
// a streamed assistant message or the reconciliation tail). This
// matches the prior behaviour and prevents the spinner from
// painting over the final response on TTYs.
func TestRunStream_StopSpinnerInvokedOnFirstOutput(t *testing.T) {
	t.Parallel()

	calls := 0
	stop := func() { calls++ }
	s := &runStream{sessionID: "S", out: &bytes.Buffer{}, read: map[string]int{}}
	_, _ = s.handle(pubsub.Event[proto.Message]{Payload: proto.Message{
		ID: "m1", SessionID: "S", Role: proto.Assistant,
		Parts: []proto.ContentPart{proto.TextContent{Text: "hi"}},
	}}, stop)
	require.GreaterOrEqual(t, calls, 1, "spinner must stop once stdout has content")
}

// TestRunStream_RunIDFiltersForeignTurns covers the busy-session
// queue scenario: `crush run --continue` attaches to a session
// whose currently running turn finishes first, publishing its
// RunComplete on the same session ID. Without per-run correlation
// the stream would exit on that foreign event and drop our own
// queued turn's output. With RunID filtering the foreign event is
// ignored and only the matching RunComplete terminates the stream.
func TestRunStream_RunIDFiltersForeignTurns(t *testing.T) {
	t.Parallel()

	const sessionID = "S"
	const myRun = "run-mine"
	const otherRun = "run-other"

	buf := &bytes.Buffer{}
	s := &runStream{
		sessionID: sessionID,
		runID:     myRun,
		out:       buf,
		read:      map[string]int{},
	}

	// The busy session's existing turn emits more text before it finishes.
	done, err := s.handle(pubsub.Event[proto.Message]{Payload: proto.Message{
		ID:        "other-msg",
		SessionID: sessionID,
		Role:      proto.Assistant,
		Parts:     []proto.ContentPart{proto.TextContent{Text: "noise from another turn"}},
	}}, nil)
	require.NoError(t, err)
	require.False(t, done,
		"foreign message on same session must not terminate our run")
	require.Empty(t, buf.String(),
		"foreign message on same session must not write to our stdout")

	// The busy session's existing turn finishes first.
	done, err = s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: sessionID,
		RunID:     otherRun,
		MessageID: "other-msg",
		Text:      "noise from another turn",
	}}, nil)
	require.NoError(t, err)
	require.False(t, done,
		"foreign RunComplete on same session must not terminate our run")
	require.Empty(t, buf.String(),
		"foreign RunComplete must not write to our stdout")

	// Our own queued turn eventually finishes.
	done, err = s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: sessionID,
		RunID:     myRun,
		MessageID: "my-msg",
		Text:      "OK",
	}}, nil)
	require.NoError(t, err)
	require.True(t, done, "matching RunID must terminate the stream")
	require.Equal(t, "OK", buf.String())
}

func TestRunStream_RunIDSuppressesLiveMessagesAndPrintsRunComplete(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &runStream{
		sessionID: "S",
		runID:     "run-mine",
		out:       buf,
		read:      map[string]int{},
	}

	done, err := s.handle(pubsub.Event[proto.Message]{Payload: proto.Message{
		ID:        "my-msg",
		SessionID: "S",
		Role:      proto.Assistant,
		Parts:     []proto.ContentPart{proto.TextContent{Text: "streamed prefix"}},
	}}, nil)
	require.NoError(t, err)
	require.False(t, done)
	require.Empty(t, buf.String())

	done, err = s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: "S",
		RunID:     "run-mine",
		MessageID: "my-msg",
		Text:      "streamed prefix final",
	}}, nil)
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, "streamed prefix final", buf.String())
}

// TestRunStream_NoRunIDFallsBackToSessionID preserves the older
// behaviour for callers (and tests) that don't supply a RunID:
// SessionID-only matching still terminates the stream on the
// session's RunComplete. This keeps the contract backwards
// compatible with servers that don't echo RunID and with the
// pre-existing TestRunStream_* assertions.
func TestRunStream_NoRunIDFallsBackToSessionID(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &runStream{sessionID: "S", out: buf, read: map[string]int{}}
	done, err := s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: "S",
		MessageID: "m2",
		Text:      "DONE",
	}}, nil)
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, "DONE", buf.String())
}
