package agent

import (
	"sync"

	"github.com/charmbracelet/crush/internal/csync"
)

// sessionOwnership is the per-session run bookkeeping of one workspace:
// which sessions have an active turn, which prompts are queued behind
// them, and which accepted-but-not-yet-active prompts a cancel covers.
//
// It is shared by every agent a coordinator builds — the coder agent,
// the `agent` tool's sub-agent, and agenticFetchTool's inline agent —
// because a session can only have one turn at a time no matter which
// agent runs it. While this state was per agent instance, a child
// session running on the sub-agent was invisible to
// coordinator.IsSessionBusy and out of coordinator.Cancel's reach, and a
// prompt dispatched into that session passed the coder agent's own busy
// check and started a second concurrent turn on it.
//
// sessionAgent embeds it by pointer, so every field is reached as though
// it still lived on the agent.
type sessionOwnership struct {
	messageQueue   *csync.Map[string, []SessionAgentCall]
	activeRequests *csync.Map[string, *activeCancel]

	// dispatchMu holds a per-session mutex that serializes the
	// accepted -> (cancel-on-entry | queued | active) transition in
	// Run against a concurrent Cancel. The lock is held only during
	// the brief handoff (no DB or LLM I/O under the lock).
	dispatchMu *csync.Map[string, *sync.Mutex]
	// acceptedRuns counts dispatched-but-not-yet-active runs per
	// session. A counter > 0 means a dispatched prompt is in flight
	// and has not yet completed the dispatch handoff in Run. Only
	// BeginAccepted increments it; only AcceptedRun.Close decrements
	// it.
	acceptedRuns *csync.Map[string, int]
	// cancelMark records, per session, a high-water accept sequence: an
	// accepted handle is canceled by it iff the handle's sequence is at
	// or below the mark. Cancel raises the mark to the latest sequence
	// assigned at cancel time, so a single Cancel covers every prompt
	// accepted-but-not-yet-active then, while a prompt accepted later
	// (higher sequence) is never poisoned. Absent or 0 means no pending
	// cancel. It is only raised by Cancel when acceptedRuns > 0, so an
	// idle Escape never records a mark.
	cancelMark *csync.Map[string, uint64]
	// dispatchMuCreate guards lazy creation of per-session entries in
	// dispatchMu so two goroutines can't race to lock different mutex
	// instances for the same session.
	dispatchMuCreate sync.Mutex
	// acceptedMu serializes increments/decrements of acceptedRuns and
	// the assignment of accept sequence numbers from acceptSeqGen. It
	// is separate from dispatchMu so AcceptedRun.Close (which may run
	// while Run holds dispatchMu for the same session) does not
	// deadlock by re-entering the dispatch lock.
	acceptedMu sync.Mutex
	// acceptSeqGen is the monotonic source of accept sequence numbers.
	// Each BeginAccepted increments it under acceptedMu and stamps the
	// returned handle, so sequences strictly increase in accept order.
	// Cancel uses its current value as the per-session high-water mark.
	acceptSeqGen uint64
}

func newSessionOwnership() *sessionOwnership {
	return &sessionOwnership{
		messageQueue:   csync.NewMap[string, []SessionAgentCall](),
		activeRequests: csync.NewMap[string, *activeCancel](),
		dispatchMu:     csync.NewMap[string, *sync.Mutex](),
		acceptedRuns:   csync.NewMap[string, int](),
		cancelMark:     csync.NewMap[string, uint64](),
	}
}
