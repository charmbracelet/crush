package agent

import (
	"context"
	"sync/atomic"
)

// runCompleteMarkerKey is the unexported context key carrying a
// [runCompleteMarker] from the dispatch boundary (backend.runAgent)
// down into the coordinator. It lets the dispatcher learn whether the
// coordinator already published the authoritative terminal
// notify.RunComplete for the run, so a fallback terminal event is only
// emitted when one is actually missing (e.g. an error returned before
// sessionAgent.Run ever executed). It avoids a breaking change to the
// Coordinator interface.
type runCompleteMarkerKey struct{}

// runCompleteMarker records whether a terminal RunComplete has been
// published for a run. It is shared by pointer through the context so
// a publish deep in the call stack is observable by the dispatcher
// after the call returns.
type runCompleteMarker struct {
	published atomic.Bool
}

// WithRunCompleteMarker returns ctx carrying a fresh marker the
// coordinator can flag via [MarkRunCompletePublished] once it emits the
// run's terminal RunComplete. Callers read the result with
// [RunCompletePublished]. Attaching the marker is optional: code paths
// without one simply skip the dedup signal.
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

// RunCompletePublished reports whether [MarkRunCompletePublished] was
// called on ctx's marker. It returns false when no marker is present.
func RunCompletePublished(ctx context.Context) bool {
	if m, ok := ctx.Value(runCompleteMarkerKey{}).(*runCompleteMarker); ok {
		return m.published.Load()
	}
	return false
}
