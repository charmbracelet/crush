package agent

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/csync"
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
