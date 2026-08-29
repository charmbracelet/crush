// Package agent is the core orchestration layer for Crush AI agents.
//
// It provides session-based AI agent functionality for managing
// conversations, tool execution, and message handling. It coordinates
// interactions between language models, messages, sessions, and tools while
// handling features like automatic summarization, queuing, and token
// management.
package agent

import (
	"cmp"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/bedrock"
	"charm.land/fantasy/providers/google"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openrouter"
	"charm.land/fantasy/providers/vercel"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/stringext"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
)

const (
	DefaultSessionName = "Untitled Session"

	// Constants for auto-summarization thresholds
	largeContextWindowThreshold = 200_000
	largeContextWindowBuffer    = 20_000
	smallContextWindowRatio     = 0.2
)

var userAgent = fmt.Sprintf("Charm-Crush/%s (https://charm.land/crush)", version.Version)

//go:embed templates/title.md
var titlePrompt []byte

//go:embed templates/summary.md
var summaryPrompt []byte

// Used to remove <think> tags from generated titles.
var (
	thinkTagRegex       = regexp.MustCompile(`(?s)<think>.*?</think>`)
	orphanThinkTagRegex = regexp.MustCompile(`</?think>`)
)

type SessionAgentCall struct {
	SessionID string
	// RunID, when non-empty, is the caller-supplied correlator that
	// gets echoed back on the notify.RunComplete event emitted for
	// this turn. It is preserved when the call is enqueued behind a
	// busy session so the queued turn's terminal event is still
	// recognisable to the original caller. Callers that need a
	// reliable completion contract (e.g. `crush run` against a
	// session that may be busy) MUST set it; SessionID alone is
	// ambiguous when concurrent turns share the same session.
	RunID string
	// AutoApprove asks the run to grant every permission request the
	// turn makes. The hold is taken when this call becomes the active
	// turn and released when the turn ends, so it cannot be revoked
	// mid-turn by a caller that exited, and it cannot outlive the run.
	// It survives the busy-session queue, so a queued prompt gets its
	// own hold when its own turn starts.
	AutoApprove bool
	// Models, when non-nil, is the model pair this run owns. It is
	// resolved once when the run starts and is the only model state the
	// turn reads, so a workspace-wide model change (the picker,
	// UpdateModels) cannot alter the turn's model between steps, and two
	// concurrent runs started with different models cannot steal each
	// other's. It survives the busy-session queue, so a prompt queued
	// behind another turn still runs on the model it asked for. When nil
	// the turn falls back to the agent's own models, which is what
	// in-package callers that build a SessionAgent directly rely on.
	Models *runModels
	// SystemPrompt, when non-empty, is the system prompt this turn runs
	// with instead of the agent's own. It exists because the prompt is
	// rendered for a specific provider and model
	// (prompt.Prompt.Build), so a turn that pinned a model other than
	// the one the shared agent was built for has to bring its own. The
	// `agent` tool sets it: a delegated turn inherits the spawning
	// run's models, and one shared sub-agent instance serves every
	// parent run, so its prompt cannot live on the instance.
	SystemPrompt     string
	Prompt           string
	ProviderOptions  fantasy.ProviderOptions
	Attachments      []message.Attachment
	MaxOutputTokens  int64
	Temperature      *float64
	TopP             *float64
	TopK             *int64
	FrequencyPenalty *float64
	PresencePenalty  *float64
	// SubAgent marks a turn started by the `agent` tool on a child
	// session. Such a turn publishes no user-facing "agent finished"
	// notification: the parent turn is still running and owns that
	// signal.
	SubAgent bool
	// NonInteractive marks a turn nobody can answer prompts for
	// (`crush run`, local or client/server). Interactive-only tools
	// are withheld from its palette, because a call to one would
	// block until the run was cancelled. It is a property of the
	// turn, not of the agent: one workspace on the shared server
	// serves an attached TUI and headless prompts at the same time.
	NonInteractive bool
	// OnComplete, when non-nil, replaces the default RunComplete
	// publish path: the inner Run hands the terminal payload to this
	// callback instead of emitting it on the RunComplete broker. The
	// coordinator uses this hook to coalesce the unauthorized →
	// re-auth → retry chain into a single user-visible terminal
	// event, so non-interactive clients (e.g. `crush run`) don't
	// exit on a stale failed-attempt RunComplete before the
	// successful retry. It is intentionally stripped when queueing
	// a busy-session call (see Run): the originating
	// coordinator.Run has long returned by the time the queued
	// recursion drains, so falling back to the default broker
	// publish keeps the event visible to subscribers.
	OnComplete func(notify.RunComplete)
	// Accepted, when non-nil, is the accept reservation taken by
	// BeginAccepted before the call was dispatched onto a goroutine
	// (the client/server fire-and-forget path). Run consumes it under
	// dispatchMu[SessionID] once the accepted -> (cancel-on-entry |
	// queued | active) transition has been chosen. When nil
	// (in-process / local callers like AppWorkspace), behavior is
	// unchanged and no accept tracking applies.
	Accepted *AcceptedRun
	// acceptSeq carries the accept sequence of the handle that produced
	// this call after it has been enqueued and its Accepted handle
	// stripped. The queue-drain paths compare it against a session's
	// cancel mark so a follow-up queued before a cancel is dropped while
	// one queued after the cancel survives. 0 means untracked (an
	// in-process enqueue with no accept reservation), which the drain
	// paths treat as covered by any present mark, preserving the
	// pre-sequence behavior.
	acceptSeq uint64
	// Lifetime, when non-nil, is the dispatched lifetime this call
	// belongs to: the run context that bounds it and the rendezvous
	// that releases its dispatcher. It survives the busy-session queue,
	// which is the point — a queued prompt stays bound to the run that
	// asked for it instead of to the turn that eventually dequeues it.
	// Nil for in-process callers, which own no per-run context. See
	// [RunLifetime].
	Lifetime *RunLifetime
	// OnAuthRefresh, when non-nil, is called by fantasy when a stream
	// fails with an authentication error (HTTP 401). The callback should
	// refresh credentials and return nil on success, in which case
	// fantasy retries the stream transparently. Returning an error
	// surfaces the original auth error without retry.
	OnAuthRefresh func(ctx context.Context, err *fantasy.ProviderError) error
}

// SummarizeCall is one request to replace a session's history with a
// summary. Auto-compaction issues it from inside a turn, so it carries
// the same per-run state a SessionAgentCall does.
type SummarizeCall struct {
	SessionID       string
	ProviderOptions fantasy.ProviderOptions
	// Models, when non-nil, is the model pair to summarize with. Auto
	// compaction is part of the run that filled the context window, so
	// it must summarize on that run's model rather than the workspace's.
	Models *runModels
	// OnAuthRefresh mirrors SessionAgentCall.OnAuthRefresh.
	OnAuthRefresh func(ctx context.Context, err *fantasy.ProviderError) error
}

type SessionAgent interface {
	Run(context.Context, SessionAgentCall) (*fantasy.AgentResult, error)
	BeginAccepted(sessionID string) *AcceptedRun
	SetModels(large Model, small Model)
	SetTools(tools []fantasy.AgentTool)
	SetSystemPrompt(systemPrompt string)
	Cancel(sessionID string)
	CancelAll()
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	QueuedPrompts(sessionID string) int
	QueuedPromptsList(sessionID string) []string
	ClearQueue(sessionID string)
	Summarize(context.Context, SummarizeCall) error
	Model() Model
	GenerateTitle(ctx context.Context, sessionID, userPrompt string)
}

type Model struct {
	Model      fantasy.LanguageModel
	CatwalkCfg catwalk.Model
	ModelCfg   config.SelectedModel
	FlatRate   bool
	// SystemPromptPrefix is the extra system message this model's
	// provider wants in front of every prompt
	// (ProviderConfig.SystemPromptPrefix). It rides on the model
	// because that is the only thing that knows which provider a turn
	// is actually talking to: a run can pin a model other than the
	// workspace's (`crush run -m`), a delegated turn inherits its
	// parent's, and the large and small slots can belong to different
	// providers.
	SystemPromptPrefix string
}

// activeCancel wraps a context.CancelFunc with a unique pointer identity.
// The pointer is used for compare-and-delete in the dispatch completion path:
// when a finishing run's deferred cleanup fires, it must only remove its own
// entry — not a newer run's entry that was installed in the window between
// the explicit Del and the function return.
type activeCancel struct {
	cancel context.CancelFunc
}

type sessionAgent struct {
	largeModel   *csync.Value[Model]
	smallModel   *csync.Value[Model]
	systemPrompt *csync.Value[string]
	tools        *csync.Slice[fantasy.AgentTool]

	isSubAgent           bool
	sessions             session.Service
	messages             message.Service
	disableAutoSummarize bool
	isYolo               bool
	notify               pubsub.Publisher[notify.Notification]
	runComplete          pubsub.Publisher[notify.RunComplete]
	// permissions is only needed for SessionAgentCall.AutoApprove, so
	// it may be nil: constructors that never set that flag (tests,
	// sub-agents built without a permission service) keep working.
	permissions permission.Service

	// sessionOwnership is the per-session run bookkeeping. It is a
	// pointer because every agent a coordinator builds shares one: a
	// session has one turn at a time whichever agent runs it.
	*sessionOwnership
}

type SessionAgentOptions struct {
	LargeModel           Model
	SmallModel           Model
	SystemPrompt         string
	IsSubAgent           bool
	DisableAutoSummarize bool
	IsYolo               bool
	Sessions             session.Service
	Messages             message.Service
	Tools                []fantasy.AgentTool
	Notify               pubsub.Publisher[notify.Notification]
	RunComplete          pubsub.Publisher[notify.RunComplete]
	// Permissions backs SessionAgentCall.AutoApprove. Leave it nil when
	// no caller sets that flag.
	Permissions permission.Service
	// Ownership, when non-nil, is the per-session run bookkeeping this
	// agent shares with the rest of its coordinator's agents. Leave it
	// nil for a standalone agent, which then owns its sessions alone.
	Ownership *sessionOwnership
}

func NewSessionAgent(
	opts SessionAgentOptions,
) SessionAgent {
	return &sessionAgent{
		largeModel:           csync.NewValue(opts.LargeModel),
		smallModel:           csync.NewValue(opts.SmallModel),
		systemPrompt:         csync.NewValue(opts.SystemPrompt),
		isSubAgent:           opts.IsSubAgent,
		sessions:             opts.Sessions,
		messages:             opts.Messages,
		disableAutoSummarize: opts.DisableAutoSummarize,
		tools:                csync.NewSliceFrom(opts.Tools),
		isYolo:               opts.IsYolo,
		notify:               opts.Notify,
		runComplete:          opts.RunComplete,
		permissions:          opts.Permissions,
		sessionOwnership:     cmp.Or(opts.Ownership, newSessionOwnership()),
	}
}

// AcceptedRun owns exactly one accept reservation taken by
// BeginAccepted. It is the only carrier of accept-state across the
// backend.runAgent / Coordinator.Run / sessionAgent.Run layers: a
// counter > 0 means a dispatched prompt is in flight and has not yet
// completed the dispatch handoff in Run. Close is the only way to
// release the reservation and is idempotent.
type AcceptedRun struct {
	agent     *sessionAgent
	sessionID string
	// seq is the monotonic accept sequence stamped by BeginAccepted. A
	// cancel covers this handle iff seq is at or below the session's
	// cancel mark, so a handle accepted after a cancel (higher seq) is
	// never poisoned by it.
	seq  uint64
	done atomic.Bool
}

// Close decrements the accept counter for this reservation. It is safe
// to call multiple times; only the first call has effect.
func (r *AcceptedRun) Close() {
	if r == nil {
		return
	}
	if !r.done.CompareAndSwap(false, true) {
		return
	}
	r.agent.endAccepted(r.sessionID)
}

// SessionID exposes the session this reservation is for so the run path
// can use it without an extra parameter.
func (r *AcceptedRun) SessionID() string {
	if r == nil {
		return ""
	}
	return r.sessionID
}

// BeginAccepted increments the accept counter for sessionID and returns
// a handle whose Close is the only way to decrement it. It is the only
// entry point that mutates acceptedRuns.
func (a *sessionAgent) BeginAccepted(sessionID string) *AcceptedRun {
	a.acceptedMu.Lock()
	defer a.acceptedMu.Unlock()
	count, _ := a.acceptedRuns.Get(sessionID)
	a.acceptedRuns.Set(sessionID, count+1)
	a.acceptSeqGen++
	return &AcceptedRun{agent: a, sessionID: sessionID, seq: a.acceptSeqGen}
}

