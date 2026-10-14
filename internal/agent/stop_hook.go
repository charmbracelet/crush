package agent

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"sync"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// maxStopHookReruns caps how many times Stop hooks can send one turn back
// to the agent, so a hook that always blocks cannot loop forever.
const maxStopHookReruns = 3

// NewCoordinatorWithStopHooks is [NewCoordinator] gated by the Stop hooks
// in opts.Config. Like PreToolUse hooks, they are read from the current
// config on every run, so a config reload applies to the next turn.
func NewCoordinatorWithStopHooks(ctx context.Context, opts CoordinatorOptions) (Coordinator, error) {
	if opts.RunComplete != nil {
		opts.RunComplete = holdRunComplete(opts.RunComplete)
	}
	inner, err := NewCoordinator(ctx, opts)
	if err != nil {
		return nil, err
	}
	return WithStopHooks(inner, func() *hooks.Runner {
		stop := opts.Config.Config().Hooks[hooks.EventStop]
		if len(stop) == 0 {
			return nil
		}
		return hooks.NewRunner(stop, opts.Config.WorkingDir(), opts.Config.WorkingDir())
	}), nil
}

// stopHookCoordinator runs Stop hooks when a top-level turn is about to
// finish. A blocking hook sends the turn back to the agent with the hook's
// reason as the next prompt.
type stopHookCoordinator struct {
	Coordinator
	stopHooks func() *hooks.Runner

	mu    sync.Mutex
	gates map[*stopGate]struct{} // Gates still running, for Cancel.
}

// WithStopHooks wraps inner so Stop hooks gate the end of every Run and
// RunAccepted. stopHooks returns the runner for the current config, or nil
// when no Stop hook is configured; WithStopHooks returns inner unchanged
// when stopHooks is nil. For the gate to hold back a blocked attempt's
// RunComplete, inner must publish through [holdRunComplete].
func WithStopHooks(inner Coordinator, stopHooks func() *hooks.Runner) Coordinator {
	if stopHooks == nil {
		return inner
	}
	return &stopHookCoordinator{Coordinator: inner, stopHooks: stopHooks, gates: map[*stopGate]struct{}{}}
}

// Cancel implements Coordinator. It also stops the session's Stop gates,
// killing a running hook and dropping any pending re-run.
func (s *stopHookCoordinator) Cancel(sessionID string) {
	s.eachGate(sessionID, false, func(g *stopGate) { g.cancel() })
	s.Coordinator.Cancel(sessionID)
}

// CancelAll implements Coordinator.
func (s *stopHookCoordinator) CancelAll() {
	s.eachGate("", true, func(g *stopGate) { g.cancel() })
	s.Coordinator.CancelAll()
}

// IsSessionBusy implements Coordinator. A session stays busy while its Stop
// hook runs and until the re-run starts, though no turn is active then.
func (s *stopHookCoordinator) IsSessionBusy(sessionID string) bool {
	var gated bool
	s.eachGate(sessionID, false, func(*stopGate) { gated = true })
	return gated || s.Coordinator.IsSessionBusy(sessionID)
}

// IsBusy implements Coordinator.
func (s *stopHookCoordinator) IsBusy() bool {
	var gated bool
	s.eachGate("", true, func(*stopGate) { gated = true })
	return gated || s.Coordinator.IsBusy()
}

// eachGate calls fn on the live gates of sessionID, or of every session
// when all is set.
func (s *stopHookCoordinator) eachGate(sessionID string, all bool, fn func(*stopGate)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for g := range s.gates {
		if all || g.sessionID == sessionID {
			fn(g)
		}
	}
}

// Run implements Coordinator.
func (s *stopHookCoordinator) Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	return s.gate(ctx, sessionID, func(ctx context.Context) (*fantasy.AgentResult, error) {
		return s.Coordinator.Run(ctx, sessionID, prompt, attachments...)
	})
}

// RunAccepted implements Coordinator.
func (s *stopHookCoordinator) RunAccepted(ctx context.Context, accept *AcceptedRun, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	return s.gate(ctx, sessionID, func(ctx context.Context) (*fantasy.AgentResult, error) {
		return s.Coordinator.RunAccepted(ctx, accept, sessionID, prompt, attachments...)
	})
}

