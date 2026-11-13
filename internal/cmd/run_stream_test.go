package cmd

import (
	"bytes"
	"errors"
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

	s := &runStream{sessionID: "S", out: &bytes.Buffer{}, seen: map[string]string{}}

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
	s := &runStream{sessionID: "S", out: buf, seen: map[string]string{}}

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
	s := &runStream{sessionID: "S", out: buf, seen: map[string]string{}}

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
	s := &runStream{sessionID: "S", out: buf, seen: map[string]string{}}

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

	s := &runStream{sessionID: "S", out: &bytes.Buffer{}, seen: map[string]string{}}
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

	s := &runStream{sessionID: "S", out: &bytes.Buffer{}, seen: map[string]string{}}
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

	s := &runStream{sessionID: "S", out: &bytes.Buffer{}, seen: map[string]string{}}
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
	s := &runStream{sessionID: "S", out: buf, seen: map[string]string{}}

	done, err := s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: "S",
		MessageID: "m2",
		Text:      "  \tactual output",
	}}, nil)
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, "actual output", buf.String())
}

// TestRunStream_LeadingBlankLines checks that leading blank lines and
// spaces are dropped until something is printed. With a RunID only the
// final text in RunComplete is printed. Without one the reply is printed
// in pieces as its events arrive.
func TestRunStream_LeadingBlankLines(t *testing.T) {
	t.Parallel()
	for name, texts := range map[string][]string{
		"whole":               {"\nHello"},
		"pieces":              {"\n", "\nHello"},
		"more blank lines":    {"\n", "\n\nHello"},
		"spaces":              {" ", "  Hello"},
		"blank then indented": {"\n", "\n  Hello"},
	} {
		for _, runID := range []string{"", "run-mine"} {
			t.Run(name+"/runID="+runID, func(t *testing.T) {
				t.Parallel()
				buf := &bytes.Buffer{}
				s := &runStream{sessionID: "S", runID: runID, out: buf, seen: map[string]string{}}
				for _, text := range texts {
					done, err := s.handle(replyEvent("m", text), nil)
					require.NoError(t, err)
					require.False(t, done)
				}
				done, err := s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
					SessionID: "S", RunID: runID, MessageID: "m", Text: texts[len(texts)-1],
				}}, nil)
				require.NoError(t, err)
				require.True(t, done)
				require.Equal(t, "Hello", buf.String())
			})
		}
	}
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
	s := &runStream{sessionID: "S", out: &bytes.Buffer{}, seen: map[string]string{}}
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
		seen:      map[string]string{},
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
		seen:      map[string]string{},
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

// TestRunStream_AgentErrorRunIDFiltersForeign verifies that an async
// agent error carrying a non-empty RunID is fatal only when it matches
// our run. A foreign RunID is ignored regardless of the event's
// SessionID, because RunID is the authoritative correlator and async
// errors share the agent event channel: without strict RunID matching
// an unrelated workspace failure would abort our run.
func TestRunStream_AgentErrorRunIDFiltersForeign(t *testing.T) {
	t.Parallel()

	// Foreign RunID with a matching session is still foreign.
	s := &runStream{sessionID: "S", runID: "run-mine", out: &bytes.Buffer{}, seen: map[string]string{}}
	done, err := s.handle(pubsub.Event[proto.AgentEvent]{Payload: proto.AgentEvent{
		Type:      proto.AgentEventTypeError,
		SessionID: "S",
		RunID:     "run-other",
		Error:     errors.New("foreign boom"),
	}}, nil)
	require.NoError(t, err, "foreign RunID error must not abort our run")
	require.False(t, done)

	// Foreign RunID with a different session is ignored.
	done, err = s.handle(pubsub.Event[proto.AgentEvent]{Payload: proto.AgentEvent{
		Type:      proto.AgentEventTypeError,
		SessionID: "other",
		RunID:     "run-other",
		Error:     errors.New("foreign boom"),
	}}, nil)
	require.NoError(t, err, "foreign RunID error must not abort our run")
	require.False(t, done)

	// Foreign RunID with a missing session is ignored.
	done, err = s.handle(pubsub.Event[proto.AgentEvent]{Payload: proto.AgentEvent{
		Type:  proto.AgentEventTypeError,
		RunID: "run-other",
		Error: errors.New("foreign boom"),
	}}, nil)
	require.NoError(t, err, "foreign RunID error must not abort our run")
	require.False(t, done)

	// Matching RunID is fatal.
	done, err = s.handle(pubsub.Event[proto.AgentEvent]{Payload: proto.AgentEvent{
		Type:      proto.AgentEventTypeError,
		SessionID: "S",
		RunID:     "run-mine",
		Error:     errors.New("my boom"),
	}}, nil)
	require.Error(t, err, "matching RunID error must be fatal")
	require.True(t, done)
}