// endAccepted decrements the accept counter for sessionID. It is only
// called via AcceptedRun.Close. It uses a dedicated lock (not the
// per-session dispatch mutex) so it can run while Run holds dispatchMu
// for the same session without deadlocking.
//
// When the count reaches zero the session's cancel mark is dropped: no
// accepted handle remains for it to cover, and any handle accepted later
// gets a strictly higher sequence that the mark would not match anyway.
// Handles canceled on entry never reach RunComplete, so this is the only
// place that clears the mark for an all-canceled batch. Sibling handles
// covered by the same mark are serialized on the per-session dispatch
// mutex and read the mark before they Close, so this never clears it out
// from under a covered handle still waiting to enter Run.
func (a *sessionAgent) endAccepted(sessionID string) {
	a.acceptedMu.Lock()
	defer a.acceptedMu.Unlock()
	count, ok := a.acceptedRuns.Get(sessionID)
	if !ok || count <= 1 {
		a.acceptedRuns.Del(sessionID)
		a.cancelMark.Del(sessionID)
		return
	}
	a.acceptedRuns.Set(sessionID, count-1)
}

// sessionMu returns the per-session dispatch mutex, creating it on first
// use. Creation is guarded so concurrent callers always observe the same
// mutex instance for a given session.
func (a *sessionAgent) sessionMu(sessionID string) *sync.Mutex {
	if mu, ok := a.dispatchMu.Get(sessionID); ok {
		return mu
	}
	a.dispatchMuCreate.Lock()
	defer a.dispatchMuCreate.Unlock()
	if mu, ok := a.dispatchMu.Get(sessionID); ok {
		return mu
	}
	mu := &sync.Mutex{}
	a.dispatchMu.Set(sessionID, mu)
	return mu
}

// enqueueCall appends call to the session's message queue. The
// OnComplete hook is stripped: the caller that supplied it (typically
// coordinator.Run) has its own retry/coalesce scope that ends when it
// returns, so by the time the queue drains nobody is left to consume the
// buffered terminal event. The recursive Run falls back to the default
// broker publish, which is what existing subscribers expect for queued
// turns.
func (a *sessionAgent) enqueueCall(call SessionAgentCall) {
	existing, ok := a.messageQueue.Get(call.SessionID)
	if !ok {
		existing = []SessionAgentCall{}
	}
	queued := call
	// Preserve the accept sequence after the handle is stripped so the
	// queue-drain paths can tell a follow-up queued before a cancel
	// (covered by the mark) from one queued after it.
	queued.acceptSeq = acceptSeqOf(call)
	queued.OnComplete = nil
	queued.Accepted = nil
	existing = append(existing, queued)
	a.messageQueue.Set(call.SessionID, existing)
}

// acceptSeqOf reports the accept sequence a call carries into the queue:
// its live accept handle's sequence when it has one, otherwise the
// sequence it was already stamped with when it was queued before.
func acceptSeqOf(call SessionAgentCall) uint64 {
	if call.Accepted != nil {
		return call.Accepted.seq
	}
	return call.acceptSeq
}

// drainQueueForStep partitions the session's queued calls for the current
// streaming step under the per-session dispatch mutex so the filtering is
// atomic against a concurrent Cancel: canceledBySeq requires the caller to
// hold that mutex, and evaluating it here (rather than after unlocking)
// prevents a cancel recorded between the drain and the check from being
// observed inconsistently.
//
// Calls covered by a pending cancel are dropped and returned in dropped,
// so the caller can release each one (publishing the terminal cancelled
// RunComplete of any that carries a RunID; a caller waiting on that
// RunID, e.g. `crush run`, would otherwise hang). Uncanceled calls
// without a RunID are returned in fold to be folded into the active
// turn, preserving the existing follow-up behavior. Uncanceled calls
// that carry a RunID are left in the queue so each runs as its own turn
// via the recursive run path and publishes its own RunComplete, giving
// every RunID-bearing prompt an explicit lifecycle instead of being
// silently absorbed into another turn. fold is processed by the caller
// without the lock held.
func (a *sessionAgent) drainQueueForStep(sessionID string) (fold, dropped []SessionAgentCall) {
	dispatchLock := a.sessionMu(sessionID)
	dispatchLock.Lock()
	defer dispatchLock.Unlock()
	queuedCalls, _ := a.messageQueue.Get(sessionID)
	var keep []SessionAgentCall
	for _, queued := range queuedCalls {
		if a.canceledBySeq(sessionID, queued.acceptSeq) {
			dropped = append(dropped, queued)
			continue
		}
		if queued.RunID != "" {
			keep = append(keep, queued)
			continue
		}
		fold = append(fold, queued)
	}
	if len(keep) == 0 {
		a.messageQueue.Del(sessionID)
	} else {
		a.messageQueue.Set(sessionID, keep)
	}
	return fold, dropped
}

