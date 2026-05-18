package agent

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/csync"
)

func TestIsSessionBusyAcrossKeys(t *testing.T) {
	t.Parallel()

	a := &sessionAgent{
		activeRequests: csync.NewMap[string, context.CancelFunc](),
	}
	noop := context.CancelFunc(func() {})

	if a.IsSessionBusy("s1") {
		t.Fatalf("expected empty agent to report not busy")
	}

	a.activeRequests.Set("s1", noop)
	if !a.IsSessionBusy("s1") {
		t.Fatalf("expected busy when regular request key is set")
	}
	a.activeRequests.Del("s1")

	a.activeRequests.Set("s1-summarize", noop)
	if !a.IsSessionBusy("s1") {
		t.Fatalf("expected busy when summarize key is set — prevents concurrent generations during summary")
	}

	if a.IsSessionBusy("s2") {
		t.Fatalf("summarize key for s1 must not leak into s2 busy check")
	}
}
