package agent

import (
	"context"
	"sync/atomic"
)

// runCompleteMarkerKey is the unexported context key carrying a
// [runCompleteMarker] from the dispatch boundary (backend.runAgent)
// down into the coordinator and the session agent. It lets the
// dispatcher learn what happened to the run it dispatched — whether the
// authoritative terminal notify.RunComplete was already published, and
// whether the run's prompt was requeued so that the outcome it is
// handed belongs to a different prompt — so it only publishes a
// fallback terminal event when one is actually missing for its own run
// (e.g. an error returned before sessionAgent.Run ever executed). It
// avoids a breaking change to the Coordinator interface.
type runCompleteMarkerKey struct{}

// runCompleteMarker records what became of a dispatched run. It is
// shared by pointer through the context so a publish (or a requeue)
// deep in the call stack is observable by the dispatcher after the call
// returns.
type runCompleteMarker struct {
	published atomic.Bool
	// requeuedTo is non-nil when sessionAgent.Run swapped the
	// dispatched prompt into the session's message queue and ran a
	// prompt already queued ahead of it instead (see
	// swapWithQueueHead). It holds that prompt's RunID, which may be
	// empty, so the pointer — not the string — is the signal.
	requeuedTo atomic.Pointer[string]
}

// WithRunCompleteMarker returns ctx carrying a fresh marker the
// coordinator can flag via [MarkRunCompletePublished] once it emits the
// run's terminal RunComplete, and the session agent can flag via
// [MarkRunRequeued] when it requeues the run's prompt. Callers read the
// results with [RunCompletePublished] and [RequeuedRun]. Attaching the
// marker is optional: code paths without one simply skip the signals.
func WithRunCompleteMarker(ctx context.Context) context.Context {
	return context.WithValue(ctx, runCompleteMarkerKey{}, &runCompleteMarker{})
}

// MarkRunCompletePublished records that the authoritative terminal
// RunComplete has been published for the run carried by ctx. It is a
// no-op when no marker is present (e.g. the in-process/local Run path,
// which is not dispatched through backend.runAgent).
func MarkRunCompletePublished(ctx context.Context) {
	if m, ok := ctx.Value(runCompleteMarkerKey{}).(*runCompleteMarker); ok {
		m.published.Store(true)
	}
}

// MarkRunRequeued records that the prompt dispatched with ctx was
// requeued behind its session's message queue and that this invocation
// ran the already-queued prompt identified by ranRunID in its place.
// The dispatcher reads it with [RequeuedRun] to keep attribution
// straight: the outcome it is handed belongs to ranRunID, while ctx's
// own run is still queued and owes its own terminal event when it runs.
// It is a no-op when no marker is present.
func MarkRunRequeued(ctx context.Context, ranRunID string) {
	if m, ok := ctx.Value(runCompleteMarkerKey{}).(*runCompleteMarker); ok {
		m.requeuedTo.Store(&ranRunID)
	}
}

// RequeuedRun reports whether [MarkRunRequeued] was called on ctx's
// marker and, if so, the RunID of the prompt the invocation ran instead
// of ctx's own (empty when that prompt carried none). It returns
// ("", false) when no marker is present.
func RequeuedRun(ctx context.Context) (string, bool) {
	if m, ok := ctx.Value(runCompleteMarkerKey{}).(*runCompleteMarker); ok {
		if ran := m.requeuedTo.Load(); ran != nil {
			return *ran, true
		}
	}
	return "", false
}

// RunCompletePublished reports whether [MarkRunCompletePublished] was
// called on ctx's marker. It returns false when no marker is present.
func RunCompletePublished(ctx context.Context) bool {
	if m, ok := ctx.Value(runCompleteMarkerKey{}).(*runCompleteMarker); ok {
		return m.published.Load()
	}
	return false
}