// publishCanceledQueueDrops releases every queued call that was removed
// from the queue without running — covered by a pending cancel, or
// cleared by Cancel/ClearQueue — and emits a terminal cancelled
// RunComplete for those that carry a RunID. Without that event a caller
// would stay blocked on its RunID: `crush run` ignores live message
// events and exits only on a RunComplete whose RunID matches. Calls
// without a RunID have no such waiter and are dropped silently as
// before. A detached, bounded context keeps the must-deliver publish
// alive even when the run context that triggered the drop is already
// canceled.
//
// Every drop also releases its dispatcher, RunID or not: a dispatched
// prompt's run stays registered and time-bounded until its call ends,
// and being dropped from the queue is one of the ways it ends.
func (a *sessionAgent) publishCanceledQueueDrops(drops []SessionAgentCall) {
	defer finishRunLifetimes(drops)
	var hasRunID bool
	for _, d := range drops {
		if d.RunID != "" {
			hasRunID = true
			break
		}
	}
	if !hasRunID {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, d := range drops {
		if d.RunID == "" {
			continue
		}
		a.publishRunComplete(ctx, d, notify.RunComplete{
			SessionID: d.SessionID,
			RunID:     d.RunID,
			Cancelled: true,
		})
	}
}

// clearQueueAndNotify removes all queued prompts for the session and
// publishes a terminal cancelled RunComplete for any that carried a RunID,
// so callers waiting on those RunIDs (e.g. `crush run`) are not left
// hanging when their queued prompt is discarded without running.
func (a *sessionAgent) clearQueueAndNotify(sessionID string) {
	queued, ok := a.messageQueue.Get(sessionID)
	a.messageQueue.Del(sessionID)
	if !ok {
		return
	}
	a.publishCanceledQueueDrops(queued)
}

// queuedHandoff is the outcome of handOffQueue: the finished frame's
// decision about what runs next on the session.
type queuedHandoff struct {
	// Next is the prompt the caller must run in place of the frame that
	// just finished; Started reports whether there was one.
	Next    SessionAgentCall
	Started bool
	// OwesRunComplete reports whether the finishing frame still has to
	// publish its own terminal RunComplete. It does when it hands off to
	// a different prompt, because each RunID has its own lifecycle and a
	// caller waiting on this one would otherwise hang. It does not when
	// the dequeued prompt carries the same RunID — the auto-compaction
	// continuation — because that prompt's own terminal event covers it.
	OwesRunComplete bool
}

// handOffQueue is the single end-of-turn handoff to a session's queue,
// shared by Run and Summarize. It runs under the per-session dispatch
// mutex so the transition from the finished frame to the queued one is
// atomic against a concurrent Cancel: the finished frame has already
// removed its activeRequests entry, so without the lock there is a
// window in which the session looks idle and a cancel becomes a no-op
// that fails to stop the queued prompt.
//
// Under that lock it drops the queued prompts a pending cancel covers
// (releasing each one, and publishing the terminal cancelled
// RunComplete of any that carries a RunID, so no caller is left waiting
// on a prompt that never runs), appends continuation when the caller
// has one and nothing else is queued, clears the queue and any stale
// cancel mark when nothing is left, and mints a fresh accept
// reservation for the dequeued prompt so acceptedRuns stays above zero
// across the handoff into the caller's recursive Run.
//
// continuation is the auto-compaction resume: the just-finished turn's
// own call, re-prompted to pick its work back up after the summary. It
// is appended only when the queue is empty, because a prompt the user
// queued supersedes the interrupted work — resuming it after
// acknowledging the newer prompt is the agent contradicting what the
// user just asked for — and only when no pending cancel covers it.
// ownerRunID is the finishing frame's RunID, "" for a summarize.
//
// dispatchedOnly restricts the handoff to prompts that carry a RunID.
// It is what a turn that ended by error or cancellation asks for: those
// prompts are separate requests whose clients each wait for a terminal
// event of their own, while a queued prompt without a RunID is an
// interactive follow-up that belongs to the next turn the user starts —
// firing every follow-up typed during a turn that just failed is not
// what the user asked for.
func (a *sessionAgent) handOffQueue(sessionID, ownerRunID string, continuation *SessionAgentCall, dispatchedOnly bool) queuedHandoff {
	mu := a.sessionMu(sessionID)
	mu.Lock()

	queued, _ := a.messageQueue.Get(sessionID)
	var canceledDrops []SessionAgentCall
	if mark, ok := a.cancelMark.Get(sessionID); ok && mark > 0 && len(queued) > 0 {
		// A cancel was recorded for this session (e.g. it arrived while
		// this frame was active and follow-ups had been queued). Drop
		// the queued prompts it covers (accept sequence at or below the
		// mark, or untracked); keep any queued after the cancel (higher
		// sequence) so they still run.
		var kept []SessionAgentCall
		for _, q := range queued {
			if a.canceledBySeq(sessionID, q.acceptSeq) {
				canceledDrops = append(canceledDrops, q)
				continue
			}
			kept = append(kept, q)
		}
		queued = kept
		a.messageQueue.Set(sessionID, kept)
	}

	if continuation != nil && len(queued) == 0 &&
		!a.canceledBySeq(sessionID, acceptSeqOf(*continuation)) {
		a.enqueueCall(*continuation)
		queued, _ = a.messageQueue.Get(sessionID)
	}

	// next is the queue index of the prompt to hand off to.
	next := 0
	if dispatchedOnly {
		next = slices.IndexFunc(queued, func(q SessionAgentCall) bool { return q.RunID != "" })
	}

	if len(queued) == 0 {
		// No queued work. Clear the cancel mark only when no accepted
		// run remains in flight that it might still cover; otherwise a
		// sibling prompt (sequence at or below the mark) waiting to
		// enter Run would lose its cancellation. When accepted runs are
		// gone, this also clears a stale mark so it can't catch a
		// future run.
		a.messageQueue.Del(sessionID)
		a.acceptedMu.Lock()
		inFlight, _ := a.acceptedRuns.Get(sessionID)
		a.acceptedMu.Unlock()
		if inFlight == 0 {
			a.cancelMark.Del(sessionID)
		}
		mu.Unlock()
		a.publishCanceledQueueDrops(canceledDrops)
		return queuedHandoff{}
	}

	if next < 0 {
		// The queue holds only interactive follow-ups and this handoff
		// must not start those. Leave them, and leave any cancel mark,
		// for the next turn the user starts.
		mu.Unlock()
		a.publishCanceledQueueDrops(canceledDrops)
		return queuedHandoff{}
	}

	handoff := queuedHandoff{Next: queued[next], Started: true, OwesRunComplete: ownerRunID != ""}
	for _, q := range queued {
		if q.RunID == ownerRunID {
			handoff.OwesRunComplete = false
			break
		}
	}
	a.messageQueue.Set(sessionID, slices.Delete(slices.Clone(queued), next, next+1))
	// Reserve a fresh accept for the dequeued prompt before dropping the
	// lock so acceptedRuns > 0 across the handoff into the recursive
	// Run. This closes the window between this dequeue and that Run
	// registering its activeRequests entry: a cancel arriving in that
	// window now records a pending cancel (acceptedRuns > 0) that the
	// recursive Run's accepted path observes as cancel-on-entry.
	handoff.Next.Accepted = a.BeginAccepted(sessionID)
	if handoff.Next.Lifetime != nil {
		// Claim the call for its turn while the dispatch mutex is still
		// held, so a cancellation arriving now waits for that turn
		// instead of racing to take the call back out of the queue.
		handoff.Next.Lifetime.started = true
	}
	mu.Unlock()
	// Publish outside the lock: a must-deliver publish can block for
	// seconds, and the per-session dispatch mutex gates every cancel and
	// dispatch on the session.
	a.publishCanceledQueueDrops(canceledDrops)
	return handoff
}

// awaitQueued blocks the dispatcher of a queued call until that call's
// own turn has ended, or until the call's run is cancelled.
//
// Returning as soon as the prompt was queued retired the run before it
// had run: its handle was deregistered, its maximum-duration timer
// stopped and its context cancelled, so nothing could reach it and it
// later ran under the cancellation scope of the unrelated turn that
// dequeued it.
//
// On cancellation, a claim that wins the race against the end-of-turn
// handoff takes the call back out of the queue and publishes its
// terminal cancelled RunComplete, so the client waiting on that RunID is
// not left hanging. A claim that loses waits for the turn instead: that
// turn runs under this same cancelled context, so it ends promptly and
// publishes its own terminal event.
func (a *sessionAgent) awaitQueued(ctx context.Context, call SessionAgentCall) error {
	select {
	case <-call.Lifetime.done:
		return nil
	case <-ctx.Done():
	}
	taken, started := a.claimQueued(call)
	switch {
	case started:
		<-call.Lifetime.done
	case taken:
		a.publishCanceledQueueDrops([]SessionAgentCall{call})
	}
	return ctx.Err()
}

// claimQueued takes call back out of its session's queue on behalf of a
// cancellation. It reports whether the call was still queued (taken) and
// whether a turn has already claimed it (started); neither is true when
// some other path already dropped and released it.
//
// It holds the per-session dispatch mutex, the same lock the end-of-turn
// handoff dequeues under and the same one that guards
// RunLifetime.started, so exactly one of the two claims the call.
func (a *sessionAgent) claimQueued(call SessionAgentCall) (taken, started bool) {
	mu := a.sessionMu(call.SessionID)
	mu.Lock()
	defer mu.Unlock()
	if call.Lifetime.started {
		return false, true
	}
	queued, ok := a.messageQueue.Get(call.SessionID)
	if !ok {
		return false, false
	}
	kept := slices.DeleteFunc(slices.Clone(queued), func(q SessionAgentCall) bool {
		return q.Lifetime == call.Lifetime
	})
	if len(kept) == len(queued) {
		return false, false
	}
	if len(kept) == 0 {
		a.messageQueue.Del(call.SessionID)
	} else {
		a.messageQueue.Set(call.SessionID, kept)
	}
	return true, false
}

// clearPendingCancel removes any pending-cancel mark for sessionID. It
// takes the per-session dispatch lock so it is ordered against Cancel
// and the dispatch handoff.
func (a *sessionAgent) clearPendingCancel(sessionID string) {
	mu := a.sessionMu(sessionID)
	mu.Lock()
	defer mu.Unlock()
	a.cancelMark.Del(sessionID)
}

// canceledBySeq reports whether an accepted handle or queued call with
// the given accept sequence is covered by a pending cancel for the
// session. Callers must hold the session's dispatch mutex. A tracked
// sequence (seq > 0) is covered only when it is at or below the cancel
// high-water mark, so a prompt accepted after the cancel (higher seq) is
// never poisoned. An untracked sequence (seq == 0, an in-process enqueue
// with no accept reservation) is covered whenever any mark is present,
// preserving the pre-sequence behavior. The mark is not consumed: it
// stays so every sibling handle it covers observes the same cancel, and
// a later handle (higher seq) ignores it regardless.
func (a *sessionAgent) canceledBySeq(sessionID string, seq uint64) bool {
	mark, ok := a.cancelMark.Get(sessionID)
	if !ok || mark == 0 {
		return false
	}
	return seq == 0 || seq <= mark
}

// persistCanceledTurn writes the user/assistant records for a turn that
// was canceled before (or just as) streaming would have produced them.
// It creates the user message only when it was not already created by an
// earlier createUserMessage call (userMsgCreated), then writes an
// assistant message with FinishReasonCanceled. Both writes use
// context.WithoutCancel(ctx) so workspace shutdown (which cancels the run
// context) can't drop them.
func (a *sessionAgent) persistCanceledTurn(ctx context.Context, call SessionAgentCall, userMsgCreated bool) error {
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if !userMsgCreated {
		if _, err := a.createUserMessage(writeCtx, call); err != nil {
			return err
		}
	}
	largeModel, _ := a.turnModels(call)
	assistant, err := a.messages.Create(writeCtx, call.SessionID, message.CreateMessageParams{
		Role:     message.Assistant,
		Parts:    []message.ContentPart{},
		Model:    largeModel.ModelCfg.Model,
		Provider: largeModel.ModelCfg.Provider,
	})
	if err != nil {
		return err
	}
	assistant.AddFinish(message.FinishReasonCanceled, "User canceled request", "")
	return a.messages.Update(writeCtx, assistant)
}

// publishRunComplete emits the authoritative terminal event for a turn.
// It honors the per-call OnComplete hook when set (so the coordinator can
// coalesce retries) and otherwise falls back to the RunComplete broker.
// ctx is used only for the bounded-blocking must-deliver publish; the
// terminal payload is supplied by the caller. This is the single emit path
// shared by the streaming defer and the cancel-on-entry early return so a
// caller waiting on RunComplete (e.g. `crush run` with a RunID) always
// observes exactly one terminal event regardless of which Run branch ends
// the turn.
func (a *sessionAgent) publishRunComplete(ctx context.Context, call SessionAgentCall, complete notify.RunComplete) {
	if call.OnComplete != nil {
		call.OnComplete(complete)
		return
	}
	if a.runComplete == nil {
		return
	}
	a.runComplete.PublishMustDeliver(ctx, pubsub.UpdatedEvent, complete)
}

// ValidateCall performs the cheap structural validation that
// sessionAgent.Run requires before a call can be dispatched: a call must
// carry either a non-empty prompt or a text attachment, and it must name a
// session. It is exported so callers that accept a run before dispatching it
// (e.g. backend.SendMessage) can apply the same checks and keep the error
// contract consistent.
func ValidateCall(call SessionAgentCall) error {
	if call.Prompt == "" && !message.ContainsTextAttachment(call.Attachments) {
		return ErrEmptyPrompt
	}
	if call.SessionID == "" {
		return ErrSessionMissing
	}
	return nil
}

func (a *sessionAgent) Run(ctx context.Context, call SessionAgentCall) (result *fantasy.AgentResult, retErr error) {
	if err := ValidateCall(call); err != nil {
		return nil, err
	}

	// genCtx/cancel are the run context and its cancel func, created under
	// the per-session dispatch mutex below so a concurrent Cancel can observe
	// the activeRequests entry before the assistant message exists.
	var (
		genCtx         context.Context
		cancel         context.CancelFunc
		userMsgCreated bool
	)

	// Serialize the dispatch decision (cancel-on-entry | queued | active)
	// against a concurrent Cancel. Cancel takes the same per-session lock, so
	// every cancel observes at least one of: a cancel mark, an activeRequests
	// entry, or a messageQueue entry it then clears. Holding the lock across
	// the busy check and the active registration also makes them atomic, so
	// two concurrent in-process callers — a burst of channel events, or a
	// channel event racing a typed prompt — cannot both pass the busy check
	// and start two runs on the same session.
	sessMu := a.sessionMu(call.SessionID)
	sessMu.Lock()

	if call.Accepted != nil && a.canceledBySeq(call.SessionID, call.Accepted.seq) {
		// Cancel-on-entry: a cancel arrived while this accepted run was
		// dispatched but not yet active, and this handle's accept sequence
		// is at or below the session's cancel mark. The mark is left in
		// place so sibling handles it also covers observe the same cancel;
		// release the accept reservation, drop the lock, and persist a
		// canceled turn without entering Stream.
		//
		// This path returns before the streaming defer that publishes
		// RunComplete is installed, so emit the terminal event explicitly.
		// Without it, a caller waiting on RunComplete for this RunID (e.g.
		// `crush run`, which ignores message events and blocks on
		// RunComplete) would hang on an immediately-canceled accepted run.
		if call.Lifetime != nil {
			// This call's turn ends here, so release the dispatcher
			// that is waiting for it. Registered as a defer so both
			// exits below are covered, and so it runs after the
			// terminal event has been published.
			defer call.Lifetime.finish()
		}
		call.Accepted.Close()
		sessMu.Unlock()
		complete := notify.RunComplete{
			SessionID: call.SessionID,
			RunID:     call.RunID,
			Cancelled: true,
		}
		if err := a.persistCanceledTurn(ctx, call, false); err != nil {
			complete.Error = err.Error()
			a.publishRunComplete(ctx, call, complete)
			return nil, err
		}
		a.publishRunComplete(ctx, call, complete)
		return nil, nil
	}

	if a.IsSessionBusy(call.SessionID) {
		// Busy: an earlier prompt is active. Queue this call so it is
		// folded into (or sequenced after) the active turn, and release any
		// accept reservation. A Cancel arriving after this point sees the
		// active entry and clears the queue.
		//
		// enqueueCall strips OnComplete: the caller that supplied the hook
		// (typically coordinator.Run) has its own retry/coalesce scope that
		// ends when it returns, so by the time the queue drains nobody is
		// left to consume the buffered terminal event. The queued turn falls
		// back to the default broker publish, which is what existing
		// subscribers expect.
		a.enqueueCall(call)
		if call.Accepted != nil {
			call.Accepted.Close()
		}
		sessMu.Unlock()
		if call.Lifetime == nil {
			return nil, nil
		}
		// A dispatched call waits for the turn it was queued for. Its
		// dispatcher owns everything that makes the run addressable —
		// the registered handle, the cancellation that reaches it, the
		// armed duration bound — and all of that is released the moment
		// the dispatching call returns. See [RunLifetime].
		return nil, a.awaitQueued(ctx, call)
	}

	// Idle: become the active run. Register the cancel func before dropping
	// the lock so a Cancel that arrives between here and assistant creation
	// is not lost.
	//
	// The run's models go on the context too, so a sub-agent started by
	// one of this turn's tool calls inherits the selection of the run
	// that spawned it. Stamping from the call (rather than trusting the
	// incoming ctx) is what keeps the queued-prompt recursion honest: a
	// dequeued prompt is its own run with its own models.
	//
	// The turn's auto-approval decision rides along for the same reason:
	// a sub-agent turn is part of this run, so it takes its own hold on
	// its child session, and a prompt this frame dequeued must not
	// inherit the approval of the turn it queued behind.
	runCtx := context.WithValue(ctx, tools.SessionIDContextKey, call.SessionID)
	runCtx = withTurnAutoApprove(runCtx, call.AutoApprove)
	if call.Models != nil {
		runCtx = withRunModels(runCtx, call.Models)
	}
	genCtx, cancel = context.WithCancel(runCtx)
	ac := &activeCancel{cancel: cancel}
	a.activeRequests.Set(call.SessionID, ac)
	if call.Accepted != nil {
		call.Accepted.Close()
	}
	sessMu.Unlock()

	if call.Lifetime != nil {
		// Released last of all this frame's defers, because the
		// failed-turn handoff below still runs under this call's run
		// context. Deferred rather than released just before the
		// end-of-turn recursion because the auto-compaction
		// continuation is this same call re-prompted and shares this
		// lifetime: releasing early would let the dispatcher retire the
		// run and cancel the context that continuation is streaming on.
		defer call.Lifetime.finish()
	}

	// A turn that ends by error or cancellation still owes the session's
	// queue a handoff. The tail of this function hands off on the
	// success path and sets handedOff; every error return skips it, and
	// once this frame returns the session is idle, so nothing else would
	// ever start the prompts queued behind it. That became reachable per
	// run once a single run could be cancelled on its own: CancelRun,
	// the loss of the requesting client's claim and the maximum run
	// duration all cancel the run context directly instead of going
	// through Cancel, which clears the queue itself.
	//
	// Registered here so it runs after the deferred RunComplete publish
	// and after the activeRequests entry is dropped below: the failing
	// turn's own client must not be made to wait for the queued turn,
	// and the queued prompt's Run must not see the session as still busy
	// and re-queue itself.
	handedOff := false
	defer func() {
		if handedOff {
			return
		}
		a.startQueuedAfterFailedTurn(ctx, call.SessionID)
	}()

	defer cancel()
	// Conditional cleanup: only remove our entry if it hasn't been replaced
	// by a newer run. Without this guard, the deferred Del fires after a
	// concurrent run registers in the completion window, silently wiping
	// the new run's cancel and breaking cancellation.
	defer a.activeRequests.CompareAndDelete(call.SessionID, ac)

	// This call owns the turn now, so take its auto-approval hold here
	// and give it back on every exit path. Callers with nobody to answer
	// a permission prompt (`crush run`) ask for it per prompt; binding
	// the hold to the turn instead of to the caller means an early client
	// exit can neither strand a live turn on an unanswerable request nor
	// leave the session silently approved afterwards. Holds are counted,
	// so concurrent runs on one session do not cancel each other out.
	//
	// The release is idempotent because this frame does not end with the
	// turn: it hands off to other calls' turns (the summarize dequeue and
	// the queued-prompt recursion below), which must not inherit this
	// call's approval. The explicit release right after the model stream
	// keeps them out; the defer covers every path that returns first.
	releaseAutoApprove := func() {}
	if call.AutoApprove && a.permissions != nil {
		a.permissions.AutoApproveSession(call.SessionID)
		var releaseOnce sync.Once
		releaseAutoApprove = func() {
			releaseOnce.Do(func() {
				a.permissions.RevokeAutoApproveSession(call.SessionID)
			})
		}
		defer releaseAutoApprove()
	}

	// Copy mutable fields under lock to avoid races with SetTools/SetModels.
	cacheControl := a.getCacheControlOptions()
	agentTools := a.turnTools(call, cacheControl)
	largeModel, smallModel := a.turnModels(call)
	systemPrompt := a.turnSystemPrompt(call)
	promptPrefix := largeModel.SystemPromptPrefix
	var instructions strings.Builder

	for _, server := range mcp.GetStates() {
		if server.State != mcp.StateConnected {
			continue
		}
		if s := server.Client.InitializeResult().Instructions; s != "" {
			instructions.WriteString(s)
			instructions.WriteString("\n\n")
		}
	}

	if s := instructions.String(); s != "" {
		systemPrompt += "\n\n<mcp-instructions>\n" + s + "\n</mcp-instructions>"
	}

	agent := fantasy.NewAgent(
		largeModel.Model,
		fantasy.WithSystemPrompt(systemPrompt),
		fantasy.WithTools(agentTools...),
		fantasy.WithUserAgent(userAgent),
	)

	sessionLock := sync.Mutex{}
	currentSession, err := a.sessions.Get(ctx, call.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	msgs, err := a.getSessionMessages(ctx, currentSession)
	if err != nil {
		return nil, fmt.Errorf("failed to get session messages: %w", err)
	}

	// Generate title from the first real (non-shell) user prompt.
	// can take tens of seconds. Blocking Run on it delays the
	// response to the caller. Use a detached context so the title
	// goroutine survives Run's cancel.
	if !hasUserTextMessage(msgs) {
		titleCtx := context.WithoutCancel(ctx)
		go a.generateTitle(titleCtx, call.SessionID, call.Prompt, largeModel, smallModel)
	}

	// Add the user message to the session.
	_, err = a.createUserMessage(ctx, call)
	if err != nil {
		return nil, err
	}
	userMsgCreated = true

	// Add the session to the context. The run context (genCtx) and its
	// cancel func were already created and registered under the dispatch
	// mutex above for both the accepted and in-process paths.
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, call.SessionID)
	// skipRunComplete is set just before the queued-recursion path so
	// the outer Run doesn't publish a RunComplete that would race
	// with — and be superseded by — the recursive call's own
	// RunComplete (each queued user prompt is its own turn and
	// publishes exactly one terminal event).
	var skipRunComplete bool
	// currentAssistant is declared here so the deferred RunComplete
	// publish below can capture the pointer that PrepareStep will
	// later (re)assign for each streaming step. The final assistant
	// message of the turn is the value reachable through this
	// pointer when the defer runs.
	var currentAssistant *message.Message
	// turnAssistants collects every assistant message of this turn, in
	// step order, because the end-of-turn tool call repair below has to
	// check all of them: an unanswered call is not necessarily on the
	// last step.
	var turnAssistants []*message.Message
	// Drain any debounced message updates before returning. message.Service
	// already flushes synchronously on terminal updates, but a defer here
	// guarantees the contract at every Run exit (success, error, panic
	// recovery upstream) without callers needing to know.
	//
	// After the flush completes — meaning all per-message
	// Publish(UpdatedEvent) calls have fired and been buffered into
	// every subscriber's channel — publish the authoritative
	// RunComplete event for this turn. The flush-then-publish order
	// gives well-behaved clients the best chance of seeing the final
	// message event before RunComplete; the embedded Text field
	// reconciles for clients that observe the events out of order
	// (the pubsub broker fan-in does not serialize publishes from
	// different upstream brokers).
	defer func() {
		// Use a context detached from the run context: workspace
		// shutdown cancels ctx before this goroutine returns, but the
		// buffered streaming deltas must still land before the DB is
		// closed. A short timeout bounds the flush.
		flushCtx, flushCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer flushCancel()
		if flushErr := a.messages.FlushAll(flushCtx); flushErr != nil {
			slog.Error("Failed to flush pending message updates after run", "error", flushErr)
		}
		if skipRunComplete {
			return
		}
		complete := notify.RunComplete{SessionID: call.SessionID, RunID: call.RunID}
		if currentAssistant != nil {
			complete.MessageID = currentAssistant.ID
			complete.Text = currentAssistant.Content().String()
		}
		if retErr != nil {
			complete.Error = retErr.Error()
			complete.Cancelled = errors.Is(retErr, context.Canceled)
		} else if ctx.Err() != nil {
			complete.Cancelled = true
		}
		// Prefer the per-call hook when supplied so the coordinator
		// can coalesce retries (e.g. unauthorized → re-auth → retry)
		// into a single user-visible terminal event. The fallback
		// must-deliver publish applies bounded-blocking semantics to
		// the authoritative terminal event so a momentarily-full
		// subscriber channel can't silently drop it and hang
		// non-interactive clients waiting on RunComplete.
		a.publishRunComplete(ctx, call, complete)
	}()

	history, files := a.preparePrompt(msgs, largeModel.CatwalkCfg.SupportsImages, call.Attachments...)

	startTime := time.Now()
	a.eventPromptSent(call.SessionID)

	var stepMessages []fantasy.Message
	var shouldSummarize bool
	sanitizedToolCalls := make(map[string]bool)
	// Don't send MaxOutputTokens if 0 — some providers (e.g. LM Studio) reject it
	var maxOutputTokens *int64
	if call.MaxOutputTokens > 0 {
		maxOutputTokens = &call.MaxOutputTokens
	}
	result, err = agent.Stream(genCtx, fantasy.AgentStreamCall{
		Prompt:           message.PromptWithTextAttachments(call.Prompt, call.Attachments),
		Files:            files,
		Messages:         history,
		Headers:          sessionHeaders(call.SessionID),
		ProviderOptions:  call.ProviderOptions,
		MaxOutputTokens:  maxOutputTokens,
		TopP:             call.TopP,
		Temperature:      call.Temperature,
		PresencePenalty:  call.PresencePenalty,
		TopK:             call.TopK,
		FrequencyPenalty: call.FrequencyPenalty,
		PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = options.Messages
			for i := range prepared.Messages {
				prepared.Messages[i].ProviderOptions = nil
			}

			// Use latest tools (updated by SetTools when MCP tools change).
			prepared.Tools = a.turnTools(call, cacheControl)

			// Drain queued follow-up prompts for this step. Calls covered
			// by a cancel recorded while they sat in the queue are dropped:
			// a cancel that arrived after a prompt was queued must not let
			// it run as part of this step. Coverage is per-call by accept
			// sequence so a follow-up queued after the cancel (higher seq)
			// is not dropped. A dropped prompt carrying a RunID still gets
			// its terminal cancelled RunComplete so a caller waiting on it
			// does not hang. Uncanceled prompts without a RunID are folded
			// into this turn; uncanceled prompts with a RunID are left
			// queued so each runs as its own turn (with its own
			// RunComplete) via the recursive run path below.
			fold, dropped := a.drainQueueForStep(call.SessionID)
			a.publishCanceledQueueDrops(dropped)
			// A folded prompt is answered by this turn, so its own run
			// ends here: release its dispatcher before the message
			// writes below, which can fail and return.
			finishRunLifetimes(fold)
			for _, queued := range fold {
				userMessage, createErr := a.createUserMessage(callContext, queued)
				if createErr != nil {
					return callContext, prepared, createErr
				}
				prepared.Messages = append(prepared.Messages, userMessage.ToAIMessage()...)
			}

			prepared.Messages = a.workaroundProviderMediaLimitations(prepared.Messages, largeModel)

			lastSystemRoleInx := 0
			systemMessageUpdated := false
			for i, msg := range prepared.Messages {
				// Only add cache control to the last message.
				if msg.Role == fantasy.MessageRoleSystem {
					lastSystemRoleInx = i
				} else if !systemMessageUpdated {
					prepared.Messages[lastSystemRoleInx].ProviderOptions = a.getCacheControlOptions()
					systemMessageUpdated = true
				}
				// Than add cache control to the last 2 messages.
				if i > len(prepared.Messages)-3 {
					prepared.Messages[i].ProviderOptions = a.getCacheControlOptions()
				}
			}

			if promptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(promptPrefix)}, prepared.Messages...)
			}

			sessionLock.Lock()
			stepMessages = cloneFantasyMessages(prepared.Messages)
			sessionLock.Unlock()

			var assistantMsg message.Message
			assistantMsg, err = a.messages.Create(callContext, call.SessionID, message.CreateMessageParams{
				Role:     message.Assistant,
				Parts:    []message.ContentPart{},
				Model:    largeModel.ModelCfg.Model,
				Provider: largeModel.ModelCfg.Provider,
			})
			if err != nil {
				return callContext, prepared, err
			}
			callContext = context.WithValue(callContext, tools.MessageIDContextKey, assistantMsg.ID)
			callContext = context.WithValue(callContext, tools.SupportsImagesContextKey, largeModel.CatwalkCfg.SupportsImages)
			callContext = context.WithValue(callContext, tools.ModelNameContextKey, largeModel.CatwalkCfg.Name)
			currentAssistant = &assistantMsg
			turnAssistants = append(turnAssistants, currentAssistant)
			return callContext, prepared, err
		},
		OnReasoningStart: func(id string, reasoning fantasy.ReasoningContent) error {
			currentAssistant.AppendReasoningContent(reasoning.Text)
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnReasoningDelta: func(id string, text string) error {
			currentAssistant.AppendReasoningContent(text)
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnReasoningEnd: func(id string, reasoning fantasy.ReasoningContent) error {
			// handle anthropic signature
			if anthropicData, ok := reasoning.ProviderMetadata[anthropic.Name]; ok {
				if reasoning, ok := anthropicData.(*anthropic.ReasoningOptionMetadata); ok {
					currentAssistant.AppendReasoningSignature(reasoning.Signature)
				}
			}
			if googleData, ok := reasoning.ProviderMetadata[google.Name]; ok {
				if reasoning, ok := googleData.(*google.ReasoningMetadata); ok {
					currentAssistant.AppendThoughtSignature(reasoning.Signature, reasoning.ToolID)
				}
			}
			if openaiData, ok := reasoning.ProviderMetadata[openai.Name]; ok {
				if reasoning, ok := openaiData.(*openai.ResponsesReasoningMetadata); ok {
					currentAssistant.SetReasoningResponsesData(reasoning)
				}
			}
			currentAssistant.FinishThinking()
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnTextDelta: func(id string, text string) error {
			// Strip leading newline from initial text content. This is is
			// particularly important in non-interactive mode where leading
			// newlines are very visible.
			if len(currentAssistant.Parts) == 0 {
				text = strings.TrimPrefix(text, "\n")
			}

			currentAssistant.AppendContent(text)
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnToolInputStart: func(id string, toolName string) error {
			toolCall := message.ToolCall{
				ID:               id,
				Name:             toolName,
				ProviderExecuted: false,
				Finished:         false,
			}
			currentAssistant.AddToolCall(toolCall)
			// Use parent ctx instead of genCtx to ensure the update succeeds
			// even if the request is canceled mid-stream
			return a.messages.Update(ctx, *currentAssistant)
		},
		OnRetry: func(err *fantasy.ProviderError, delay time.Duration) {
			slog.Warn("Provider request failed, retrying", providerRetryLogFields(err, delay)...)
			// Reset streamed content so the retried response doesn't
			// concatenate with partial content from the failed attempt.
			// On the final attempt (no more retries), any partial content
			// stays in the message as useful context beneath the error.
			currentAssistant.ResetStreamedContent()
			if updateErr := a.messages.Update(genCtx, *currentAssistant); updateErr != nil {
				slog.Error("Failed to reset message on retry", "error", updateErr)
			}
		},
		OnAuthRefresh: call.OnAuthRefresh,
		// Called on every step and on every retry attempt, so it must
		// read the run's own pair: the shared agent's model can change
		// under a live turn, and a refresh rebuilds this run's pair in
		// place so the retry gets fresh credentials.
		ModelProvider: func() fantasy.LanguageModel {
			m := largeModel
			if call.Models != nil {
				m = call.Models.Large()
			}
			slog.Info("ModelProvider called",
				"provider", m.ModelCfg.Provider,
				"model", m.ModelCfg.Model)
			return m.Model
		},
		OnToolCall: func(tc fantasy.ToolCallContent) error {
			input, wasSanitized := sanitizeToolInput(tc.ToolName, tc.ToolCallID, tc.Input)
			if wasSanitized {
				sanitizedToolCalls[tc.ToolCallID] = true
			}
			toolCall := message.ToolCall{
				ID:               tc.ToolCallID,
				Name:             tc.ToolName,
				Input:            input,
				ProviderExecuted: false,
				Finished:         true,
			}
			currentAssistant.AddToolCall(toolCall)
			// Use parent ctx instead of genCtx to ensure the update succeeds
			// even if the request is canceled mid-stream
			return a.messages.Update(ctx, *currentAssistant)
		},
		OnToolResult: func(result fantasy.ToolResultContent) error {
			toolResult := a.convertToToolResult(result)
			if sanitizedToolCalls[result.ToolCallID] {
				toolResult.Content = "Tool call failed: arguments were not valid JSON. Please check your tool call format and try again."
				toolResult.IsError = true
			}
			// Use parent ctx instead of genCtx to ensure the message is created
			// even if the request is canceled mid-stream
			_, createMsgErr := a.messages.Create(ctx, currentAssistant.SessionID, message.CreateMessageParams{
				Role: message.Tool,
				Parts: []message.ContentPart{
					toolResult,
				},
			})
			return createMsgErr
		},
		OnStepFinish: func(stepResult fantasy.StepResult) error {
			for _, w := range stepResult.Warnings {
				slog.Warn("Provider warning", "type", w.Type, "message", w.Message)
			}
			finishReason := message.FinishReasonUnknown
			switch stepResult.FinishReason {
			case fantasy.FinishReasonLength:
				finishReason = message.FinishReasonMaxTokens
			case fantasy.FinishReasonStop:
				finishReason = message.FinishReasonEndTurn
			case fantasy.FinishReasonToolCalls:
				finishReason = message.FinishReasonToolUse
			case fantasy.FinishReasonContentFilter:
				// Provider safety classifier stopped the model
				// (Anthropic stop_reason=refusal, OpenAI content_filter).
				// The TUI owns the display copy; we only persist the
				// reason so the UI can show a REFUSED banner.
				finishReason = message.FinishReasonContentFilter
				slog.Warn(
					"Provider content filter stopped the model",
					"session_id", call.SessionID,
					"finish_reason", string(stepResult.FinishReason),
				)
			}
			// If a tool result halted the turn (e.g. a hook halt or a
			// permission denial), the step ends on FinishReasonToolCalls but
			// the model will not be called again. Treat it as the end of the
			// turn so the UI can render the assistant footer.
			if finishReason == message.FinishReasonToolUse {
				for _, tr := range stepResult.Content.ToolResults() {
					if tr.StopTurn {
						finishReason = message.FinishReasonEndTurn
						break
					}
				}
			}
			currentAssistant.AddFinish(finishReason, "", "")
			sessionLock.Lock()
			defer sessionLock.Unlock()

			updatedSession, getSessionErr := a.sessions.Get(ctx, call.SessionID)
			if getSessionErr != nil {
				return getSessionErr
			}
			usage, estimated := fallbackStepUsage(stepMessages, stepResult)
			a.updateSessionUsage(largeModel, &updatedSession, usage, a.openrouterCost(stepResult.ProviderMetadata), estimated)
			extractHyperCredits(stepResult.ProviderMetadata)
			_, sessionErr := a.sessions.Save(ctx, updatedSession)
			if sessionErr != nil {
				return sessionErr
			}
			currentSession = updatedSession
			return a.messages.Update(genCtx, *currentAssistant)
		},
		StopWhen: []fantasy.StopCondition{
			func(_ []fantasy.StepResult) bool {
				cw := int64(largeModel.CatwalkCfg.ContextWindow)
				// If context window is unknown (0), skip auto-summarize
				// to avoid immediately truncating custom/local models.
				if cw == 0 {
					return false
				}
				tokens := currentSession.CompletionTokens + currentSession.PromptTokens
				remaining := cw - tokens
				var threshold int64
				if cw > largeContextWindowThreshold {
					threshold = largeContextWindowBuffer
				} else {
					threshold = int64(float64(cw) * smallContextWindowRatio)
				}
				if (remaining <= threshold) && !a.disableAutoSummarize {
					shouldSummarize = true
					return true
				}
				return false
			},
			func(steps []fantasy.StepResult) bool {
				return hasRepeatedToolCalls(steps, loopDetectionWindowSize, loopDetectionMaxRepeats)
			},
		},
	})

	a.eventPromptResponded(call.SessionID, time.Since(startTime).Truncate(time.Second))

	// Tool dispatch happens entirely inside agent.Stream, so the turn's
	// auto-approval hold has done its job by here. Give it back before
	// the rest of this frame, which can start another call's turn: the
	// summarize dequeue and the queued-prompt recursion below both run
	// Run again in place, and a prompt that did not ask for approval
	// (an interactive one, say) must still get a permission prompt.
	releaseAutoApprove()

	if err != nil {
		isHyper := largeModel.ModelCfg.Provider == hyper.Name
		isCancelErr := errors.Is(err, context.Canceled)
		slog.Info("Agent stream returned error",
			"error", err.Error(),
			"error_type", fmt.Sprintf("%T", err),
			"is_hyper", isHyper,
			"is_cancel", isCancelErr)
		if currentAssistant == nil {
			// Cancel-before-assistant-creation window: the run was
			// canceled after activeRequests.Set but before PrepareStep
			// created the assistant message. Without this, the turn
			// would return with no FinishReasonCanceled marker and no
			// user-visible record. The user message was already created
			// above, so persistCanceledTurn only writes the assistant
			// record.
			if isCancelErr {
				if persistErr := a.persistCanceledTurn(ctx, call, userMsgCreated); persistErr != nil {
					return nil, persistErr
				}
			}
			return result, err
		}
		// Persist final state with a context detached from the run
		// context. The run context (ctx) is derived from the
		// workspace context, which workspace shutdown cancels before
		// agent goroutines finish; using ctx here would drop the
		// final assistant state. WithoutCancel keeps the values
		// (e.g. session ID) while ignoring cancellation, and a short
		// timeout bounds the cleanup writes.
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cleanupCancel()
		// Ensure we finish thinking on error to close the reasoning state.
		currentAssistant.FinishThinking()
		// INFO: we use the cleanup context here because the genCtx has
		// been cancelled.
		content := "There was an error while executing the tool"
		if isCancelErr {
			content = "Error: user cancelled assistant tool calling"
		}
		sameContent := func(*message.Message) string { return content }
		if _, finalizeErr := finalizeUnresolvedToolCalls(cleanupCtx, a.messages, turnAssistants, sameContent); finalizeErr != nil {
			return nil, finalizeErr
		}
		var fantasyErr *fantasy.Error
		var providerErr *fantasy.ProviderError
		const defaultTitle = "Provider Error"
		linkStyle := lipgloss.NewStyle().Foreground(charmtone.Guac).Underline(true)
		if isCancelErr {
			currentAssistant.AddFinish(message.FinishReasonCanceled, "User canceled request", "")
		} else if isHyper && errors.As(err, &providerErr) && providerErr.StatusCode == http.StatusUnauthorized {
			currentAssistant.AddFinish(message.FinishReasonError, "Unauthorized", `Please re-authenticate with Hyper. You can also run "crush auth" to re-authenticate.`)
		} else if errors.As(err, &providerErr) {
			if providerErr.Message == "The requested model is not supported." {
				url := "https://github.com/settings/copilot/features"
				link := linkStyle.Hyperlink(url, "id=copilot").Render(url)
				currentAssistant.AddFinish(
					message.FinishReasonError,
					"Copilot model not enabled",
					fmt.Sprintf("%q is not enabled in Copilot. Go to the following page to enable it. Then, wait 5 minutes before trying again. %s", largeModel.CatwalkCfg.Name, link),
				)
			} else {
				currentAssistant.AddFinish(message.FinishReasonError, cmp.Or(stringext.Capitalize(providerErr.Title), defaultTitle), providerErr.Message)
			}
		} else if errors.As(err, &fantasyErr) {
			currentAssistant.AddFinish(message.FinishReasonError, cmp.Or(stringext.Capitalize(fantasyErr.Title), defaultTitle), fantasyErr.Message)
		} else if fantasy.IsTransportError(err) {
			wrapped := fantasy.NewTransportError(err)
			currentAssistant.AddFinish(message.FinishReasonError, stringext.Capitalize(wrapped.Title), wrapped.Message)
		} else {
			currentAssistant.AddFinish(message.FinishReasonError, defaultTitle, err.Error())
		}
		// Note: we use the cleanup context here because the genCtx has been
		// cancelled.
		updateErr := a.messages.Update(cleanupCtx, *currentAssistant)
		if updateErr != nil {
			return nil, updateErr
		}
		return nil, err
	}

	// A step that ends truncated (finish_reason=length) never dispatches
	// its buffered tool calls, and Stream still returns no error, so the
	// tool calls already persisted by OnToolCall can never get a result.
	// The same stored state is reachable whenever a result row fails to
	// write, because fantasy discards the error OnToolResult returns.
	// Detect the unanswered calls rather than the finish reason, across
	// every step of the turn: repair the transcript, then fail the turn.
	// Reporting success would tell the caller the tools ran and leave
	// every client rendering them as still running, forever.
	//
	// This sits before the summarize block on purpose, so a truncated
	// turn is not re-queued as a continuation of tool calls that never
	// ran. The failure is recorded in turnErr rather than returned,
	// because this turn still owes the end-of-turn handoff below: a
	// prompt queued behind it is an independent request with its own
	// RunID and its own client waiting for a terminal event, and
	// returning here would leave it in the queue with nothing left to
	// start it.
	var turnErr error
	if len(turnAssistants) > 0 {
		// Detached and bounded for the same reason as the error path:
		// workspace shutdown can cancel ctx before these writes land.
		orphanCtx, orphanCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		written, finalizeErr := finalizeUnresolvedToolCalls(orphanCtx, a.messages, turnAssistants, unansweredToolCallContent)
		orphanCancel()
		switch {
		case finalizeErr != nil:
			// A half-repaired transcript is still a failed turn, and it
			// takes the same handoff: a prompt queued behind it must not
			// be stranded by a write that failed for reasons of its own.
			turnErr = finalizeErr
		case written > 0:
			slog.Error("Turn ended with tool calls that have no result",
				"session_id", call.SessionID,
				"count", written)
			// Only a truncated final step proves the calls never ran.
			// Anything else got here through a lost result row, where
			// the tool did run.
			turnErr = ErrToolResultsMissing
			if finish := currentAssistant.FinishPart(); finish != nil && finish.Reason == message.FinishReasonMaxTokens {
				turnErr = ErrToolCallsNotRun
			}
		}
	}

	// continuation is the auto-compaction resume: this same call,
	// re-prompted, handed to the end-of-turn handoff below so the turn
	// picks its work back up after the summary. The handoff decides
	// whether it actually runs — a prompt the user queued meanwhile
	// supersedes it, and a pending cancel drops it.
	var continuation *SessionAgentCall
	if turnErr == nil && shouldSummarize {
		// Auto-compaction is part of this turn, so it runs under this
		// frame's ownership: the activeRequests entry stays installed
		// and summarizeSession does no bookkeeping of its own. Dropping
		// the entry here — which is what made the exported Summarize's
		// busy check pass — left the session unowned across the
		// summarize's DB reads, so IsSessionBusy reported an idle
		// session while this turn was still live, a concurrent dispatch
		// could start a second run on it, and Cancel reached the
		// summarize's context instead of this run's.
		summarizeCall := SummarizeCall{
			SessionID:       call.SessionID,
			ProviderOptions: call.ProviderOptions,
			Models:          call.Models,
			OnAuthRefresh:   call.OnAuthRefresh,
		}
		switch summarizeErr := a.summarizeSession(ctx, genCtx, summarizeCall); {
		case summarizeErr != nil:
			// Recorded rather than returned, for the same reason as the
			// tool-repair failure above: this turn still owes the
			// end-of-turn handoff, and a prompt queued behind it is an
			// independent request whose client waits for a terminal
			// event under its own RunID. Returning here would leave it
			// in the queue with nothing left to start it.
			turnErr = summarizeErr
		case len(currentAssistant.ToolCalls()) > 0:
			// The model was still working when the context window
			// filled, so the turn resumes after the summary.
			resume := call
			resume.Prompt = fmt.Sprintf("The previous session was interrupted because it got too long, the initial user request was: `%s`", call.Prompt)
			continuation = &resume
		}
	}

	// Release active request before publishing the notification.
	// TUI handlers poll IsSessionBusy() and only re-evaluate when a
	// tea.Msg arrives, so the cleanup must precede the notify or
	// subscribers see stale busy state at the moment of receipt.
	//
	// Own-entry-only: a plain Del would remove whatever entry is
	// registered, so a frame that installed its own would be left live
	// but invisible to IsSessionBusy and beyond Cancel's reach.
	a.activeRequests.CompareAndDelete(call.SessionID, ac)
	cancel()

	// Send notification that agent has finished its turn (skip for
	// sub-agent sessions, whose parent turn is still running and owns
	// that signal, and for a failed turn: the error paths above return
	// before this point, so a turn that ends in turnErr must not claim
	// it finished either). A non-interactive turn does publish it: the
	// event is the TUI's busy->idle edge, and an attached TUI has to see
	// a `crush run` turn on this session end.
	if turnErr == nil && !call.SubAgent && a.notify != nil {
		a.notify.Publish(pubsub.CreatedEvent, notify.Notification{
			SessionID:    call.SessionID,
			SessionTitle: currentSession.Title,
			Type:         notify.TypeAgentFinished,
		})
	}

	// Hand off to the next queued prompt, if any. The shared helper does
	// it under dispatchMu so the transition from this finished run to
	// the queued one is atomic against a concurrent Cancel:
	// activeRequests for this session was just deleted above, so without
	// the lock there is a window in which the session looks idle and a
	// cancel becomes a no-op that fails to stop the queued prompt.
	handedOff = true
	handoff := a.handOffQueue(call.SessionID, call.RunID, continuation, false)
	if !handoff.Started {
		if turnErr != nil {
			return nil, turnErr
		}
		return result, err
	}
	// There are queued messages, restart the loop. Suppress the outer
	// defer's emit: it would otherwise observe the recursive Run's retErr
	// (named-return clobbering through the return below) against this
	// turn's MessageID/Text and publish a mixed, racing event.
	skipRunComplete = true
	if handoff.OwesRunComplete {
		complete := notify.RunComplete{SessionID: call.SessionID, RunID: call.RunID}
		if currentAssistant != nil {
			complete.MessageID = currentAssistant.ID
			complete.Text = currentAssistant.Content().String()
		}
		if turnErr != nil {
			complete.Error = turnErr.Error()
		}
		if ctx.Err() != nil {
			complete.Cancelled = true
		}
		a.publishRunComplete(ctx, call, complete)
	}
	if turnErr != nil {
		// This turn failed, but the prompt behind it is an independent
		// request that has not run yet: start it, then report this
		// turn's own failure. The queued turn owns its own outcome and
		// publishes it under its own RunID, so its error is not this
		// call's to return.
		_, _ = a.Run(runContextFor(ctx, handoff.Next), handoff.Next)
		return nil, turnErr
	}
	return a.Run(runContextFor(ctx, handoff.Next), handoff.Next)
}

// Summarize replaces the session's history with a summary. It owns the
// session for the whole call.
//
// The busy check and the registration happen together under the
// per-session dispatch mutex, the same lock Run's dispatch decision
// uses, so a Run and a Summarize can never both believe they own one
// session. Without the lock the three DB round trips inside
// summarizeSession sat between the two, a Run dispatched in that window
// passed its own busy check, and whichever registered second silently
// overwrote the other's *activeCancel: the overwritten frame could no
// longer be cancelled, its deferred CompareAndDelete no-oped, and
// IsSessionBusy reported an idle session that was still streaming.
func (a *sessionAgent) Summarize(ctx context.Context, call SummarizeCall) error {
	sessionID := call.SessionID

	sessMu := a.sessionMu(sessionID)
	sessMu.Lock()
	if a.IsSessionBusy(sessionID) {
		sessMu.Unlock()
		return ErrSessionBusy
	}
	genCtx, cancel := context.WithCancel(ctx)
	ac := &activeCancel{cancel: cancel}
	a.activeRequests.Set(sessionID, ac)
	sessMu.Unlock()

	defer a.activeRequests.CompareAndDelete(sessionID, ac)
	defer cancel()

	sumErr := a.summarizeSession(ctx, genCtx, call)

	// Release the session before handing off to the queue so the
	// dequeued prompt's Run does not see it as busy and re-queue itself,
	// which would leave it with nothing left to start it. Own-entry-only:
	// a plain Del here would remove whatever entry is registered,
	// including one another frame installed, leaving a live run invisible
	// to IsSessionBusy and beyond Cancel's reach.
	a.activeRequests.CompareAndDelete(sessionID, ac)
	cancel()

	// Hand off through the same guarded helper Run uses. The handoff this
	// replaced was an unlocked messageQueue.Get/Set pair: it raced
	// Cancel, ClearQueue, enqueueCall and drainQueueForStep, so a prompt
	// enqueued in the window could be dropped with no terminal event
	// (blocking its caller on a RunID forever) or run twice, it ignored
	// the session's cancel mark, so a prompt the user had cancelled ran
	// anyway, and no accept reservation covered the dequeue.
	//
	// A failed summarize hands off too: each queued prompt is an
	// independent request whose client waits for a terminal event under
	// its own RunID, and returning here would strand it in the queue on
	// an idle session with nothing left to start it.
	handoff := a.handOffQueue(sessionID, "", nil, false)
	if !handoff.Started {
		return sumErr
	}
	_, runErr := a.Run(runContextFor(ctx, handoff.Next), handoff.Next)
	if sumErr != nil {
		// This call's own outcome. The queued turn owns and publishes
		// its own under its RunID.
		return sumErr
	}
	return runErr
}

// startQueuedAfterFailedTurn hands the session over to the next
// dispatched prompt queued behind a turn that ended by error or
// cancellation, in place of the end-of-turn handoff that error return
// skipped. Without it those prompts sat in the queue of a session that
// is now idle, with nothing left to start them and a client blocked on
// each one's RunID.
//
// The dequeued prompt runs under its own run context, not the failed
// turn's, so it neither inherits that cancellation nor loses its own.
func (a *sessionAgent) startQueuedAfterFailedTurn(ctx context.Context, sessionID string) {
	handoff := a.handOffQueue(sessionID, "", nil, true)
	if !handoff.Started {
		return
	}
	// The queued turn owns and publishes its own outcome under its own
	// RunID, so its error is not this frame's to return.
	_, _ = a.Run(runContextFor(ctx, handoff.Next), handoff.Next)
}

// summarizeSession writes the summary for one session. It does no
// ownership bookkeeping and no queued-prompt handoff: the caller must
// already own the session's activeRequests entry, which is what lets an
// auto-compaction inside a turn run under that turn's ownership.
//
// genCtx is the owner's cancellable run context: the model stream and
// its streaming writes use it, so a Cancel of the owner stops the
// summary too. ctx is the owner's caller context, used for the cleanup
// writes that still have to land after such a cancel.
func (a *sessionAgent) summarizeSession(ctx, genCtx context.Context, call SummarizeCall) error {
	sessionID := call.SessionID

	// Copy mutable fields under lock to avoid races with SetModels.
	largeModel := a.largeModel.Get()
	if call.Models != nil {
		largeModel = call.Models.Large()
	}
	// The prefix belongs to the provider being summarized on, which is
	// the run's model when auto-compaction is part of a turn.
	systemPromptPrefix := largeModel.SystemPromptPrefix

	currentSession, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	msgs, err := a.getSessionMessages(ctx, currentSession)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		// Nothing to summarize.
		return nil
	}

	aiMsgs, _ := a.preparePrompt(msgs, largeModel.CatwalkCfg.SupportsImages)

	defer func() {
		if flushErr := a.messages.FlushAll(ctx); flushErr != nil {
			slog.Error("Failed to flush pending message updates after summarize", "error", flushErr)
		}
	}()

	agent := fantasy.NewAgent(
		largeModel.Model,
		fantasy.WithSystemPrompt(string(summaryPrompt)),
		fantasy.WithUserAgent(userAgent),
	)
	summaryMessage, err := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:             message.Assistant,
		Model:            largeModel.ModelCfg.Model,
		Provider:         largeModel.ModelCfg.Provider,
		IsSummaryMessage: true,
	})
	if err != nil {
		return err
	}

	summaryPromptText := buildSummaryPrompt(currentSession.Todos)

	resp, err := agent.Stream(genCtx, fantasy.AgentStreamCall{
		Prompt:          summaryPromptText,
		Messages:        aiMsgs,
		Headers:         sessionHeaders(sessionID),
		ProviderOptions: call.ProviderOptions,
		OnAuthRefresh:   call.OnAuthRefresh,
		ModelProvider: func() fantasy.LanguageModel {
			if call.Models != nil {
				return call.Models.Large().Model
			}
			return largeModel.Model
		},
		PrepareStep: prefixPrepareStep(systemPromptPrefix),
		OnReasoningDelta: func(id string, text string) error {
			summaryMessage.AppendReasoningContent(text)
			return a.messages.Update(genCtx, summaryMessage)
		},
		OnReasoningEnd: func(id string, reasoning fantasy.ReasoningContent) error {
			// Handle anthropic signature.
			if anthropicData, ok := reasoning.ProviderMetadata["anthropic"]; ok {
				if signature, ok := anthropicData.(*anthropic.ReasoningOptionMetadata); ok && signature.Signature != "" {
					summaryMessage.AppendReasoningSignature(signature.Signature)
				}
			}
			summaryMessage.FinishThinking()
			return a.messages.Update(genCtx, summaryMessage)
		},
		OnTextDelta: func(id, text string) error {
			summaryMessage.AppendContent(text)
			return a.messages.Update(genCtx, summaryMessage)
		},
	})
	if err != nil {
		isCancelErr := errors.Is(err, context.Canceled)
		if isCancelErr {
			// User cancelled summarize we need to remove the summary message.
			deleteErr := a.messages.Delete(ctx, summaryMessage.ID)
			return deleteErr
		}
		// Mark the summary message as finished with an error so the UI
		// stops spinning.
		summaryMessage.AddFinish(message.FinishReasonError, "Summarization Error", err.Error())
		if updateErr := a.messages.Update(ctx, summaryMessage); updateErr != nil {
			return updateErr
		}
		return err
	}

	summaryMessage.AddFinish(message.FinishReasonEndTurn, "", "")
	err = a.messages.Update(genCtx, summaryMessage)
	if err != nil {
		return err
	}

	var openrouterCost *float64
	for _, step := range resp.Steps {
		stepCost := a.openrouterCost(step.ProviderMetadata)
		if stepCost != nil {
			newCost := *stepCost
			if openrouterCost != nil {
				newCost += *openrouterCost
			}
			openrouterCost = &newCost
		}
		extractHyperCredits(step.ProviderMetadata)
	}

	a.updateSessionUsage(largeModel, &currentSession, resp.TotalUsage, openrouterCost, false)

	// Just in case, get just the last usage info.
	usage := resp.Response.Usage
	currentSession.SummaryMessageID = summaryMessage.ID
	currentSession.CompletionTokens = summaryCompletionTokens(usage, summaryMessage)
	currentSession.PromptTokens = 0
	currentSession.EstimatedUsage = usageIsZero(usage)
	_, err = a.sessions.Save(genCtx, currentSession)
	return err
}

