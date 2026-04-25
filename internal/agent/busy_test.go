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

// TestEnsureAssistantFinishedWritesFallback verifies the defense-in-depth
// fallback: when Run's stream returns nil but the assistant message has no
// terminal finish part, ensureAssistantFinished must append one so the UI
// spinner stops.
func TestEnsureAssistantFinishedWritesFallback(t *testing.T) {
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
	a.ensureAssistantFinished(sess.ID, &msg)

	// The in-memory msg now has the finish part, and the DB must reflect it.
	require.True(t, msg.IsFinished(), "in-memory message should be finished after fallback")

	persisted, err := env.messages.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	require.True(t, persisted.IsFinished(), "persisted message must have a finish part")
	require.Equal(t, message.FinishReasonEndTurn, persisted.FinishPart().Reason)
}

// TestEnsureAssistantFinishedNoOpWhenAlreadyFinished verifies the fallback
// is idempotent: if a finish part is already present it must not be
// duplicated or overwritten.
func TestEnsureAssistantFinishedNoOpWhenAlreadyFinished(t *testing.T) {
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
	a.ensureAssistantFinished(sess.ID, &msg)

	persisted, err := env.messages.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	finish := persisted.FinishPart()
	require.NotNil(t, finish)
	require.Equal(t, message.FinishReasonError, finish.Reason,
		"existing finish reason must not be overwritten")
	require.Equal(t, "boom", finish.Message,
		"existing finish message must not be overwritten")
}

// TestEnsureAssistantFinishedNilIsSafe documents that the helper tolerates
// a nil assistant pointer — Run may bail out before creating the message.
func TestEnsureAssistantFinishedNilIsSafe(t *testing.T) {
	t.Parallel()

	a := &sessionAgent{}
	a.ensureAssistantFinished("sess", nil) // must not panic
}
