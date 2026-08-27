package backend

import (
	"context"
	"log/slog"
	"time"

	"github.com/charmbracelet/crush/internal/proto"
)

// Why a dispatched run was cancelled. These reach the server log only.
//
// They are deliberately not carried as the run context's cancel cause:
// net/http reports the cause of a cancelled request context as the
// request's error, so a cause would travel out through the provider call
// and reach the agent instead of context.Canceled. The agent branches
// its whole cancellation cleanup on errors.Is(err, context.Canceled)
// (FinishReasonCanceled, the cancelled terminal event, the "user
// cancelled" tool-call repair text), so a run ended by the server would
// have been recorded as a provider error.
const (
	reasonRequested  = "cancelled by request"
	reasonClientGone = "the client that requested it is gone"
	reasonMaxRun     = "maximum run duration exceeded"
)

// DefaultMaxRunDuration bounds how long any single dispatched agent run
// may live. A run is not tied to the HTTP request that started it and
// can outlive every client, so without a ceiling a turn stuck on
// something that never returns stays on the server until it is
// restarted. The default is far longer than any real turn; it exists to
// make "forever" impossible, not to interrupt work. Overridable via
// CRUSH_SERVER_MAX_RUN_DURATION (seconds; 0 removes the bound).
var DefaultMaxRunDuration = 6 * time.Hour

// runHandle is one dispatched agent run, tracked so it can be ended
// without ending the session's other work.
//
// A run is registered by [Backend.SendMessage] before the goroutine that
// executes it is scheduled, so a cancel arriving in that window still
// finds it, and is released by [Backend.runAgent] on return. A prompt
// dispatched into a busy session is queued rather than run at once, and
// runAgent stays on the stack until that prompt's own turn has ended, so
// the handle covers the wait as well as the turn.
type runHandle struct {
	// runID is the caller's correlator (proto.AgentMessage.RunID). It
	// may be empty: callers that never wait for a terminal event do not
	// have to mint one, and such a run is simply not addressable by
	// [Backend.CancelRun].
	runID string
	// sessionID is the session the run is prompting. Recorded for logs
	// only; ending a run must not end a session.
	sessionID string
	// clientID is the client that asked for the run
	// (proto.AgentMessage.ClientID), or empty for an unowned run. When
	// set, losing that client's workspace claim cancels the run.
	clientID string
	ctx      context.Context
	cancel   context.CancelFunc
	// maxDuration, when non-nil, is the armed ceiling timer.
	maxDuration *time.Timer
}

// newRun builds the run context for msg and registers it on the
// workspace. The context is a child of the workspace context, so
// workspace shutdown still ends the run; the extra cancel layer is what
// lets one run be ended on its own.
//
// maxDuration <= 0 leaves the run unbounded in time.
func (w *Workspace) newRun(msg proto.AgentMessage, maxDuration time.Duration) *runHandle {
	ctx, cancel := context.WithCancel(w.ctx)
	h := &runHandle{
		runID:     msg.RunID,
		sessionID: msg.SessionID,
		clientID:  msg.ClientID,
		ctx:       ctx,
		cancel:    cancel,
	}
	if maxDuration > 0 {
		h.maxDuration = time.AfterFunc(maxDuration, func() {
			slog.Warn("Agent run exceeded the maximum run duration, cancelling",
				"session_id", h.sessionID, "run_id", h.runID, "max_duration", maxDuration)
			h.cancel()
		})
	}

	w.runsMu.Lock()
	defer w.runsMu.Unlock()
	if w.runs == nil {
		w.runs = make(map[*runHandle]struct{})
	}
	w.runs[h] = struct{}{}
	return h
}

// end releases the handle once its run has returned — which, for a
// prompt that had to queue behind a busy session, is after that
// prompt's own turn, not after it was queued. It stops the ceiling
// timer, cancels the run context so nothing derived from it leaks, and
// deregisters the handle.
func (w *Workspace) end(h *runHandle) {
	w.runsMu.Lock()
	delete(w.runs, h)
	w.runsMu.Unlock()

	if h.maxDuration != nil {
		h.maxDuration.Stop()
	}
	h.cancel()
}

// cancelRun ends every registered run carrying runID and reports how
// many it cancelled. An empty runID matches nothing: a run that supplied
// no correlator is not addressable.
func (w *Workspace) cancelRun(runID, reason string) int {
	if runID == "" {
		return 0
	}
	return w.cancelRuns(reason, func(h *runHandle) bool { return h.runID == runID })
}

// cancelClientRuns ends every registered run owned by clientID and
// reports how many it cancelled. An empty clientID matches nothing:
// unowned runs have no client whose departure could end them.
func (w *Workspace) cancelClientRuns(clientID, reason string) int {
	if clientID == "" {
		return 0
	}
	return w.cancelRuns(reason, func(h *runHandle) bool { return h.clientID == clientID })
}

// cancelRuns cancels the registered runs matching want. The handles are
// collected under the lock and cancelled after it is dropped, so a
// cancel that synchronously fans out to child contexts never runs with
// the registry held.
func (w *Workspace) cancelRuns(reason string, want func(*runHandle) bool) int {
	w.runsMu.Lock()
	var matched []*runHandle
	for h := range w.runs {
		if want(h) {
			matched = append(matched, h)
		}
	}
	w.runsMu.Unlock()

	for _, h := range matched {
		slog.Info("Cancelling agent run",
			"session_id", h.sessionID, "run_id", h.runID,
			"client_id", h.clientID, "reason", reason)
		h.cancel()
	}
	return len(matched)
}

// CancelRun ends the dispatched agent run identified by runID, leaving
// any other work on the same session running.
//
// It is deliberately forgiving: a run that has already finished is no
// longer registered and reports success, so a client can cancel
// unconditionally on its way out without racing normal completion.
// Only an unknown workspace is an error.
func (b *Backend) CancelRun(workspaceID, runID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	ws.cancelRun(runID, reasonRequested)
	return nil
}