func (a *sessionAgent) getCacheControlOptions() fantasy.ProviderOptions {
	if t, _ := strconv.ParseBool(os.Getenv("CRUSH_DISABLE_ANTHROPIC_CACHE")); t {
		return fantasy.ProviderOptions{}
	}
	return fantasy.ProviderOptions{
		anthropic.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
		bedrock.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
		vercel.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
	}
}

// toolProviderOptions overrides one tool's provider options for a single
// request without touching the tool it wraps.
//
// The agent's tool instances are shared by every run on this agent, and
// a server runs several sessions against one agent at a time. Calling
// SetProviderOptions on a shared instance therefore wrote a field that
// another run's request build was reading (fantasy reads
// tool.ProviderOptions() for every step) — a data race on any workspace
// with two concurrent runs.
type toolProviderOptions struct {
	fantasy.AgentTool
	opts fantasy.ProviderOptions
}

func (t *toolProviderOptions) ProviderOptions() fantasy.ProviderOptions {
	return t.opts
}

func (t *toolProviderOptions) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.opts = opts
}

// withCacheControl marks the last tool of a request with Anthropic-style
// cache control, so the provider caches the prefix that ends with it. It
// takes ownership of agentTools, which must be a per-request copy.
func withCacheControl(agentTools []fantasy.AgentTool, opts fantasy.ProviderOptions) []fantasy.AgentTool {
	if len(agentTools) == 0 {
		return agentTools
	}
	last := len(agentTools) - 1
	agentTools[last] = &toolProviderOptions{AgentTool: agentTools[last], opts: opts}
	return agentTools
}