// TestRunStream_AgentErrorNoRunIDFiltersBySession verifies the
// compatibility fallback: when the event carries no RunID, attribution
// falls back to SessionID. An error for another session or with an
// empty session is ignored, while an error for our own session is fatal
// so a real failure is never dropped.
func TestRunStream_AgentErrorNoRunIDFiltersBySession(t *testing.T) {
	t.Parallel()

	s := &runStream{sessionID: "S", out: &bytes.Buffer{}, seen: map[string]string{}}

	// Empty RunID for another session is ignored.
	done, err := s.handle(pubsub.Event[proto.AgentEvent]{Payload: proto.AgentEvent{
		Type:      proto.AgentEventTypeError,
		SessionID: "other",
		Error:     errors.New("foreign boom"),
	}}, nil)
	require.NoError(t, err, "error for another session must not abort our run")
	require.False(t, done)

	// Empty RunID with an empty session is ignored.
	done, err = s.handle(pubsub.Event[proto.AgentEvent]{Payload: proto.AgentEvent{
		Type:  proto.AgentEventTypeError,
		Error: errors.New("foreign boom"),
	}}, nil)
	require.NoError(t, err, "error with no session must not abort our run")
	require.False(t, done)

	// Empty RunID with a matching session is fatal.
	done, err = s.handle(pubsub.Event[proto.AgentEvent]{Payload: proto.AgentEvent{
		Type:      proto.AgentEventTypeError,
		SessionID: "S",
		Error:     errors.New("my boom"),
	}}, nil)
	require.Error(t, err, "error for our own session must be fatal")
	require.True(t, done)
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
	s := &runStream{sessionID: "S", out: buf, seen: map[string]string{}}
	done, err := s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: "S",
		MessageID: "m2",
		Text:      "DONE",
	}}, nil)
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, "DONE", buf.String())
}

// replyEvent returns a message event for an assistant reply with the
// given text in session "S".
func replyEvent(id, text string) pubsub.Event[proto.Message] {
	return pubsub.Event[proto.Message]{Payload: proto.Message{
		ID: id, SessionID: "S", Role: proto.Assistant,
		Parts: []proto.ContentPart{proto.TextContent{Text: text}},
	}}
}

// TestRunStream_RunIDRetryPrintsFinalText pins what `crush run` prints
// through a server when a provider request is retried after part of the
// reply has streamed. runNonInteractive always supplies a RunID, so
// message events are not printed and stdout comes from RunComplete
// alone: the retried text, even when it is shorter than what the first
// attempt streamed.
func TestRunStream_RunIDRetryPrintsFinalText(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &runStream{sessionID: "S", runID: "run-mine", out: buf, seen: map[string]string{}}
	for _, ev := range []pubsub.Event[proto.Message]{
		replyEvent("m", "A first attempt that"),
		// The reset update of a retry has no parts.
		{Payload: proto.Message{ID: "m", SessionID: "S", Role: proto.Assistant}},
		replyEvent("m", "Real."),
	} {
		done, err := s.handle(ev, nil)
		require.NoError(t, err)
		require.False(t, done)
	}
	require.Empty(t, buf.String())

	done, err := s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: "S", RunID: "run-mine", MessageID: "m", Text: "Real.",
	}}, nil)
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, "Real.", buf.String())
}