func (s *stopHookCoordinator) gate(ctx context.Context, sessionID string, first func(context.Context) (*fantasy.AgentResult, error)) (result *fantasy.AgentResult, err error) {
	gateCtx, cancel := context.WithCancel(ctx)
	g := &stopGate{sessionID: sessionID, runID: RunIDFromContext(ctx), cancel: cancel}
	s.mu.Lock()
	s.gates[g] = struct{}{}
	s.mu.Unlock()
	var canceled bool
	// Every RunComplete of the session published under the gate is held,
	// and each attempt's publish is recorded on a private marker; the held
	// events are released when the gate ends, and only then is the caller's
	// marker (backend.runAgent) set.
	defer func() {
		s.mu.Lock()
		delete(s.gates, g)
		s.mu.Unlock()
		cancel()
		if g.release(ctx, canceled, err) {
			MarkRunCompletePublished(ctx)
		}
	}()
	attemptCtx := WithRunCompleteMarker(context.WithValue(ctx, stopGateKey{}, g))

	result, err = first(attemptCtx)
	// A nil result means the prompt was queued behind a busy turn or
	// canceled on entry: no turn finished here, so there is no gate.
	for reruns := 0; err == nil && result != nil; reruns++ {
		runner := s.stopHooks()
		if runner == nil {
			return result, err
		}
		if canceled = gateCtx.Err() != nil; canceled {
			return result, err
		}
		agg, hookErr := runner.Run(gateCtx, hooks.EventStop, sessionID, "", fmt.Sprintf(`{"stop_hook_active":%t}`, reruns > 0))
		if canceled = gateCtx.Err() != nil; canceled {
			return result, err
		}
		if hookErr != nil || agg.Decision != hooks.DecisionDeny || agg.Halt {
			return result, err
		}
		if reruns == maxStopHookReruns {
			slog.Warn("Stop hook kept blocking", "session_id", sessionID, "reruns", reruns, "reason", agg.Reason)
			return result, err
		}
		// Reserve the re-run before the last cancel check: a Cancel after
		// it records a cancel mark the re-run observes on entry.
		accept := s.Coordinator.BeginAccepted(sessionID)
		if canceled = gateCtx.Err() != nil; canceled {
			accept.Close()
			return result, err
		}
		g.nextAttempt()
		// An empty prompt fails validation, so a reasonless block still
		// needs something to send back.
		result, err = s.Coordinator.RunAccepted(attemptCtx, accept, sessionID, cmp.Or(agg.Reason, "A Stop hook blocked finishing. Continue the task."))
	}
	return result, err
}

type stopGateKey struct{}

// stopGate is one gated run. Until it ends it holds every RunComplete its
// attempts publish for its session: its own RunID's, and those of prompts
// with their own RunID that a turn drained from the queue.
type stopGate struct {
	sessionID, runID string
	cancel           context.CancelFunc

	mu      sync.Mutex
	pub     pubsub.Publisher[notify.RunComplete]
	held    []heldRunComplete // The latest per RunID, in first-publish order.
	attempt int
}

type heldRunComplete struct {
	rc      notify.RunComplete
	attempt int
}

func (g *stopGate) hold(pub pubsub.Publisher[notify.RunComplete], rc notify.RunComplete) bool {
	// Sub-agent turns (child sessions) run on this context too: let them
	// through.
	if rc.SessionID != g.sessionID {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pub = pub
	h := heldRunComplete{rc: rc, attempt: g.attempt}
	for i := range g.held {
		if g.held[i].rc.RunID == rc.RunID {
			g.held[i] = h
			return true
		}
	}
	g.held = append(g.held, h)
	return true
}

func (g *stopGate) nextAttempt() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.attempt++
}

// release publishes every held event, marked cancelled when the gate was
// cancelled, and reports whether one carried the gate's own RunID. An event
// from a blocked attempt carries the final attempt's result instead, under
// its own RunID. If the final attempt published nothing (it failed before
// its turn, or was queued), the gate's own event is left to the caller's
// fallback or the queued re-run, and the others carry err, if any.
func (g *stopGate) release(ctx context.Context, canceled bool, err error) (own bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	final, ok := heldRunComplete{}, false
	for _, h := range g.held {
		if h.rc.RunID == g.runID && h.attempt == g.attempt {
			final, ok = h, true
		}
	}
	for _, h := range g.held {
		rc := h.rc
		switch {
		case h.attempt == g.attempt:
		case ok:
			rc = final.rc
			rc.RunID = h.rc.RunID
		case h.rc.RunID == g.runID:
			continue
		case err != nil:
			rc = notify.RunComplete{SessionID: rc.SessionID, RunID: rc.RunID, Error: err.Error()}
		}
		rc.Cancelled = rc.Cancelled || canceled
		g.pub.PublishMustDeliver(ctx, pubsub.UpdatedEvent, rc)
		own = own || rc.RunID == g.runID
	}
	return own
}

// holdRunComplete wraps pub so RunComplete events published under a Stop
// gate are held until the gate lets the turn end, instead of reaching
// subscribers once per attempt. Outside a gate it passes events through.
func holdRunComplete(pub pubsub.Publisher[notify.RunComplete]) pubsub.Publisher[notify.RunComplete] {
	return holdingPublisher{pub}
}

type holdingPublisher struct {
	pubsub.Publisher[notify.RunComplete]
}

func (p holdingPublisher) PublishMustDeliver(ctx context.Context, t pubsub.EventType, rc notify.RunComplete) {
	if g, ok := ctx.Value(stopGateKey{}).(*stopGate); ok && g.hold(p.Publisher, rc) {
		return
	}
	p.Publisher.PublishMustDeliver(ctx, t, rc)
}