// turnTools is the tool palette for one turn: a per-request copy of the
// agent's current tools, with interactive-only tools removed when nobody
// is watching the turn, and the last one marked for cache control.
//
// The palette is filtered per turn rather than per agent because one
// agent serves both kinds of caller at once on the shared server. A
// `crush run` turn that was offered the question tool would block on the
// first call to it until the run was cancelled.
func (a *sessionAgent) turnTools(call SessionAgentCall, cacheControl fantasy.ProviderOptions) []fantasy.AgentTool {
	palette := a.tools.Copy()
	if call.NonInteractive {
		palette = slices.DeleteFunc(palette, func(tool fantasy.AgentTool) bool {
			return tool.Info().Name == tools.QuestionToolName
		})
	}
	return withCacheControl(palette, cacheControl)
}

// sessionHeaders returns the HTTP headers we use for cache affinity on
// every LLM request for a given session.
//
// We use the session hash is used instead of the raw UUID so the header
// value is deterministic and opaque.
func sessionHeaders(sessionID string) map[string]string {
	hash := session.HashID(sessionID)
	return map[string]string{
		"x-session-id":       hash,
		"x-session-affinity": hash,
	}
}

func (a *sessionAgent) createUserMessage(ctx context.Context, call SessionAgentCall) (message.Message, error) {
	parts := []message.ContentPart{message.TextContent{Text: call.Prompt}}
	var attachmentParts []message.ContentPart
	for _, attachment := range call.Attachments {
		attachmentParts = append(attachmentParts, message.BinaryContent{Path: attachment.FilePath, MIMEType: attachment.MimeType, Data: attachment.Content})
	}
	parts = append(parts, attachmentParts...)
	msg, err := a.messages.Create(ctx, call.SessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: parts,
	})
	if err != nil {
		return message.Message{}, fmt.Errorf("failed to create user message: %w", err)
	}
	return msg, nil
}

