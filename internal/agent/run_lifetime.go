package agent

import (
	"context"
	"sync"
)

// runLifetimeKey is the unexported context key carrying a [RunLifetime]
// from the dispatch boundary (backend.runAgent) down into the
// coordinator, which copies it onto SessionAgentCall.Lifetime. It avoids
// a breaking change to the Coordinator interface, the same way
// [WithRunID] and [WithRunCompleteMarker] do.
type runLifetimeKey struct{}

// RunLifetime is the dispatched lifetime of one agent run: the context
// that bounds it, plus the rendezvous that reports when the run's own
// turn has ended.
//
// A prompt dispatched into a busy session is queued and only runs later,
// in the frame of the turn it queued behind. Its dispatcher owns the
// state that makes it addressable — the registered run handle, the
// cancellation that reaches it and the armed maximum-duration timer —
// and that state only lives as long as the dispatching call is on the
// stack. So the dispatcher waits: [SessionAgent.Run] returns to it only
// once this rendezvous reports that the queued call's own turn is over.
//
// Ctx is what keeps the two runs apart. Without it a queued call
// inherited the context of whichever turn dequeued it, which reversed
// ownership in both directions: cancelling the already-finished earlier
// run killed the queued turn, and cancelling the queued run did nothing
// at all.
type RunLifetime struct {
	// Ctx bounds this call's own turn. It is the context the dispatcher
	// registered the run under, never the context of the turn that
	// happens to dequeue the call.
	Ctx context.Context

	// done is closed once the call's own turn has ended, releasing the
	// dispatcher.
	done chan struct{}
	once sync.Once

	// started reports whether the call has left the queue and been
	// handed to a turn. It is written and read only while the session's
	// dispatch mutex is held, so a cancellation and the end-of-turn
	// handoff cannot both claim the same queued call.
	started bool
}

// WithRunLifetime returns ctx carrying a fresh lifetime bound to ctx
// itself. Only the dispatcher that owns a run's handle and duration
// bound should attach one; callers without a per-run context (the
// in-process path) leave it unset, and their queued prompts keep the
// older fire-and-forget behaviour.
func WithRunLifetime(ctx context.Context) context.Context {
	return context.WithValue(ctx, runLifetimeKey{}, &RunLifetime{
		Ctx:  ctx,
		done: make(chan struct{}),
	})
}

// RunLifetimeFromContext returns the lifetime set by [WithRunLifetime],
// or nil when none was set.
func RunLifetimeFromContext(ctx context.Context) *RunLifetime {
	l, _ := ctx.Value(runLifetimeKey{}).(*RunLifetime)
	return l
}

// finish releases the dispatcher waiting for this run. It is idempotent
// because several paths can end a call's lifetime — its own turn, a fold
// into another turn, or a drop from the queue — and the losing paths must
// stay harmless.
func (l *RunLifetime) finish() {
	l.once.Do(func() { close(l.done) })
}

// finishRunLifetimes releases the dispatcher of every call whose run
// ended without a turn of its own: folded into another turn, or dropped
// from the queue.
func finishRunLifetimes(calls []SessionAgentCall) {
	for _, c := range calls {
		if c.Lifetime != nil {
			c.Lifetime.finish()
		}
	}
}

// runContextFor reports the context a dequeued call's turn must run
// under: its own dispatched lifetime when it has one, so it neither
// inherits the cancellation of the turn that dequeued it nor loses its
// own. A call without a lifetime keeps the dequeuing frame's context, as
// before.
func runContextFor(ctx context.Context, call SessionAgentCall) context.Context {
	if call.Lifetime != nil {
		return call.Lifetime.Ctx
	}
	return ctx
}
