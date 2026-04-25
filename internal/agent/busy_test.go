package agent

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// TestIsSessionBusyAcrossKeys verifies that a running summarize counts as
// busy for the same session (preventing concurrent generations) without
// leaking into other sessions.
func TestIsSessionBusyAcrossKeys(t *testing.T) {
	t.Parallel()

	a := &sessionAgent{activeRequests: csync.NewMap[string, context.CancelFunc]()}
	noop := context.CancelFunc(func() {})

	if a.IsSessionBusy("s1") {
		t.Fatalf("empty agent must not be busy")
	}

	a.activeRequests.Set("s1", noop)
	if !a.IsSessionBusy("s1") {
		t.Fatalf("busy when regular request key is set")
	}
	a.activeRequests.Del("s1")

	a.activeRequests.Set(summarizeRequestKey("s1"), noop)
	if !a.IsSessionBusy("s1") {
		t.Fatalf("busy when summarize key is set — prevents concurrent generations during summary")
	}
	if a.IsSessionBusy("s2") {
		t.Fatalf("summarize key for s1 must not leak into s2 busy check")
	}
}

// TestSummarizeRequestKey guards the key format because Cancel and
// CancelAll trim the suffix to recover the session ID.
func TestSummarizeRequestKey(t *testing.T) {
	t.Parallel()

	got := summarizeRequestKey("abc")
	want := "abc" + summarizeRequestKeySuffix
	if got != want {
		t.Fatalf("summarizeRequestKey(abc) = %q, want %q", got, want)
	}
}

// TestCancelCancelsBothRunAndSummarize verifies Cancel triggers cancel funcs
// for both the Run and Summarize slots on the same session — a regression
// guard for the previous behavior where the summarize cancel func was only
// reachable through the literal "sessionID-summarize" key.
func TestCancelCancelsBothRunAndSummarize(t *testing.T) {
	t.Parallel()

	a := &sessionAgent{
		activeRequests: csync.NewMap[string, context.CancelFunc](),
		messageQueue:   csync.NewMap[string, []SessionAgentCall](),
	}

	var runCalled, summarizeCalled bool
	a.activeRequests.Set("sess", func() { runCalled = true })
	a.activeRequests.Set(summarizeRequestKey("sess"), func() { summarizeCalled = true })

	a.Cancel("sess")

	if !runCalled {
		t.Errorf("Run cancel func was not invoked by Cancel")
	}
	if !summarizeCalled {
		t.Errorf("Summarize cancel func was not invoked by Cancel")
	}
}

// TestCancelAllTranslatesSummarizeKeyToSessionID verifies CancelAll does not
// pass the suffixed summarize key as a session ID to Cancel, which would
// miss the run cancel func. We set only a summarize key; CancelAll must
// still invoke it.
func TestCancelAllTranslatesSummarizeKeyToSessionID(t *testing.T) {
	t.Parallel()

	a := &sessionAgent{
		activeRequests: csync.NewMap[string, context.CancelFunc](),
		messageQueue:   csync.NewMap[string, []SessionAgentCall](),
	}

	var summarizeCalled bool
	a.activeRequests.Set(summarizeRequestKey("sess"), func() {
		summarizeCalled = true
		a.activeRequests.Del(summarizeRequestKey("sess"))
	})

	a.CancelAll()

	if !summarizeCalled {
		t.Errorf("Summarize cancel func must be invoked by CancelAll even when only the summarize key is set")
	}
}

// TestPersistTerminalFinishWritesFallback verifies the helper: when an
// assistant message has no terminal finish part, persistTerminalFinish must
// append one and flush it so the UI spinner stops. Covers Run's success
// fallback and Summarize's success path.
func TestPersistTerminalFinishWritesFallback(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	msg, err := env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:     message.Assistant,
		Model:    "test",
		Provider: "test",
	})
	require.NoError(t, err)
	require.False(t, msg.IsFinished(), "precondition: message must start unfinished")

	a := &sessionAgent{messages: env.messages}
	a.persistTerminalFinish(sess.ID, &msg, message.FinishReasonEndTurn, "")

	// The in-memory msg now has the finish part, and the DB must reflect it.
	require.True(t, msg.IsFinished(), "in-memory message should be finished after fallback")

	persisted, err := env.messages.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	require.True(t, persisted.IsFinished(), "persisted message must have a finish part")
	require.Equal(t, message.FinishReasonEndTurn, persisted.FinishPart().Reason)
}

// TestPersistTerminalFinishWritesErrorReason verifies that passing an error
// title surfaces through the finish part. Covers Summarize's non-cancel
// error path.
func TestPersistTerminalFinishWritesErrorReason(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	msg, err := env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:     message.Assistant,
		Model:    "test",
		Provider: "test",
	})
	require.NoError(t, err)

	a := &sessionAgent{messages: env.messages}
	a.persistTerminalFinish(sess.ID, &msg, message.FinishReasonError, "boom")

	persisted, err := env.messages.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	finish := persisted.FinishPart()
	require.NotNil(t, finish)
	require.Equal(t, message.FinishReasonError, finish.Reason)
	require.Equal(t, "boom", finish.Message)
}

// TestPersistTerminalFinishNoOpWhenAlreadyFinished verifies idempotency:
// if a finish part is already present it must not be duplicated or
// overwritten. This is what lets callers invoke the helper on paths where
// fantasy's OnStepFinish may or may not have already written a finish.
func TestPersistTerminalFinishNoOpWhenAlreadyFinished(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	msg, err := env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:     message.Assistant,
		Model:    "test",
		Provider: "test",
	})
	require.NoError(t, err)
	msg.AddFinish(message.FinishReasonError, "boom", "details")
	require.NoError(t, env.messages.Update(t.Context(), msg))

	a := &sessionAgent{messages: env.messages}
	// Even with a conflicting EndTurn request the existing Error finish
	// must stand.
	a.persistTerminalFinish(sess.ID, &msg, message.FinishReasonEndTurn, "")

	persisted, err := env.messages.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	finish := persisted.FinishPart()
	require.NotNil(t, finish)
	require.Equal(t, message.FinishReasonError, finish.Reason,
		"existing finish reason must not be overwritten")
	require.Equal(t, "boom", finish.Message,
		"existing finish message must not be overwritten")
}

// TestPersistTerminalFinishNilIsSafe documents that the helper tolerates a
// nil message pointer — Run may bail out before creating the assistant
// message.
func TestPersistTerminalFinishNilIsSafe(t *testing.T) {
	t.Parallel()

	a := &sessionAgent{}
	a.persistTerminalFinish("sess", nil, message.FinishReasonEndTurn, "") // must not panic
}