func (a *sessionAgent) preparePrompt(msgs []message.Message, supportsImages bool, attachments ...message.Attachment) ([]fantasy.Message, []fantasy.FilePart) {
	var history []fantasy.Message
	if !a.isSubAgent {
		history = append(history, fantasy.NewUserMessage(
			fmt.Sprintf(
				"<system_reminder>%s</system_reminder>",
				`This is a reminder that your todo list is currently empty. DO NOT mention this to the user explicitly because they are already aware.
If you are working on tasks that would benefit from a todo list please use the "todos" tool to create one.
If not, please feel free to ignore. Again do not mention this message to the user.`,
			),
		))
	}
	// Index every stored tool result by the ID of the tool call it
	// answers. Provider APIs require a tool result to sit directly
	// behind the message holding its call, and the stored order does
	// not guarantee that: a transcript repaired at the end of a turn
	// appends the missing results after the later assistant messages.
	// Pairing by ID instead of by position keeps the prompt valid
	// wherever a result was stored. The first result stored for a call
	// wins, so a repair row can never displace a real result.
	resultsByToolCallID := make(map[string]message.ToolResult)
	for _, m := range msgs {
		if m.Role != message.Tool {
			continue
		}
		for _, tr := range m.ToolResults() {
			if _, seen := resultsByToolCallID[tr.ToolCallID]; seen {
				continue
			}
			resultsByToolCallID[tr.ToolCallID] = tr
		}
	}
	paired := make(map[string]struct{}, len(resultsByToolCallID))

	for _, m := range msgs {
		if len(m.Parts) == 0 {
			continue
		}
		// Assistant message without content or tool calls (cancelled before it returned anything).
		if m.Role == message.Assistant && len(m.ToolCalls()) == 0 && m.Content().Text == "" && m.ReasoningContent().String() == "" {
			continue
		}
		// Tool messages are never emitted from their stored position:
		// every result is emitted behind its own call below, and one
		// with no matching call is dropped (it would fail API
		// validation on every later turn, locking the session).
		if m.Role == message.Tool {
			continue
		}
		aiMsgs := m.ToAIMessage()
		if !supportsImages {
			for i := range aiMsgs {
				if aiMsgs[i].Role == fantasy.MessageRoleUser {
					aiMsgs[i].Content = filterFileParts(aiMsgs[i].Content)
				}
			}
		}
		history = append(history, aiMsgs...)

		if m.Role == message.Assistant {
			if msg, ok := toolResultsForCalls(m, resultsByToolCallID, paired); ok {
				history = append(history, msg)
			}
		}
	}

	for id := range resultsByToolCallID {
		if _, ok := paired[id]; !ok {
			slog.Warn(
				"Dropping orphaned tool result with no matching tool call",
				"tool_call_id", id,
			)
		}
	}

	var files []fantasy.FilePart
	for _, attachment := range attachments {
		if attachment.IsText() {
			continue
		}
		if !supportsImages {
			continue
		}
		files = append(files, fantasy.FilePart{
			Filename:  attachment.FileName,
			Data:      attachment.Content,
			MediaType: attachment.MimeType,
		})
	}

	return history, files
}