// TestRunStream_NoRunIDRetryReplacesText covers the fallback without a
// RunID, where message events are printed as they arrive. A provider
// retry resets the reply after part of it has streamed. The run must not
// fail, and the new text is printed in full without its leading spaces,
// whether it is shorter or longer than the first attempt. It starts on a
// new line unless the first attempt was blank.
func TestRunStream_NoRunIDRetryReplacesText(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		texts []string
		want  string
	}{
		"shorter": {
			texts: []string{"A first attempt that", "Real."},
			want:  "A first attempt that\nReal.",
		},
		"longer": {
			texts: []string{"A first attempt that", "The real answer, which is longer than the first attempt."},
			want:  "A first attempt that\nThe real answer, which is longer than the first attempt.",
		},
		"indented": {
			texts: []string{"First", "  Second"},
			want:  "First\nSecond",
		},
		"blank first attempt": {
			texts: []string{"\n", "Real."},
			want:  "Real.",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			s := &runStream{sessionID: "S", out: buf, seen: map[string]string{}}
			for _, text := range tc.texts {
				done, err := s.handle(replyEvent("m", text), nil)
				require.NoError(t, err)
				require.False(t, done)
			}
			final := tc.texts[len(tc.texts)-1]
			done, err := s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
				SessionID: "S", MessageID: "m", Text: final,
			}}, nil)
			require.NoError(t, err)
			require.True(t, done)
			require.Equal(t, tc.want, buf.String())
		})
	}
}

// TestRunStream_NoRunIDReconcilesAfterMissedRetry covers a retry whose
// message events never reach the stream. RunComplete then carries text
// that does not continue what was printed, so it is printed in full on
// a new line instead of being dropped or cut at the old length.
func TestRunStream_NoRunIDReconcilesAfterMissedRetry(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &runStream{sessionID: "S", out: buf, seen: map[string]string{}}
	_, err := s.handle(replyEvent("m", "A first attempt that"), nil)
	require.NoError(t, err)
	done, err := s.handle(pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: "S", MessageID: "m", Text: "Real.",
	}}, nil)
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, "A first attempt that\nReal.", buf.String())
}

// TestRunStream_NoRunIDGrowingAndStaleUpdates covers text that grows,
// which continues the output, and an update older than one already
// printed, which adds nothing. A retry that starts with the text of the
// failed attempt looks the same as growing text.
func TestRunStream_NoRunIDGrowingAndStaleUpdates(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &runStream{sessionID: "S", out: buf, seen: map[string]string{}}
	for _, text := range []string{"The ans", "The answer", "The ans", "The answer is 42."} {
		done, err := s.handle(replyEvent("m", text), nil)
		require.NoError(t, err)
		require.False(t, done)
	}
	require.Equal(t, "The answer is 42.", buf.String())
}

// TestRunStream_NoRunIDSkipsSummaries covers the fallback without a
// RunID: a compaction summary is stored as an assistant message in the
// session, but it is not part of the answer, so it is not printed.
func TestRunStream_NoRunIDSkipsSummaries(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &runStream{sessionID: "S", out: buf, seen: map[string]string{}}
	summary := replyEvent("sum", "SUMMARY")
	summary.Payload.IsSummaryMessage = true
	for _, ev := range []pubsub.Event[proto.Message]{
		replyEvent("m1", "Before."), summary, replyEvent("m2", "After."),
	} {
		done, err := s.handle(ev, nil)
		require.NoError(t, err)
		require.False(t, done)
	}
	require.Equal(t, "Before.After.", buf.String())
}