// filterFileParts removes fantasy.FilePart entries from a slice of message
// parts. Used to strip image attachments from historical user messages when
// the current model does not support them.
func filterFileParts(parts []fantasy.MessagePart) []fantasy.MessagePart {
	filtered := make([]fantasy.MessagePart, 0, len(parts))
	for _, part := range parts {
		if _, ok := fantasy.AsMessagePart[fantasy.FilePart](part); ok {
			continue
		}
		filtered = append(filtered, part)
	}
	return filtered
}

// toolResultsForCalls returns the tool message that answers every tool
// call in the given assistant message: the stored result when one
// exists, or a synthetic error result when none does. It records the
// tool call IDs it answered in paired so a stored result is emitted
// exactly once.
//
// LLM APIs require every tool call to be answered directly behind the
// message that made it. An interrupted session leaves calls with no
// result at all, and a session whose transcript was repaired at the end
// of a turn stores the missing results after later messages; either
// shape fails API validation on every subsequent turn and permanently
// locks the conversation.
func toolResultsForCalls(m message.Message, results map[string]message.ToolResult, paired map[string]struct{}) (fantasy.Message, bool) {
	toolCalls := m.ToolCalls()
	if len(toolCalls) == 0 {
		return fantasy.Message{}, false
	}
	parts := make([]message.ContentPart, 0, len(toolCalls))
	for _, tc := range toolCalls {
		result, ok := results[tc.ID]
		if _, alreadyPaired := paired[tc.ID]; ok && !alreadyPaired {
			paired[tc.ID] = struct{}{}
			parts = append(parts, result)
			continue
		}
		slog.Warn(
			"Injecting synthetic tool result for orphaned tool call",
			"tool_call_id", tc.ID,
			"tool_name", tc.Name,
		)
		parts = append(parts, message.ToolResult{
			ToolCallID: tc.ID,
			Name:       tc.Name,
			Content:    "tool call was interrupted and did not produce a result, you may retry this call if the result is still needed",
			IsError:    true,
		})
	}
	toolMsg := message.Message{Role: message.Tool, Parts: parts}
	aiMsgs := toolMsg.ToAIMessage()
	if len(aiMsgs) == 0 {
		return fantasy.Message{}, false
	}
	return aiMsgs[0], true
}

func (a *sessionAgent) getSessionMessages(ctx context.Context, session session.Session) ([]message.Message, error) {
	msgs, err := a.messages.List(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}

	if session.SummaryMessageID != "" {
		summaryMsgIndex := -1
		for i, msg := range msgs {
			if msg.ID == session.SummaryMessageID {
				summaryMsgIndex = i
				break
			}
		}
		if summaryMsgIndex != -1 {
			msgs = msgs[summaryMsgIndex:]
			msgs[0].Role = message.User
		}
	}
	return msgs, nil
}

// hasUserTextMessage reports whether any user message in msgs contains
// text content (as opposed to only shell commands or other non-text parts).
func hasUserTextMessage(msgs []message.Message) bool {
	for _, msg := range msgs {
		if msg.Role != message.User {
			continue
		}
		for _, part := range msg.Parts {
			if tc, ok := part.(message.TextContent); ok && tc.Text != "" {
				return true
			}
		}
	}
	return false
}

// GenerateTitle generates a session title based on the initial prompt,
// using the workspace's current models. Turns that own a model pair go
// through generateTitle so the title comes from the same model the run
// asked for.
func (a *sessionAgent) GenerateTitle(ctx context.Context, sessionID string, userPrompt string) {
	a.generateTitle(ctx, sessionID, userPrompt, a.largeModel.Get(), a.smallModel.Get())
}

// generateTitle names the session with smallModel, falling back to
// largeModel when the small one truncates.
func (a *sessionAgent) generateTitle(ctx context.Context, sessionID string, userPrompt string, largeModel, smallModel Model) {
	if userPrompt == "" {
		return
	}

	// Ensure the session always gets a title even if every path below
	// fails or the context is cancelled before we finish.
	var titleSaved bool
	defer func() {
		if !titleSaved {
			fallbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if err := a.sessions.Rename(fallbackCtx, sessionID, DefaultSessionName); err != nil {
				slog.Error("Failed to save fallback session title", "error", err)
			}
		}
	}()

	newAgent := func(m fantasy.LanguageModel, p []byte, tok int64) fantasy.Agent {
		return fantasy.NewAgent(
			m,
			fantasy.WithSystemPrompt(string(p)+"\n /no_think"),
			fantasy.WithMaxOutputTokens(tok),
			fantasy.WithUserAgent(userAgent),
		)
	}

	streamCall := fantasy.AgentStreamCall{
		Prompt:  fmt.Sprintf("Generate a concise title for the following content:\n\n%s\n <think>\n\n</think>", userPrompt),
		Headers: sessionHeaders(sessionID),
	}

	type modelAttempt struct {
		name  string
		model Model
	}
	attempts := []modelAttempt{
		{"small", smallModel},
		{"large", largeModel},
	}

	var resp *fantasy.AgentResult
	var err error
	var model Model
	var success bool
	for _, attempt := range attempts {
		tok := int64(40)
		if attempt.model.CatwalkCfg.CanReason {
			tok = attempt.model.CatwalkCfg.DefaultMaxTokens
		}
		// The two attempts can be different providers' models, so the
		// prefix has to be the one belonging to the model about to be
		// streamed.
		streamCall.PrepareStep = prefixPrepareStep(attempt.model.SystemPromptPrefix)
		agent := newAgent(attempt.model.Model, titlePrompt, tok)
		resp, err = agent.Stream(ctx, streamCall)
		if err == nil && resp.Response.FinishReason != fantasy.FinishReasonLength {
			model = attempt.model
			slog.Debug("Generated title with " + attempt.name + " model")
			success = true
			break
		}
		if err != nil {
			slog.Error("Error generating title with "+attempt.name+" model; trying next", "err", err)
		} else {
			slog.Error("Title generation hit token limit with " + attempt.name + " model; trying next")
		}
	}
	if !success {
		// The deferred fallback will save the default session name.
		return
	}

	// Clean up title.
	var title string
	title = strings.ReplaceAll(resp.Response.Content.Text(), "\n", " ")

	// Remove thinking tags if present.
	title = thinkTagRegex.ReplaceAllString(title, "")
	title = orphanThinkTagRegex.ReplaceAllString(title, "")

	title = strings.TrimSpace(title)
	if title == "" {
		// LLM returned empty content. Use the prompt itself as a
		// fallback title, truncated to 50 chars, before resorting to
		// the generic default.
		fallback := strings.ReplaceAll(userPrompt, "\n", " ")
		fallback = strings.TrimSpace(fallback)
		if len(fallback) > 50 {
			fallback = ansi.Truncate(fallback, 50, "…")
		}
		title = cmp.Or(fallback, DefaultSessionName)
	}

	// Calculate usage and cost.
	var openrouterCost *float64
	for _, step := range resp.Steps {
		stepCost := a.openrouterCost(step.ProviderMetadata)
		if stepCost != nil {
			newCost := *stepCost
			if openrouterCost != nil {
				newCost += *openrouterCost
			}
			openrouterCost = &newCost
		}
		extractHyperCredits(step.ProviderMetadata)
	}

	modelConfig := model.CatwalkCfg
	cost := modelConfig.CostPer1MInCached/1e6*float64(resp.TotalUsage.CacheCreationTokens) +
		modelConfig.CostPer1MOutCached/1e6*float64(resp.TotalUsage.CacheReadTokens) +
		modelConfig.CostPer1MIn/1e6*float64(resp.TotalUsage.InputTokens) +
		modelConfig.CostPer1MOut/1e6*float64(resp.TotalUsage.OutputTokens)

	// Use override cost if available (e.g., from OpenRouter).
	if openrouterCost != nil {
		cost = *openrouterCost
	}

	// Skip cost accumulation
	if model.FlatRate {
		cost = 0
	}

	promptTokens := resp.TotalUsage.InputTokens + resp.TotalUsage.CacheCreationTokens
	completionTokens := resp.TotalUsage.OutputTokens

	// Atomically update only title and usage fields to avoid overriding other
	// concurrent session updates.
	saveErr := a.sessions.UpdateTitleAndUsage(ctx, sessionID, title, promptTokens, completionTokens, cost)
	if saveErr != nil {
		slog.Error("Failed to save session title and usage", "error", saveErr)
		return
	}
	titleSaved = true
}

func (a *sessionAgent) openrouterCost(metadata fantasy.ProviderMetadata) *float64 {
	openrouterMetadata, ok := metadata[openrouter.Name]
	if !ok {
		return nil
	}

	opts, ok := openrouterMetadata.(*openrouter.ProviderMetadata)
	if !ok {
		return nil
	}
	return &opts.Usage.Cost
}

// extractHyperCredits reads usage.remaining.hypercredits from OpenAI
// provider metadata and stores it for the next FetchCredits call.
func extractHyperCredits(metadata fantasy.ProviderMetadata) {
	openaiMeta, ok := metadata[openai.Name]
	if !ok {
		return
	}
	pm, ok := openaiMeta.(*openai.ProviderMetadata)
	if !ok {
		return
	}
	var remaining struct {
		Hypercredits float64 `json:"hypercredits"`
	}
	if pm.ExtraField("remaining", &remaining) && remaining.Hypercredits > 0 {
		hyper.SetBalance(int(math.Round(remaining.Hypercredits)))
	}
}

func (a *sessionAgent) updateSessionUsage(model Model, session *session.Session, usage fantasy.Usage, overrideCost *float64, estimated bool) {
	if !usageIsZero(usage) {
		session.EstimatedUsage = estimated
	}

	modelConfig := model.CatwalkCfg
	cost := modelConfig.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
		modelConfig.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
		modelConfig.CostPer1MIn/1e6*float64(usage.InputTokens) +
		modelConfig.CostPer1MOut/1e6*float64(usage.OutputTokens)

	if !estimated {
		a.eventTokensUsed(session.ID, model, usage, cost)
	}

	if estimated {
		cost = 0
	} else {
		// Use override cost if available (e.g., from OpenRouter).
		if overrideCost != nil {
			cost = *overrideCost
		}

		// Skip cost accumulation
		if model.FlatRate {
			cost = 0
		}
	}

	session.Cost += cost
	updateSessionTokenCounters(session, usage)
}

func updateSessionTokenCounters(session *session.Session, usage fantasy.Usage) {
	if usage.OutputTokens != 0 {
		session.CompletionTokens = usage.OutputTokens
	}
	if promptTokens := usage.InputTokens + usage.CacheReadTokens; promptTokens != 0 {
		session.PromptTokens = promptTokens
	}
}

func summaryCompletionTokens(usage fantasy.Usage, summaryMessage message.Message) int64 {
	if usage.OutputTokens != 0 {
		return usage.OutputTokens
	}
	return approxTokenCount(summaryMessage.Content().Text) + approxTokenCount(summaryMessage.ReasoningContent().String())
}

func (a *sessionAgent) Cancel(sessionID string) {
	// Serialize against the dispatch handoff in Run so the accepted ->
	// (cancel-on-entry | queued | active) transition is atomic against
	// this cancel. Every cancel observes at least one of: an active
	// request, an accepted run (recorded as a pending cancel), or a
	// queue entry it then clears. If none of those hold, an idle Escape
	// is a true no-op and must not poison the next prompt.
	mu := a.sessionMu(sessionID)
	mu.Lock()
	defer mu.Unlock()

	// Cancel the session's active frame, whether that is a run or a
	// summarize: both register under the plain session ID. Don't use
	// Take() here - we need the entry to remain in activeRequests so
	// IsBusy() returns true until the goroutine fully completes
	// (including error handling that may access the DB). The owning
	// frame's deferred CompareAndDelete will clean the entry up.
	if ac, ok := a.activeRequests.Get(sessionID); ok && ac != nil {
		slog.Debug("Request cancellation initiated", "session_id", sessionID)
		ac.cancel()
	}

	// Record a pending cancel only when a dispatched-but-not-yet-active
	// run exists. This catches runs still in the goroutine scheduler or
	// about to enter Run's busy-queue branch, while leaving an idle
	// session untouched. Active and accepted are not mutually exclusive:
	// when a run is active and a follow-up has been accepted, both the
	// cancel above and this pending record fire.
	//
	// Raise the session's cancel mark to the latest accept sequence
	// assigned so far. Every prompt currently accepted-but-not-yet-
	// active has a sequence at or below that value, so one cancel covers
	// all of them; a prompt accepted after this cancel gets a strictly
	// higher sequence and is never poisoned. Using max keeps repeated
	// cancels idempotent while the same prompts are in flight and lets a
	// later cancel extend coverage to prompts accepted since.
	a.acceptedMu.Lock()
	count, ok := a.acceptedRuns.Get(sessionID)
	mark := a.acceptSeqGen
	a.acceptedMu.Unlock()
	if ok && count > 0 {
		slog.Debug("Recording cancel mark for accepted runs", "session_id", sessionID, "count", count, "mark", mark)
		existing, _ := a.cancelMark.Get(sessionID)
		a.cancelMark.Set(sessionID, max(existing, mark))
	}

	if a.QueuedPrompts(sessionID) > 0 {
		slog.Debug("Clearing queued prompts", "session_id", sessionID)
		a.clearQueueAndNotify(sessionID)
	}
}

func (a *sessionAgent) ClearQueue(sessionID string) {
	if a.QueuedPrompts(sessionID) > 0 {
		slog.Debug("Clearing queued prompts", "session_id", sessionID)
		a.clearQueueAndNotify(sessionID)
	}
}

func (a *sessionAgent) CancelAll() {
	if !a.IsBusy() {
		return
	}
	for key := range a.activeRequests.Seq2() {
		a.Cancel(key) // key is sessionID
	}

	timeout := time.After(5 * time.Second)
	for a.IsBusy() {
		select {
		case <-timeout:
			return
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func (a *sessionAgent) IsBusy() bool {
	var busy bool
	for ac := range a.activeRequests.Seq() {
		if ac != nil {
			busy = true
			break
		}
	}
	return busy
}

func (a *sessionAgent) IsSessionBusy(sessionID string) bool {
	_, busy := a.activeRequests.Get(sessionID)
	return busy
}

func (a *sessionAgent) QueuedPrompts(sessionID string) int {
	l, ok := a.messageQueue.Get(sessionID)
	if !ok {
		return 0
	}
	return len(l)
}

func (a *sessionAgent) QueuedPromptsList(sessionID string) []string {
	l, ok := a.messageQueue.Get(sessionID)
	if !ok {
		return nil
	}
	prompts := make([]string, len(l))
	for i, call := range l {
		prompts[i] = call.Prompt
	}
	return prompts
}

func (a *sessionAgent) SetModels(large Model, small Model) {
	a.largeModel.Set(large)
	a.smallModel.Set(small)
}

func (a *sessionAgent) SetTools(tools []fantasy.AgentTool) {
	a.tools.SetSlice(tools)
}

func (a *sessionAgent) SetSystemPrompt(systemPrompt string) {
	a.systemPrompt.Set(systemPrompt)
}

func (a *sessionAgent) Model() Model {
	return a.largeModel.Get()
}

// turnModels returns the models one turn must use. A call that carries
// its own pair (every production caller does; see
// SessionAgentCall.Models) is pinned to it for the whole turn, so a
// concurrent SetModels cannot change the model the turn is streaming on.
// Callers that supply none get the agent's own models, which is what the
// models passed to NewSessionAgent are for.
func (a *sessionAgent) turnModels(call SessionAgentCall) (large, small Model) {
	if call.Models != nil {
		return call.Models.Large(), call.Models.Small()
	}
	return a.largeModel.Get(), a.smallModel.Get()
}

// turnSystemPrompt returns the system prompt one turn must use. A call
// that brings its own (see SessionAgentCall.SystemPrompt) is pinned to
// it, so a turn streaming a model the shared agent was not built for
// still sends a prompt rendered for that model. Everything else uses
// the agent's own, which is what SetSystemPrompt maintains.
func (a *sessionAgent) turnSystemPrompt(call SessionAgentCall) string {
	if call.SystemPrompt != "" {
		return call.SystemPrompt
	}
	return a.systemPrompt.Get()
}

// prefixPrepareStep returns a PrepareStep that puts a provider's system
// prompt prefix in front of every step's messages. The prefix is passed
// in rather than read from the agent because it belongs to the provider
// of the model being streamed, which differs per turn and, during title
// generation, per attempt.
func prefixPrepareStep(prefix string) fantasy.PrepareStepFunction {
	return func(callCtx context.Context, opts fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
		prepared.Messages = opts.Messages
		if prefix != "" {
			prepared.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(prefix)}, prepared.Messages...)
		}
		return callCtx, prepared, nil
	}
}

// convertToToolResult converts a fantasy tool result to a message tool result.
func (a *sessionAgent) convertToToolResult(result fantasy.ToolResultContent) message.ToolResult {
	baseResult := message.ToolResult{
		ToolCallID: result.ToolCallID,
		Name:       result.ToolName,
		Metadata:   result.ClientMetadata,
	}

	switch result.Result.GetType() {
	case fantasy.ToolResultContentTypeText:
		if r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](result.Result); ok {
			baseResult.Content = r.Text
		}
	case fantasy.ToolResultContentTypeError:
		if r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](result.Result); ok {
			baseResult.Content = r.Error.Error()
			baseResult.IsError = true
		}
	case fantasy.ToolResultContentTypeMedia:
		if r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](result.Result); ok {
			if !stringext.IsValidBase64(r.Data) {
				slog.Warn(
					"Tool returned media with invalid base64 data, discarding image",
					"tool", result.ToolName,
					"tool_call_id", result.ToolCallID,
				)
				baseResult.Content = "Tool returned image data with invalid encoding"
				baseResult.IsError = true
			} else {
				content := r.Text
				if content == "" {
					content = fmt.Sprintf("Loaded %s content", r.MediaType)
				}
				baseResult.Content = content
				baseResult.Data = r.Data
				baseResult.MIMEType = r.MediaType
			}
		}
	}

	return baseResult
}

// workaroundProviderMediaLimitations converts media content in tool results to
// user messages for providers that don't natively support images in tool results.
//
// Problem: OpenAI, Google, OpenRouter, and other OpenAI-compatible providers
// don't support sending images/media in tool result messages - they only accept
// text in tool results. However, they DO support images in user messages.
//
// If we send media in tool results to these providers, the API returns an error.
//
// Solution: For these providers, we:
//  1. Replace the media in the tool result with a text placeholder
//  2. Inject a user message immediately after with the image as a file attachment
//  3. This maintains the tool execution flow while working around API limitations
//
// Anthropic and Bedrock support images natively in tool results, so we skip
// this workaround for them.
//
// Example transformation:
//
//	BEFORE: [tool result: image data]
//	AFTER:  [tool result: "Image loaded - see attached"], [user: image attachment]
func (a *sessionAgent) workaroundProviderMediaLimitations(messages []fantasy.Message, largeModel Model) []fantasy.Message {
	providerSupportsMedia := largeModel.ModelCfg.Provider == string(catwalk.InferenceProviderAnthropic) ||
		largeModel.ModelCfg.Provider == string(catwalk.InferenceProviderBedrock) ||
		largeModel.ModelCfg.Provider == string(catwalk.InferenceProviderBedrockEurope)

	if providerSupportsMedia {
		return messages
	}

	supportsImages := largeModel.CatwalkCfg.SupportsImages

	convertedMessages := make([]fantasy.Message, 0, len(messages))

	for _, msg := range messages {
		if msg.Role != fantasy.MessageRoleTool {
			convertedMessages = append(convertedMessages, msg)
			continue
		}

		textParts := make([]fantasy.MessagePart, 0, len(msg.Content))
		var mediaFiles []fantasy.FilePart

		for _, part := range msg.Content {
			toolResult, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part)
			if !ok {
				textParts = append(textParts, part)
				continue
			}

			if media, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](toolResult.Output); ok {
				if !supportsImages {
					// Model cannot process images. Replace with a text
					// placeholder and skip creating a synthetic user
					// message with FilePart, which would brick the
					// session on text-only models.
					textParts = append(textParts, fantasy.ToolResultPart{
						ToolCallID: toolResult.ToolCallID,
						Output: fantasy.ToolResultOutputContentText{
							Text: "[Image/media content not supported by this model]",
						},
						ProviderOptions: toolResult.ProviderOptions,
					})
					continue
				}

				decoded, err := base64.StdEncoding.DecodeString(media.Data)
				if err != nil {
					slog.Warn("Failed to decode media data", "error", err)
					textParts = append(textParts, part)
					continue
				}

				mediaFiles = append(mediaFiles, fantasy.FilePart{
					Data:      decoded,
					MediaType: media.MediaType,
					Filename:  fmt.Sprintf("tool-result-%s", toolResult.ToolCallID),
				})

				textParts = append(textParts, fantasy.ToolResultPart{
					ToolCallID: toolResult.ToolCallID,
					Output: fantasy.ToolResultOutputContentText{
						Text: "[Image/media content loaded - see attached file]",
					},
					ProviderOptions: toolResult.ProviderOptions,
				})
			} else {
				textParts = append(textParts, part)
			}
		}

		convertedMessages = append(convertedMessages, fantasy.Message{
			Role:    fantasy.MessageRoleTool,
			Content: textParts,
		})

		if len(mediaFiles) > 0 {
			convertedMessages = append(convertedMessages, fantasy.NewUserMessage(
				"Here is the media content from the tool result:",
				mediaFiles...,
			))
		}
	}

	return convertedMessages
}

// buildSummaryPrompt constructs the prompt text for session summarization.
func buildSummaryPrompt(todos []session.Todo) string {
	var sb strings.Builder
	sb.WriteString("Provide a detailed summary of our conversation above.")
	if len(todos) > 0 {
		sb.WriteString("\n\n## Current Todo List\n\n")
		for _, t := range todos {
			fmt.Fprintf(&sb, "- [%s] %s\n", t.Status, t.Content)
		}
		sb.WriteString("\nInclude these tasks and their statuses in your summary. ")
		sb.WriteString("Instruct the resuming assistant to use the `todos` tool to continue tracking progress on these tasks.")
	}
	return sb.String()
}

func providerRetryLogFields(err *fantasy.ProviderError, delay time.Duration) []any {
	fields := []any{
		"retry_delay", delay.String(),
	}
	if err == nil {
		return fields
	}
	fields = append(fields, "status_code", err.StatusCode)
	if err.Title != "" {
		fields = append(fields, "title", err.Title)
	}
	if err.Message != "" {
		fields = append(fields, "message", err.Message)
	}
	return fields
}

// sanitizeToolInput validates tool call JSON from the provider.
// Malformed input is replaced with an empty object to prevent
// stuck conversations from truncated or malformed model output.
// The second return value indicates whether sanitization occurred.
func sanitizeToolInput(toolName, toolCallID, input string) (string, bool) {
	if !json.Valid([]byte(input)) {
		slog.Warn(
			"Malformed tool call JSON from provider, replacing with empty object",
			"tool", toolName,
			"id", toolCallID,
			"input_len", len(input),
		)
		return "{}", true
	}
	return input, false
}
