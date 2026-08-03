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
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/stringext"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
)

// providerMaxRetries is how many times a failed provider request is
// retried before the error is surfaced. Fantasy defaults to 3; Crush
// raises this so transient rate limits and network blips recover more
// often. The wait is cancelable with Escape / Ctrl+C.
//
// This number is bounded, not tuned for maximum persistence. Fantasy's
// backoff is uncapped exponential (5s initial, x2 per attempt, see
// fantasy.DefaultRetryOptions), so the worst-case wall time grows as
// 5s*(2^n-1): n=6 gives a 2m40s longest single wait and 5m15s total,
// while n=10 would give 42m40s and 1h25m15s. A user staring at a
// 42-minute countdown is indistinguishable from a hang, so the budget
// is capped here. TestProviderRetryBudgetIsBounded enforces the bound;
// raise it only alongside a delay cap in fantasy.
const providerMaxRetries = 6

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

//go:embed templates/btw.md
var sideQuestionPrompt []byte

//go:embed templates/plan_mode.md
var planModePrompt string

// Used to remove <think>, DSML, and tool tags from generated outputs.
var (
	thinkTagRegex       = regexp.MustCompile(`(?s)<think>.*?</think>`)
	orphanThinkTagRegex = regexp.MustCompile(`</?think>`)
	dsmlTagRegex        = regexp.MustCompile(`(?s)<\|+DSML\|+.*?>.*?</\|+DSML\|+.*?>`)
	orphanDsmlTagRegex  = regexp.MustCompile(`</?\|+DSML\|+.*?>`)
	toolCallTagRegex    = regexp.MustCompile(`(?s)<(?:tool_call|function_call)>.*?</(?:tool_call|function_call)>`)
	orphanIntroRegex    = regexp.MustCompile(`(?i)^(?:let me check|checking|running).*?:\s*$`)
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
	RunID            string
	Prompt           string
	ProviderOptions  fantasy.ProviderOptions
	Attachments      []message.Attachment
	MaxOutputTokens  int64
	Temperature      *float64
	TopP             *float64
	TopK             *int64
	FrequencyPenalty *float64
	PresencePenalty  *float64
	NonInteractive   bool
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
	// OnAuthRefresh, when non-nil, is called by fantasy when a stream
	// fails with an authentication error (HTTP 401). The callback should
	// refresh credentials and return nil on success, in which case
	// fantasy retries the stream transparently. Returning an error
	// surfaces the original auth error without retry.
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
	Summarize(context.Context, string, fantasy.ProviderOptions, func(context.Context, *fantasy.ProviderError) error) error
	Model() Model
	GenerateTitle(ctx context.Context, sessionID, userPrompt string)
	SideQuestion(ctx context.Context, sessionID, question string, exchanges []SideQuestionExchange) (SideQuestionResult, error)
}

// SideQuestionExchange is one prior question/answer pair from the same
// ephemeral side conversation.
type SideQuestionExchange struct {
	Question string
	Answer   string
}

// SideQuestionResult is the answer to an ephemeral side question.
type SideQuestionResult struct {
	Answer           string
	Model            string
	Provider         string
	PromptTokens     int64
	CompletionTokens int64
}

type Model struct {
	Model      fantasy.LanguageModel
	CatwalkCfg catwalk.Model
	ModelCfg   config.SelectedModel
	FlatRate   bool
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
	largeModel         *csync.Value[Model]
	smallModel         *csync.Value[Model]
	systemPromptPrefix *csync.Value[string]
	systemPrompt       *csync.Value[string]
	tools              *csync.Slice[fantasy.AgentTool]

	isSubAgent           bool
	sessions             session.Service
	messages             message.Service
	disableAutoSummarize bool
	isYolo               bool
	stopHooks            *hooks.Runner
	planModeFn           func() bool
	notify               pubsub.Publisher[notify.Notification]
	runComplete          pubsub.Publisher[notify.RunComplete]

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
	// returned handle, so sequences strictly increase in accept order
	// across the agent. Cancel uses its current value as the per-session
	// high-water mark.
	acceptSeqGen uint64
}

type SessionAgentOptions struct {
	LargeModel           Model
	SmallModel           Model
	SystemPromptPrefix   string
	SystemPrompt         string
	IsSubAgent           bool
	DisableAutoSummarize bool
	IsYolo               bool
	// StopHooks fires the Stop event when a top-level turn ends. Nil for
	// subagents or when no Stop hooks are configured.
	StopHooks *hooks.Runner
	// PlanModeFn reports whether plan mode is active; nil for subagents.
	PlanModeFn  func() bool
	Sessions    session.Service
	Messages    message.Service
	Tools       []fantasy.AgentTool
	Notify      pubsub.Publisher[notify.Notification]
	RunComplete pubsub.Publisher[notify.RunComplete]
}

func NewSessionAgent(
	opts SessionAgentOptions,
) SessionAgent {
	return &sessionAgent{
		largeModel:           csync.NewValue(opts.LargeModel),
		smallModel:           csync.NewValue(opts.SmallModel),
		systemPromptPrefix:   csync.NewValue(opts.SystemPromptPrefix),
		systemPrompt:         csync.NewValue(opts.SystemPrompt),
		isSubAgent:           opts.IsSubAgent,
		sessions:             opts.Sessions,
		messages:             opts.Messages,
		disableAutoSummarize: opts.DisableAutoSummarize,
		tools:                csync.NewSliceFrom(opts.Tools),
		isYolo:               opts.IsYolo,
		stopHooks:            opts.StopHooks,
		planModeFn:           opts.PlanModeFn,
		notify:               opts.Notify,
		runComplete:          opts.RunComplete,
		messageQueue:         csync.NewMap[string, []SessionAgentCall](),
		activeRequests:       csync.NewMap[string, *activeCancel](),
		dispatchMu:           csync.NewMap[string, *sync.Mutex](),
		acceptedRuns:         csync.NewMap[string, int](),
		cancelMark:           csync.NewMap[string, uint64](),
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
	if call.Accepted != nil {
		// Preserve the accept sequence after the handle is stripped so
		// the queue-drain paths can tell a follow-up queued before a
		// cancel (covered by the mark) from one queued after it.
		queued.acceptSeq = call.Accepted.seq
	}
	queued.OnComplete = nil
	queued.Accepted = nil
	existing = append(existing, queued)
	a.messageQueue.Set(call.SessionID, existing)
}

// drainQueueForStep partitions the session's queued calls for the current
// streaming step under the per-session dispatch mutex so the filtering is
// atomic against a concurrent Cancel: canceledBySeq requires the caller to
// hold that mutex, and evaluating it here (rather than after unlocking)
// prevents a cancel recorded between the drain and the check from being
// observed inconsistently.
//
// Calls covered by a pending cancel are dropped; the dropped ones that
// carry a RunID are returned in canceledWithRunID so the caller can
// publish their terminal cancelled RunComplete (a caller waiting on that
// RunID, e.g. `crush run`, would otherwise hang). Uncanceled calls without
// a RunID are returned in fold to be folded into the active turn,
// preserving the existing follow-up behavior. Uncanceled calls that carry
// a RunID are left in the queue so each runs as its own turn via the
// recursive run path and publishes its own RunComplete, giving every
// RunID-bearing prompt an explicit lifecycle instead of being silently
// absorbed into another turn. fold is processed by the caller without the
// lock held.
func (a *sessionAgent) drainQueueForStep(sessionID string) (fold, canceledWithRunID []SessionAgentCall) {
	dispatchLock := a.sessionMu(sessionID)
	dispatchLock.Lock()
	defer dispatchLock.Unlock()
	queuedCalls, _ := a.messageQueue.Get(sessionID)
	var keep []SessionAgentCall
	for _, queued := range queuedCalls {
		if a.canceledBySeq(sessionID, queued.acceptSeq) {
			if queued.RunID != "" {
				canceledWithRunID = append(canceledWithRunID, queued)
			}
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
	return fold, canceledWithRunID
}

// publishCanceledQueueDrops emits a terminal cancelled RunComplete for
// every dropped queued call that carries a RunID. A queued prompt removed
// from the queue without ever running — covered by a pending cancel, or
// cleared by Cancel/ClearQueue — would otherwise leave a caller blocked on
// that RunID: `crush run` ignores live message events and exits only on a
// RunComplete whose RunID matches. Calls without a RunID had no such waiter
// and are dropped silently as before. A detached, bounded context keeps the
// must-deliver publish alive even when the run context that triggered the
// drop is already canceled.
func (a *sessionAgent) publishCanceledQueueDrops(drops []SessionAgentCall) {
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
		// deliverRunComplete, not publishRunComplete: these prompts never
		// ran, so no Stop event is owed. Cancel holds the per-session
		// mutex across this loop.
		a.deliverRunComplete(ctx, d, notify.RunComplete{
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
	largeModel := a.largeModel.Get()
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
	a.fireStopHook(ctx, call, complete)
	a.deliverRunComplete(ctx, call, complete)
}

// stopHookTeardownGrace caps a Stop hook when the run context is already
// dead — the user pressed Escape or the app is shutting down. Stop is
// observational (its AggregateResult is discarded), so it must never pin
// a session as busy for the hook's full configured timeout while the user
// is trying to get out. On a clean turn end the configured timeout still
// applies in full.
const stopHookTeardownGrace = 2 * time.Second

// fireStopHook runs the Stop event for a turn that actually executed.
//
// Known edge: on the coordinator's re-auth retry path every attempt
// publishes through publishRunComplete with an OnComplete coalescer, so
// Stop hooks can fire once per attempt (at most twice, and only on a
// 401-refresh retry). Accepted for v1 — moving the fire after the
// OnComplete branch would skip Stop entirely for interactive runs.
func (a *sessionAgent) fireStopHook(ctx context.Context, call SessionAgentCall, complete notify.RunComplete) {
	if a.stopHooks == nil {
		return
	}
	outcome := "complete"
	switch {
	case complete.Cancelled:
		outcome = "cancelled"
	case complete.Error != "":
		outcome = "error"
	}
	// Detach from the run context so a cancelled turn still reports Stop,
	// but keep a short leash when the caller is already tearing down.
	hookCtx := context.WithoutCancel(ctx)
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		hookCtx, cancel = context.WithTimeout(hookCtx, stopHookTeardownGrace)
		defer cancel()
	}
	if _, err := a.stopHooks.RunEvent(hookCtx, hooks.EventInput{
		Event:     hooks.EventStop,
		SessionID: call.SessionID,
		Outcome:   outcome,
		Error:     complete.Error,
	}); err != nil {
		slog.Warn("Stop hook error, ignoring", "error", err)
	}
}

// deliverRunComplete publishes the terminal event without firing Stop.
// Used for calls that were dropped before they ever ran: a queued prompt
// discarded by a cancel never produced a turn, so there is no Stop to
// report — and firing one there would block Cancel (which holds the
// per-session mutex) for the hook's full timeout, wedging Escape and
// app shutdown.
func (a *sessionAgent) deliverRunComplete(ctx context.Context, call SessionAgentCall, complete notify.RunComplete) {
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

// newRunContexts builds the two contexts a turn runs under.
//
// runCtx carries the per-turn tool values; genCtx is runCtx made
// cancellable and is what agent.Stream executes under, which makes it the
// ancestor of every tool call's context. The two are built together, and
// only here, because the ordering is load-bearing: a value stamped onto
// some other context variable *after* genCtx has been derived is invisible
// to tools.
//
// That is not hypothetical. The root session ID was originally stamped
// further down in Run, on a ctx variable genCtx did not descend from, so
// tools.GetRootSessionFromContext fell back to the *sub-agent's own*
// session ID. Ctrl+B releases waits for the session the user is viewing,
// so a bash command inside an agent-tool sub-run was registered under a
// session the UI never names and could not be backgrounded at all.
//
// A sub-agent run inherits the parent turn's root session; a top-level run
// stamps itself as the root.
func newRunContexts(ctx context.Context, sessionID string) (runCtx, genCtx context.Context, cancel context.CancelFunc) {
	runCtx = context.WithValue(ctx, tools.SessionIDContextKey, sessionID)
	if runCtx.Value(tools.RootSessionIDContextKey) == nil {
		runCtx = context.WithValue(runCtx, tools.RootSessionIDContextKey, sessionID)
	}
	genCtx, cancel = context.WithCancel(runCtx)
	return runCtx, genCtx, cancel
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
		return nil, nil
	}

	// Idle: become the active run. Register the cancel func before dropping
	// the lock so a Cancel that arrives between here and assistant creation
	// is not lost.
	var runCtx context.Context
	runCtx, genCtx, cancel = newRunContexts(ctx, call.SessionID)
	ac := &activeCancel{cancel: cancel}
	a.activeRequests.Set(call.SessionID, ac)
	if call.Accepted != nil {
		call.Accepted.Close()
	}
	sessMu.Unlock()

	defer cancel()
	// Conditional cleanup: only remove our entry if it hasn't been replaced
	// by a newer run. Without this guard, the deferred Del fires after a
	// concurrent run registers in the completion window, silently wiping
	// the new run's cancel and breaking cancellation.
	defer a.activeRequests.CompareAndDelete(call.SessionID, ac)

	// Copy mutable fields under lock to avoid races with SetTools/SetModels.
	agentTools := a.tools.Copy()
	largeModel := a.largeModel.Get()
	systemPrompt := a.systemPrompt.Get()
	promptPrefix := a.systemPromptPrefix.Get()
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

	if a.planModeFn != nil && a.planModeFn() {
		systemPrompt += "\n\n" + planModePrompt
	}

	if len(agentTools) > 0 {
		// Add Anthropic caching to the last tool.
		agentTools[len(agentTools)-1].SetProviderOptions(a.getCacheControlOptions())
	}

	agent := fantasy.NewAgent(
		wrapRetryableModel(largeModel.Model),
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
		go a.GenerateTitle(titleCtx, call.SessionID, call.Prompt)
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
	//
	// runCtx already carries both the session ID and the root session ID
	// (see newRunContexts); reuse it rather than re-deriving from ctx so
	// the two contexts cannot drift apart.
	ctx = runCtx
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

	history, files := a.preparePrompt(msgs, largeModel.CatwalkCfg.SupportsImages, currentSession.Todos, call.Attachments...)

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
	maxRetries := providerMaxRetries
	var retryAttempt retryAttemptCounter
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
		MaxRetries:       &maxRetries,
		PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			// Fantasy gives each step its own retry budget. Reset the
			// user-visible attempt counter to match, otherwise retries
			// on later steps keep climbing past a prior success
			// ("attempt 5/6" after the previous step already recovered).
			retryAttempt.Reset()

			prepared.Messages = options.Messages
			for i := range prepared.Messages {
				prepared.Messages[i].ProviderOptions = nil
			}

			// Use latest tools (updated by SetTools when MCP tools change).
			prepared.Tools = a.tools.Copy()

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
			fold, canceledRunIDs := a.drainQueueForStep(call.SessionID)
			a.publishCanceledQueueDrops(canceledRunIDs)
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
			attempt := retryAttempt.Next()
			slog.Warn("Provider request failed, retrying", providerRetryLogFields(err, delay, attempt, maxRetries)...)
			// Reset streamed content so the retried response doesn't
			// concatenate with partial content from the failed attempt.
			// On the final attempt (no more retries), any partial content
			// stays in the message as useful context beneath the error.
			if currentAssistant != nil {
				currentAssistant.ResetStreamedContent()
				if updateErr := a.messages.Update(genCtx, *currentAssistant); updateErr != nil {
					slog.Error("Failed to reset message on retry", "error", updateErr)
				}
			}
			a.publishRetry(call.SessionID, currentSession.Title, largeModel.ModelCfg.Provider, err, delay, attempt, maxRetries)
		},
		OnAuthRefresh: func(callContext context.Context, err *fantasy.ProviderError) error {
			if call.OnAuthRefresh != nil {
				refreshErr := call.OnAuthRefresh(callContext, err)
				if refreshErr == nil {
					retryAttempt.Reset()
				}
				return refreshErr
			}
			return nil
		},
		ModelProvider: func() fantasy.LanguageModel {
			m := a.largeModel.Get()
			slog.Info("ModelProvider called",
				"provider", m.ModelCfg.Provider,
				"model", m.ModelCfg.Model)
			return wrapRetryableModel(m.Model)
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
				slog.Warn("Provider content filter stopped the model",
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
			sessionLock.Lock()
			defer sessionLock.Unlock()

			updatedSession, getSessionErr := a.sessions.Get(ctx, call.SessionID)
			if getSessionErr != nil {
				return getSessionErr
			}
			usage, estimated := fallbackStepUsage(stepMessages, stepResult)
			overrideCost := a.openrouterCost(stepResult.ProviderMetadata)
			stepCost := stepUsageCost(largeModel, usage, overrideCost, estimated)
			promptTok, completionTok := stepUsageTokens(usage)
			currentAssistant.AddFinishWithUsage(finishReason, "", "", promptTok, completionTok, stepCost)
			a.updateSessionUsage(largeModel, &updatedSession, usage, overrideCost, estimated)
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
		toolCalls := currentAssistant.ToolCalls()
		// INFO: we use the cleanup context here because the genCtx has been cancelled.
		msgs, createErr := a.messages.List(cleanupCtx, currentAssistant.SessionID)
		if createErr != nil {
			return nil, createErr
		}
		for _, tc := range toolCalls {
			if !tc.Finished {
				tc.Finished = true
				tc.Input = "{}"
				currentAssistant.AddToolCall(tc)
				updateErr := a.messages.Update(cleanupCtx, *currentAssistant)
				if updateErr != nil {
					return nil, updateErr
				}
			}

			found := false
			for _, msg := range msgs {
				if msg.Role == message.Tool {
					for _, tr := range msg.ToolResults() {
						if tr.ToolCallID == tc.ID {
							found = true
							break
						}
					}
				}
				if found {
					break
				}
			}
			if found {
				continue
			}
			content := "There was an error while executing the tool"
			if isCancelErr {
				content = "Error: user cancelled assistant tool calling"
			}
			toolResult := message.ToolResult{
				ToolCallID: tc.ID,
				Name:       tc.Name,
				Content:    content,
				IsError:    true,
			}
			_, createErr = a.messages.Create(cleanupCtx, currentAssistant.SessionID, message.CreateMessageParams{
				Role: message.Tool,
				Parts: []message.ContentPart{
					toolResult,
				},
			})
			if createErr != nil {
				return nil, createErr
			}
		}
		var fantasyErr *fantasy.Error
		var providerErr *fantasy.ProviderError
		const defaultTitle = "Provider Error"
		linkStyle := lipgloss.NewStyle().Foreground(charmtone.Guac).Underline(true)
		if isCancelErr {
			currentAssistant.AddFinish(message.FinishReasonCanceled, "User canceled request", "")
		} else if isHyper && errors.As(err, &providerErr) && providerErr.StatusCode == http.StatusUnauthorized {
			currentAssistant.AddFinish(message.FinishReasonError, "Unauthorized", `Please re-authenticate with Hyper. You can also run "crush auth" to re-authenticate.`)
		} else if isHyper && errors.As(err, &providerErr) && providerErr.StatusCode == http.StatusPaymentRequired {
			url := hyper.BaseURL()
			link := linkStyle.Hyperlink(url, "id=hyper").Render(url)
			currentAssistant.AddFinish(message.FinishReasonError, "No credits", "You're out of credits. Add more at "+link)
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
				title, body := formatProviderErrorForAssistant(providerErr)
				currentAssistant.AddFinish(message.FinishReasonError, title, body)
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

	if shouldSummarize {
		a.activeRequests.Del(call.SessionID)
		if summarizeErr := a.Summarize(genCtx, call.SessionID, call.ProviderOptions, call.OnAuthRefresh); summarizeErr != nil {
			return nil, summarizeErr
		}
		// If the agent wasn't done...
		if len(currentAssistant.ToolCalls()) > 0 {
			existing, ok := a.messageQueue.Get(call.SessionID)
			if !ok {
				existing = []SessionAgentCall{}
			}
			call.Prompt = fmt.Sprintf("The previous session was interrupted because it got too long, the initial user request was: `%s`", call.Prompt)
			existing = append(existing, call)
			a.messageQueue.Set(call.SessionID, existing)
		}
	}

	// Release active request before publishing the notification.
	// TUI handlers poll IsSessionBusy() and only re-evaluate when a
	// tea.Msg arrives, so the cleanup must precede the notify or
	// subscribers see stale busy state at the moment of receipt.
	a.activeRequests.Del(call.SessionID)
	cancel()

	// Send notification that agent has finished its turn (skip for
	// nested/non-interactive sessions).
	if !call.NonInteractive && a.notify != nil {
		a.notify.Publish(pubsub.CreatedEvent, notify.Notification{
			SessionID:    call.SessionID,
			SessionTitle: currentSession.Title,
			Type:         notify.TypeAgentFinished,
		})
	}

	// Hand off to the next queued prompt (if any) under dispatchMu so
	// the transition from this finished run to the queued run is atomic
	// against a concurrent Cancel. activeRequests for this session was
	// just deleted above, so without the lock there is a window in
	// which the session looks idle and a cancel becomes a no-op that
	// fails to stop the queued prompt. Holding the lock lets us observe
	// a pending cancel recorded against the session and drop the queue
	// instead of running it, and (for the recursion) hand a fresh
	// accept reservation to the dequeued call so acceptedRuns stays > 0
	// across the recursive Run's own dispatch handoff — keeping the
	// session observable to Cancel for the entire transition and
	// closing the dequeue -> re-register window.
	mu := a.sessionMu(call.SessionID)
	mu.Lock()
	queuedMessages, _ := a.messageQueue.Get(call.SessionID)
	if mark, ok := a.cancelMark.Get(call.SessionID); ok && mark > 0 && len(queuedMessages) > 0 {
		// A cancel was recorded for this session (e.g. it arrived while
		// this run was active and follow-ups had been queued). Drop the
		// queued prompts it covers (accept sequence at or below the
		// mark, or untracked); keep any queued after the cancel (higher
		// sequence) so they still run.
		var kept []SessionAgentCall
		var canceledRunIDDrops []SessionAgentCall
		for _, q := range queuedMessages {
			if q.acceptSeq == 0 || q.acceptSeq <= mark {
				if q.RunID != "" {
					canceledRunIDDrops = append(canceledRunIDDrops, q)
				}
				continue
			}
			kept = append(kept, q)
		}
		queuedMessages = kept
		a.messageQueue.Set(call.SessionID, kept)
		// A dropped prompt carrying a RunID must still publish its
		// terminal cancelled RunComplete so a caller waiting on that
		// RunID does not hang.
		a.publishCanceledQueueDrops(canceledRunIDDrops)
	}
	if len(queuedMessages) == 0 {
		// No queued work. Clear the cancel mark only when no accepted
		// run remains in flight that it might still cover; otherwise a
		// sibling prompt (sequence at or below the mark) waiting to
		// enter Run would lose its cancellation. When accepted runs are
		// gone, this also clears a stale mark so it can't catch a
		// future run.
		a.messageQueue.Del(call.SessionID)
		a.acceptedMu.Lock()
		inFlight, _ := a.acceptedRuns.Get(call.SessionID)
		a.acceptedMu.Unlock()
		if inFlight == 0 {
			a.cancelMark.Del(call.SessionID)
		}
		mu.Unlock()
		return result, err
	}
	// There are queued messages, restart the loop. Suppress the outer
	// defer's emit: it would otherwise observe the recursive Run's retErr
	// (named-return clobbering through the return below) against this
	// turn's MessageID/Text and publish a mixed, racing event.
	skipRunComplete = true
	// Decide whether this turn still owes its own terminal RunComplete.
	// Each submitted prompt with a RunID has its own lifecycle, so a turn
	// that is finished and handing off to a *different* queued prompt must
	// publish its own RunComplete here — leaving it to the recursive turn
	// (which carries a different RunID) would hang a caller waiting on
	// this turn's RunID. The exception is the summarize-continuation path,
	// which re-queues this same call (same RunID) to resume after a
	// summary; in that case the eventual terminal turn for this RunID
	// publishes, so publishing now would double-emit.
	outerOwesRunComplete := call.RunID != ""
	if outerOwesRunComplete {
		for _, q := range queuedMessages {
			if q.RunID == call.RunID {
				outerOwesRunComplete = false
				break
			}
		}
	}
	firstQueuedMessage := queuedMessages[0]
	a.messageQueue.Set(call.SessionID, queuedMessages[1:])
	// Reserve a fresh accept for the dequeued prompt before dropping the
	// lock so acceptedRuns > 0 across the handoff into the recursive
	// Run. This closes the window between this dequeue and the recursive
	// Run registering its activeRequests entry: a cancel arriving in
	// that window now records a pending cancel (acceptedRuns > 0) that
	// the recursive Run's accepted path observes as cancel-on-entry.
	firstQueuedMessage.Accepted = a.BeginAccepted(call.SessionID)
	mu.Unlock()
	if outerOwesRunComplete {
		complete := notify.RunComplete{SessionID: call.SessionID, RunID: call.RunID}
		if currentAssistant != nil {
			complete.MessageID = currentAssistant.ID
			complete.Text = currentAssistant.Content().String()
		}
		if ctx.Err() != nil {
			complete.Cancelled = true
		}
		a.publishRunComplete(ctx, call, complete)
	}
	return a.Run(ctx, firstQueuedMessage)
}

func (a *sessionAgent) Summarize(ctx context.Context, sessionID string, opts fantasy.ProviderOptions, onAuthRefresh func(context.Context, *fantasy.ProviderError) error) error {
	if a.IsSessionBusy(sessionID) {
		return ErrSessionBusy
	}

	// Copy mutable fields under lock to avoid races with SetModels.
	largeModel := a.largeModel.Get()
	systemPromptPrefix := a.systemPromptPrefix.Get()

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

	aiMsgs, _ := a.preparePrompt(msgs, largeModel.CatwalkCfg.SupportsImages, currentSession.Todos)

	genCtx, cancel := context.WithCancel(ctx)
	ac := &activeCancel{cancel: cancel}
	a.activeRequests.Set(sessionID, ac)
	defer a.activeRequests.CompareAndDelete(sessionID, ac)
	defer cancel()
	defer func() {
		if flushErr := a.messages.FlushAll(ctx); flushErr != nil {
			slog.Error("Failed to flush pending message updates after summarize", "error", flushErr)
		}
	}()

	agent := fantasy.NewAgent(
		wrapRetryableModel(largeModel.Model),
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

	maxRetries := providerMaxRetries
	var retryAttempt retryAttemptCounter
	resp, err := agent.Stream(genCtx, fantasy.AgentStreamCall{
		Prompt:          summaryPromptText,
		Messages:        aiMsgs,
		Headers:         sessionHeaders(sessionID),
		ProviderOptions: opts,
		MaxRetries:      &maxRetries,
		OnAuthRefresh:   onAuthRefresh,
		ModelProvider: func() fantasy.LanguageModel {
			return a.largeModel.Get().Model
		},
		PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			// Match fantasy's per-step retry budget (see run path).
			retryAttempt.Reset()

			prepared.Messages = options.Messages
			if systemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(systemPromptPrefix)}, prepared.Messages...)
			}
			return callContext, prepared, nil
		},
		OnRetry: func(err *fantasy.ProviderError, delay time.Duration) {
			attempt := retryAttempt.Next()
			slog.Warn("Provider request failed, retrying", providerRetryLogFields(err, delay, attempt, maxRetries)...)
			summaryMessage.ResetStreamedContent()
			if updateErr := a.messages.Update(genCtx, summaryMessage); updateErr != nil {
				slog.Error("Failed to reset summary message on retry", "error", updateErr)
			}
			a.publishRetry(sessionID, currentSession.Title, largeModel.ModelCfg.Provider, err, delay, attempt, maxRetries)
		},
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

	// Record the summarization usage on the summary message itself, using the
	// same figures that are about to be added to session.Cost below. A plain
	// AddFinish here would leave summarization spend on the session row but
	// invisible to `session show --by-model`, so the breakdown could never
	// reconcile on an auto-summarized session.
	summaryPromptTok, summaryCompletionTok := stepUsageTokens(resp.TotalUsage)
	summaryMessage.AddFinishWithUsage(
		message.FinishReasonEndTurn, "", "",
		summaryPromptTok, summaryCompletionTok,
		stepUsageCost(largeModel, resp.TotalUsage, openrouterCost, false),
	)
	err = a.messages.Update(genCtx, summaryMessage)
	if err != nil {
		return err
	}

	a.updateSessionUsage(largeModel, &currentSession, resp.TotalUsage, openrouterCost, false)

	// Just in case, get just the last usage info.
	usage := resp.Response.Usage
	currentSession.SummaryMessageID = summaryMessage.ID
	currentSession.CompletionTokens = summaryCompletionTokens(usage, summaryMessage)
	currentSession.PromptTokens = 0
	currentSession.EstimatedUsage = usageIsZero(usage)
	_, err = a.sessions.Save(genCtx, currentSession)
	if err != nil {
		return err
	}

	// Release the active request before processing queued messages so that
	// Run() does not see the session as busy.
	a.activeRequests.Del(sessionID)
	cancel()

	// Process any messages that were queued while summarizing.
	queuedMessages, ok := a.messageQueue.Get(sessionID)
	if !ok || len(queuedMessages) == 0 {
		return nil
	}
	firstQueuedMessage := queuedMessages[0]
	a.messageQueue.Set(sessionID, queuedMessages[1:])
	_, qErr := a.Run(ctx, firstQueuedMessage)
	return qErr
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

func (a *sessionAgent) preparePrompt(msgs []message.Message, supportsImages bool, todos []session.Todo, attachments ...message.Attachment) ([]fantasy.Message, []fantasy.FilePart) {
	var history []fantasy.Message
	if reminder := todoSystemReminder(a.isSubAgent, todos); reminder != "" {
		history = append(history, fantasy.NewUserMessage(
			fmt.Sprintf("<system_reminder>%s</system_reminder>", reminder),
		))
	}
	// Collect all tool call IDs present in assistant messages and all tool
	// result IDs present in tool messages. This lets us detect both orphaned
	// tool results (result without a call) and orphaned tool calls (call
	// without a result).
	knownToolCallIDs := make(map[string]struct{})
	knownToolResultIDs := make(map[string]struct{})
	for _, m := range msgs {
		switch m.Role {
		case message.Assistant:
			for _, tc := range m.ToolCalls() {
				knownToolCallIDs[tc.ID] = struct{}{}
			}
		case message.Tool:
			for _, tr := range m.ToolResults() {
				knownToolResultIDs[tr.ToolCallID] = struct{}{}
			}
		}
	}

	for _, m := range msgs {
		if len(m.Parts) == 0 {
			continue
		}
		// Assistant message without content or tool calls (cancelled before it returned anything).
		if m.Role == message.Assistant && len(m.ToolCalls()) == 0 && m.Content().Text == "" && m.ReasoningContent().String() == "" {
			continue
		}
		if m.Role == message.Tool {
			if msg, ok := filterOrphanedToolResults(m, knownToolCallIDs); ok {
				history = append(history, msg)
			}
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
			if msg, ok := syntheticToolResultsForOrphanedCalls(m, knownToolResultIDs); ok {
				history = append(history, msg)
			}
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

// filterOrphanedToolResults converts a tool message to a fantasy.Message,
// dropping any tool result parts whose tool_call_id has no matching tool call
// in the known set. An orphaned result causes API validation to fail on every
// subsequent turn, permanently locking the session. Returns the filtered
// message and true if at least one valid part remains.
func filterOrphanedToolResults(m message.Message, knownToolCallIDs map[string]struct{}) (fantasy.Message, bool) {
	aiMsgs := m.ToAIMessage()
	if len(aiMsgs) == 0 {
		return fantasy.Message{}, false
	}
	var validParts []fantasy.MessagePart
	for _, part := range aiMsgs[0].Content {
		tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part)
		if !ok {
			validParts = append(validParts, part)
			continue
		}
		if _, known := knownToolCallIDs[tr.ToolCallID]; known {
			validParts = append(validParts, part)
		} else {
			slog.Warn(
				"Dropping orphaned tool result with no matching tool call",
				"tool_call_id", tr.ToolCallID,
			)
		}
	}
	if len(validParts) == 0 {
		return fantasy.Message{}, false
	}
	msg := aiMsgs[0]
	msg.Content = validParts
	return msg, true
}

// syntheticToolResultsForOrphanedCalls returns a tool message containing
// synthetic tool results for any tool calls in the assistant message that
// have no matching result in knownToolResultIDs. LLM APIs require every
// tool_use to be immediately followed by a tool_result; an interrupted
// session can leave orphaned tool_use blocks that permanently lock the
// conversation. Returns the message and true if any synthetic results were
// produced.
func syntheticToolResultsForOrphanedCalls(m message.Message, knownToolResultIDs map[string]struct{}) (fantasy.Message, bool) {
	var syntheticParts []fantasy.MessagePart
	for _, tc := range m.ToolCalls() {
		if _, hasResult := knownToolResultIDs[tc.ID]; hasResult {
			continue
		}
		slog.Warn(
			"Injecting synthetic tool result for orphaned tool call",
			"tool_call_id", tc.ID,
			"tool_name", tc.Name,
		)
		syntheticParts = append(syntheticParts, fantasy.ToolResultPart{
			ToolCallID: tc.ID,
			Output: fantasy.ToolResultOutputContentError{
				Error: errors.New("tool call was interrupted and did not produce a result, you may retry this call if the result is still needed"),
			},
		})
	}
	if len(syntheticParts) == 0 {
		return fantasy.Message{}, false
	}
	return fantasy.Message{
		Role:    fantasy.MessageRoleTool,
		Content: syntheticParts,
	}, true
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

// sideQuestionAttempts returns the ordered model attempts for a side
// question. Prefer the small model when session usage fits under 80% of
// its context window; otherwise try only the large model. Nil models are
// skipped by the caller.
func sideQuestionAttempts(small, large Model, promptTokens, completionTokens int64) []Model {
	used := promptTokens + completionTokens
	if small.Model != nil {
		if cw := int64(small.CatwalkCfg.ContextWindow); cw <= 0 || used <= (cw*8)/10 {
			attempts := []Model{small}
			if large.Model != nil {
				attempts = append(attempts, large)
			}
			return attempts
		}
	}
	if large.Model != nil {
		return []Model{large}
	}
	if small.Model != nil {
		return []Model{small}
	}
	return nil
}

// SideQuestion answers an ephemeral side question using session history
// as context. It persists nothing: no messages, no session updates.
func (a *sessionAgent) SideQuestion(ctx context.Context, sessionID, question string, exchanges []SideQuestionExchange) (SideQuestionResult, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return SideQuestionResult{}, ErrEmptySideQuestion
	}
	if sessionID == "" {
		return SideQuestionResult{}, ErrSessionMissing
	}

	sess, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return SideQuestionResult{}, fmt.Errorf("failed to get session: %w", err)
	}

	if err := a.messages.FlushAll(ctx); err != nil {
		return SideQuestionResult{}, fmt.Errorf("failed to flush messages: %w", err)
	}

	msgs, err := a.getSessionMessages(ctx, sess)
	if err != nil {
		return SideQuestionResult{}, err
	}

	small := a.smallModel.Get()
	large := a.largeModel.Get()
	attempts := sideQuestionAttempts(small, large, sess.PromptTokens, sess.CompletionTokens)
	if len(attempts) == 0 {
		return SideQuestionResult{}, fmt.Errorf("no model available for side question")
	}

	var lastErr error
	for i, chosen := range attempts {
		aiMsgs, _ := a.preparePrompt(msgs, chosen.CatwalkCfg.SupportsImages, sess.Todos)
		for _, ex := range exchanges {
			q := strings.TrimSpace(ex.Question)
			ans := strings.TrimSpace(ex.Answer)
			if q == "" || ans == "" {
				continue
			}
			aiMsgs = append(aiMsgs, fantasy.NewUserMessage(q))
			aiMsgs = append(aiMsgs, fantasy.Message{
				Role:    fantasy.MessageRoleAssistant,
				Content: []fantasy.MessagePart{fantasy.TextPart{Text: ans}},
			})
		}

		sideAgent := fantasy.NewAgent(
			wrapRetryableModel(chosen.Model),
			fantasy.WithSystemPrompt(string(sideQuestionPrompt)),
			fantasy.WithMaxOutputTokens(2048),
			fantasy.WithUserAgent(userAgent),
		)
		res, err := sideAgent.Generate(ctx, fantasy.AgentCall{
			Prompt:   question,
			Messages: aiMsgs,
			Headers:  sessionHeaders(sessionID),
		})
		if err != nil {
			lastErr = err
			name := "large"
			if i == 0 && chosen.ModelCfg.Model == small.ModelCfg.Model {
				name = "small"
			}
			slog.Error("Side question failed with "+name+" model", "error", err)
			continue
		}

		promptTokens := res.TotalUsage.InputTokens + res.TotalUsage.CacheCreationTokens
		completionTokens := res.TotalUsage.OutputTokens
		return SideQuestionResult{
			Answer:           sanitizeSideQuestionAnswer(res.Response.Content.Text()),
			Model:            chosen.ModelCfg.Model,
			Provider:         chosen.ModelCfg.Provider,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
		}, nil
	}
	return SideQuestionResult{}, lastErr
}

func sanitizeSideQuestionAnswer(ans string) string {
	ans = thinkTagRegex.ReplaceAllString(ans, "")
	ans = orphanThinkTagRegex.ReplaceAllString(ans, "")
	ans = dsmlTagRegex.ReplaceAllString(ans, "")
	ans = orphanDsmlTagRegex.ReplaceAllString(ans, "")
	ans = toolCallTagRegex.ReplaceAllString(ans, "")
	ans = strings.TrimSpace(ans)
	ans = orphanIntroRegex.ReplaceAllString(ans, "")
	return strings.TrimSpace(ans)
}

// GenerateTitle generates a session title based on the initial prompt.
func (a *sessionAgent) GenerateTitle(ctx context.Context, sessionID string, userPrompt string) {
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

	smallModel := a.smallModel.Get()
	largeModel := a.largeModel.Get()
	systemPromptPrefix := a.systemPromptPrefix.Get()

	newAgent := func(m fantasy.LanguageModel, p []byte, tok int64) fantasy.Agent {
		return fantasy.NewAgent(
			wrapRetryableModel(m),
			fantasy.WithSystemPrompt(string(p)+"\n /no_think"),
			fantasy.WithMaxOutputTokens(tok),
			fantasy.WithUserAgent(userAgent),
		)
	}

	maxRetries := providerMaxRetries
	streamCall := fantasy.AgentStreamCall{
		Prompt:     fmt.Sprintf("Generate a concise title for the following content:\n\n%s\n <think>\n\n</think>", userPrompt),
		Headers:    sessionHeaders(sessionID),
		MaxRetries: &maxRetries,
		PrepareStep: func(callCtx context.Context, opts fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = opts.Messages
			if systemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{
					fantasy.NewSystemMessage(systemPromptPrefix),
				}, prepared.Messages...)
			}
			return callCtx, prepared, nil
		},
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
		// Each model attempt gets its own retry budget from fantasy, so
		// it needs its own counter too. Sharing one across the
		// small→large fallback made the countdown read "attempt 7/6".
		streamCall.OnRetry = newRetryAttemptReporter(a, sessionID, attempt.model.ModelCfg.Provider, maxRetries)
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

	cost := stepUsageCost(model, resp.TotalUsage, openrouterCost, false)
	promptTokens, completionTokens := stepUsageTokens(resp.TotalUsage)

	// Record the title generation's usage against the model that incurred it.
	// UpdateTitleAndUsage below adds the money to the PARENT session's cost
	// column, so without a per-model record of it the spend would be
	// unattributable and `session show --by-model` could never reconcile: title
	// generation runs on the first prompt of every session, and it may run on
	// the small model or fall back to the large one.
	a.recordTitleUsage(ctx, sessionID, model, promptTokens, completionTokens, cost)

	// Atomically update only title and usage fields to avoid overriding other
	// concurrent session updates.
	saveErr := a.sessions.UpdateTitleAndUsage(ctx, sessionID, title, promptTokens, completionTokens, cost)
	if saveErr != nil {
		slog.Error("Failed to save session title and usage", "error", saveErr)
		return
	}
	titleSaved = true
}

// recordTitleUsage persists the title generation's token and cost usage as an
// assistant message in the session's dedicated title sub-session, so the spend
// is attributable to the model that actually incurred it.
//
// The parent keeps the money on its own cost column (see the
// UpdateTitleAndUsage call in GenerateTitle) exactly as sub-agent cost is
// propagated upward, and this sub-session carries the record that makes the
// two reconcile. Failures are logged and swallowed: a title is not worth
// failing a turn over.
func (a *sessionAgent) recordTitleUsage(ctx context.Context, sessionID string, model Model, promptTokens, completionTokens int64, cost float64) {
	titleSession, err := a.sessions.CreateTitleSession(ctx, sessionID)
	if err != nil {
		slog.Warn("Failed to create title session for usage accounting", "session_id", sessionID, "error", err)
		return
	}
	msg, err := a.messages.Create(ctx, titleSession.ID, message.CreateMessageParams{
		Role:     message.Assistant,
		Model:    model.ModelCfg.Model,
		Provider: model.ModelCfg.Provider,
	})
	if err != nil {
		slog.Warn("Failed to record title usage message", "session_id", sessionID, "error", err)
		return
	}
	msg.AddFinishWithUsage(message.FinishReasonEndTurn, "", "", promptTokens, completionTokens, cost)
	if err := a.messages.Update(ctx, msg); err != nil {
		slog.Warn("Failed to save title usage message", "session_id", sessionID, "error", err)
		return
	}
	titleSession.Cost = cost
	titleSession.PromptTokens = promptTokens
	titleSession.CompletionTokens = completionTokens
	if _, err := a.sessions.Save(ctx, titleSession); err != nil {
		slog.Warn("Failed to save title session usage", "session_id", sessionID, "error", err)
	}
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

func stepUsageTokens(usage fantasy.Usage) (promptTokens, completionTokens int64) {
	return usage.InputTokens + usage.CacheCreationTokens + usage.CacheReadTokens, usage.OutputTokens
}

func stepUsageCost(model Model, usage fantasy.Usage, overrideCost *float64, estimated bool) float64 {
	if estimated {
		return 0
	}
	if overrideCost != nil {
		if model.FlatRate {
			return 0
		}
		return *overrideCost
	}
	if model.FlatRate {
		return 0
	}
	modelConfig := model.CatwalkCfg
	return modelConfig.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
		modelConfig.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
		modelConfig.CostPer1MIn/1e6*float64(usage.InputTokens) +
		modelConfig.CostPer1MOut/1e6*float64(usage.OutputTokens)
}

func (a *sessionAgent) updateSessionUsage(model Model, session *session.Session, usage fantasy.Usage, overrideCost *float64, estimated bool) {
	if !usageIsZero(usage) {
		session.EstimatedUsage = estimated
	}

	cost := stepUsageCost(model, usage, overrideCost, estimated)
	if !estimated {
		a.eventTokensUsed(session.ID, model, usage, cost)
	}

	session.Cost += cost
	updateSessionTokenCounters(session, usage)
}

// updateSessionTokenCounters overwrites (does not accumulate) the session row's
// token counters. They track the CONTEXT OCCUPANCY of the latest step, which is
// what the auto-summarize StopWhen condition, the header context gauge and the
// sidebar all read them as; Summarize resets PromptTokens to 0 for the same
// reason. They are therefore NOT a per-turn total and must never be compared
// with the per-model breakdown, which sums each step's usage.
//
// The prompt figure counts every token the provider had to read for the step,
// cache-creation included, so it matches stepUsageTokens.
func updateSessionTokenCounters(session *session.Session, usage fantasy.Usage) {
	if usage.OutputTokens != 0 {
		session.CompletionTokens = usage.OutputTokens
	}
	if promptTokens, _ := stepUsageTokens(usage); promptTokens != 0 {
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

	// Cancel regular requests. Don't use Take() here - we need the entry to
	// remain in activeRequests so IsBusy() returns true until the goroutine
	// fully completes (including error handling that may access the DB).
	// The defer in processRequest will clean up the entry.
	if ac, ok := a.activeRequests.Get(sessionID); ok && ac != nil {
		slog.Debug("Request cancellation initiated", "session_id", sessionID)
		ac.cancel()
	}

	// Also check for summarize requests.
	if ac, ok := a.activeRequests.Get(sessionID + "-summarize"); ok && ac != nil {
		slog.Debug("Summarize cancellation initiated", "session_id", sessionID)
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

// todoSystemReminder returns a prompt-only reminder about the session
// todo list. Empty when there is nothing useful to say (sub-agents, or
// a healthy in-progress list). Incomplete lists that have no current
// in_progress item, and lists where every item is already completed,
// get an explicit nudge so the UI pill does not linger after the work
// is done. An empty list only gets the optional "consider creating"
// hint.
func todoSystemReminder(isSubAgent bool, todos []session.Todo) string {
	if isSubAgent {
		return ""
	}
	if len(todos) == 0 {
		return `This is a reminder that your todo list is currently empty. DO NOT mention this to the user explicitly because they are already aware.
If you are working on tasks that would benefit from a todo list please use the "todos" tool to create one.
If not, please feel free to ignore. Again do not mention this message to the user.`
	}

	completed := 0
	inProgress := 0
	for _, t := range todos {
		switch t.Status {
		case session.TodoStatusCompleted:
			completed++
		case session.TodoStatusInProgress:
			inProgress++
		}
	}

	switch {
	case completed == len(todos):
		// Pre-auto-clear sessions (or external writers) can leave an
		// all-completed list on disk. Nudge one empty update so the
		// pill disappears.
		return `Your todo list is fully completed but still visible in the UI. Call the todos tool with an empty todos array to clear it. DO NOT mention this reminder to the user.`
	case inProgress == 0:
		return `Your todo list still has unfinished items and none are marked in_progress. Mark the next task in_progress before continuing, or complete remaining work and clear the list with an empty todos array when finished. DO NOT mention this reminder to the user.`
	default:
		return ""
	}
}

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

func providerRetryLogFields(err *fantasy.ProviderError, delay time.Duration, attempt, maxRetries int) []any {
	fields := []any{
		"retry_delay", delay.String(),
		"attempt", attempt,
		"max_retries", maxRetries,
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

// retryAttemptCounter tracks the 1-based attempt number shown in the
// retry countdown. Fantasy allocates a fresh MaxRetries budget per
// stream step (and per AgentStreamCall), so callers must Reset between
// those boundaries or the UI keeps climbing past a prior recovery
// ("attempt 5/6") and can exceed maxRetries ("attempt 7/6").
type retryAttemptCounter struct {
	n atomic.Int32
}

// Next returns the next 1-based attempt number.
func (c *retryAttemptCounter) Next() int {
	return int(c.n.Add(1))
}

// Reset clears the counter so the next Next() returns 1.
func (c *retryAttemptCounter) Reset() {
	c.n.Store(0)
}

// newRetryAttemptReporter returns an OnRetry callback with a fresh
// attempt counter. Used when a single call has no PrepareStep hook to
// reset on (GenerateTitle's small→large fallback): each call site must
// install a new reporter so the first call's count does not leak.
func newRetryAttemptReporter(a *sessionAgent, sessionID, providerID string, maxRetries int) func(*fantasy.ProviderError, time.Duration) {
	var retryAttempt retryAttemptCounter
	return func(err *fantasy.ProviderError, delay time.Duration) {
		attempt := retryAttempt.Next()
		slog.Warn("Provider request failed, retrying", providerRetryLogFields(err, delay, attempt, maxRetries)...)
		a.publishRetry(sessionID, "", providerID, err, delay, attempt, maxRetries)
	}
}

type parsedErrorJSON struct {
	Type       string        `json:"type"`
	Message    string        `json:"message"`
	RetryAfter time.Duration `json:"retryAfter"`
}

// epochThreshold distinguishes a relative duration in seconds from a Unix
// epoch timestamp. Providers return both shapes in Retry-After and
// x-ratelimit-reset-* headers with no way to tell them apart other than
// magnitude. 1e9 seconds is ~31 years as a relative delay -- never a real
// backoff -- and 1e9 as an epoch is 2001-09-09, safely in the past, so
// anything above it is a timestamp.
const epochThreshold = 1e9

// timeNow is overridable so tests can fix the wall clock for the epoch
// and HTTP-date duration maths in secondsOrEpoch and parseFlexDuration.
var timeNow = time.Now

// secondsOrEpoch interprets a positive number of seconds as either a relative
// delay or a Unix epoch timestamp, depending on magnitude. An epoch already in
// the past yields a zero duration, which callers would silently treat as
// "no value"; clamp it to 0 explicitly.
func secondsOrEpoch(sec float64) time.Duration {
	if sec > epochThreshold {
		if t := time.Unix(int64(sec), 0); t.After(timeNow()) {
			return t.Sub(timeNow())
		}
		return 0
	}
	return time.Duration(sec * float64(time.Second))
}

func parseFlexDuration(v any) time.Duration {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		if val > 0 {
			return secondsOrEpoch(val)
		}
	case int64:
		if val > 0 {
			return secondsOrEpoch(float64(val))
		}
	case int:
		if val > 0 {
			return secondsOrEpoch(float64(val))
		}
	case string:
		val = strings.TrimSpace(val)
		if val == "" {
			return 0
		}
		if sec, err := strconv.ParseFloat(val, 64); err == nil && sec > 0 {
			return secondsOrEpoch(sec)
		}
		if d, err := time.ParseDuration(val); err == nil && d > 0 {
			return d
		}
		// Retry-After may also be an HTTP-date (RFC 9110) rather than
		// delta-seconds. Try it last: a date can never be parsed as a
		// number, so the numeric forms above take precedence with no
		// ambiguity.
		if t, err := http.ParseTime(val); err == nil {
			if d := t.Sub(timeNow()); d > 0 {
				return d
			}
			return 0
		}
	}
	return 0
}

func formatDurationHuman(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	d = d.Truncate(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("resets in %ds", int(d.Seconds()))
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60
	if hours > 0 {
		if mins > 0 {
			return fmt.Sprintf("resets in %dh%dm", hours, mins)
		}
		return fmt.Sprintf("resets in %dh", hours)
	}
	if secs > 0 {
		return fmt.Sprintf("resets in %dm%ds", mins, secs)
	}
	return fmt.Sprintf("resets in %dm", mins)
}

func parseJSONErrorString(input string) *parsedErrorJSON {
	input = strings.TrimSpace(input)
	start := strings.Index(input, "{")
	end := strings.LastIndex(input, "}")
	if start == -1 || end == -1 || end <= start {
		return nil
	}
	jsonSub := input[start : end+1]
	var payload struct {
		Type            string `json:"type"`
		Message         string `json:"message"`
		RetryAfter      any    `json:"retryAfter"`
		RetryAfterSnake any    `json:"retry_after"`
		RetryAfterSecs  any    `json:"retry_after_seconds"`
		ResetsIn        any    `json:"resets_in"`
		ResetIn         any    `json:"reset_in"`
		Error           struct {
			Type            string `json:"type"`
			Message         string `json:"message"`
			Code            any    `json:"code"`
			Status          string `json:"status"`
			Detail          string `json:"detail"`
			RetryAfter      any    `json:"retryAfter"`
			RetryAfterSnake any    `json:"retry_after"`
			RetryAfterSecs  any    `json:"retry_after_seconds"`
			ResetsIn        any    `json:"resets_in"`
			ResetIn         any    `json:"reset_in"`
		} `json:"error"`
		Detail string `json:"detail"`
	}
	if json.Unmarshal([]byte(jsonSub), &payload) != nil {
		return nil
	}
	res := &parsedErrorJSON{}
	if payload.Type != "" {
		res.Type = payload.Type
	} else if payload.Error.Type != "" {
		res.Type = payload.Error.Type
	}

	if payload.Message != "" {
		res.Message = payload.Message
	} else if payload.Error.Message != "" {
		res.Message = payload.Error.Message
	} else if payload.Detail != "" {
		res.Message = payload.Detail
	} else if payload.Error.Detail != "" {
		res.Message = payload.Error.Detail
	}

	for _, rawVal := range []any{
		payload.RetryAfter, payload.RetryAfterSnake, payload.RetryAfterSecs, payload.ResetsIn, payload.ResetIn,
		payload.Error.RetryAfter, payload.Error.RetryAfterSnake, payload.Error.RetryAfterSecs, payload.Error.ResetsIn, payload.Error.ResetIn,
	} {
		if dur := parseFlexDuration(rawVal); dur > 0 {
			res.RetryAfter = dur
			break
		}
	}

	return res
}

func cleanErrorString(s string) string {
	s = strings.TrimSpace(s)
	// Strip Go HTTP client request URL prefix (e.g. `POST "https://...": 429 Too Many Requests ...`)
	if idx := strings.Index(s, "\": "); idx != -1 {
		s = strings.TrimSpace(s[idx+3:])
	}
	// Strip trailing embedded JSON substring if present after non-JSON text
	if start := strings.Index(s, "{"); start > 0 {
		s = strings.TrimSpace(s[:start])
	} else if start == 0 {
		// Pure JSON string
		s = ""
	}
	return s
}

// formatProviderError returns a detailed, human-readable description of a provider failure.
// It inspects status codes, response bodies, error messages, and cause chains to clearly
// distinguish rate limits vs quota exhaustion vs provider outages, and includes the full
// underlying error message.
func formatProviderError(err *fantasy.ProviderError) string {
	if err == nil {
		return "provider error"
	}

	title := strings.TrimSpace(err.Title)
	msg := strings.TrimSpace(err.Message)
	var causeStr string
	if err.Cause != nil {
		causeStr = strings.TrimSpace(err.Cause.Error())
	}

	var jsonParsed *parsedErrorJSON
	for _, raw := range []string{msg, string(err.ResponseBody), causeStr} {
		if parsed := parseJSONErrorString(raw); parsed != nil {
			jsonParsed = parsed
			break
		}
	}

	detail := ""
	jsonType := ""
	var retryAfterDur time.Duration
	if jsonParsed != nil {
		jsonType = jsonParsed.Type
		detail = jsonParsed.Message
		retryAfterDur = jsonParsed.RetryAfter
	}

	if retryAfterDur == 0 && err.ResponseHeaders != nil {
		for _, k := range []string{"retry-after", "Retry-After", "x-ratelimit-reset-requests", "X-RateLimit-Reset-Requests"} {
			if val, ok := err.ResponseHeaders[k]; ok {
				if d := parseFlexDuration(val); d > 0 {
					retryAfterDur = d
					break
				}
			}
		}
	}

	retryAfterStr := formatDurationHuman(retryAfterDur)

	// Build full text across all available error fields for categorization
	fullText := strings.Join([]string{title, msg, detail, jsonType, causeStr}, " ")
	lowerFull := strings.ToLower(fullText)

	isQuota := strings.Contains(lowerFull, "quota") ||
		strings.Contains(lowerFull, "credit") ||
		strings.Contains(lowerFull, "billing") ||
		strings.Contains(lowerFull, "insufficient_quota") ||
		strings.Contains(lowerFull, "resource_exhausted") ||
		strings.Contains(lowerFull, "freeusage") ||
		strings.Contains(lowerFull, "usage_limit") ||
		strings.Contains(lowerFull, "limiterror") ||
		strings.Contains(lowerFull, "payment_required") ||
		err.StatusCode == http.StatusPaymentRequired

	isRateLimit := strings.Contains(lowerFull, "rate limit") ||
		strings.Contains(lowerFull, "rate_limit") ||
		strings.Contains(lowerFull, "too many requests") ||
		err.StatusCode == http.StatusTooManyRequests

	isServerDown := strings.Contains(lowerFull, "overloaded") ||
		strings.Contains(lowerFull, "service unavailable") ||
		strings.Contains(lowerFull, "bad gateway") ||
		strings.Contains(lowerFull, "internal server error") ||
		err.StatusCode == http.StatusServiceUnavailable ||
		err.StatusCode == http.StatusBadGateway ||
		err.StatusCode == http.StatusInternalServerError

	var category string
	if isQuota {
		category = "Quota Exceeded / Out of Credits"
	} else if isServerDown {
		category = "Provider Server Down / Overloaded"
	} else if isRateLimit {
		category = "Rate Limit Reached"
	} else if err.StatusCode == http.StatusUnauthorized || err.StatusCode == http.StatusForbidden {
		category = "Authentication / Access Denied"
	}

	// Pick the most specific explanation string
	explanation := detail
	if explanation == "" && causeStr != "" {
		explanation = cleanErrorString(causeStr)
	}
	if explanation == "" {
		explanation = cleanErrorString(msg)
	}
	if explanation == "" {
		explanation = cleanErrorString(title)
	}

	var parts []string
	if err.StatusCode > 0 {
		parts = append(parts, fmt.Sprintf("HTTP %d", err.StatusCode))
	}
	if category != "" {
		catStr := category
		if retryAfterStr != "" {
			catStr += fmt.Sprintf(" (%s)", retryAfterStr)
		}
		parts = append(parts, catStr)
	} else if retryAfterStr != "" {
		parts = append(parts, retryAfterStr)
	}

	// Add explanation if it provides additional information beyond the category name
	if explanation != "" {
		cleanExp := strings.TrimSpace(explanation)
		if !strings.EqualFold(cleanExp, category) {
			// Suppress generic "too many requests" / "rate limit" text when category already explains it
			if (isRateLimit || isQuota) && (strings.EqualFold(cleanExp, "too many requests") || strings.EqualFold(cleanExp, "rate limit")) {
				// Suppress redundant generic string
			} else {
				parts = append(parts, cleanExp)
			}
		}
	}

	if len(parts) > 0 {
		return strings.Join(parts, " - ")
	}
	return "provider error"
}

func formatProviderErrorForAssistant(err *fantasy.ProviderError) (string, string) {
	if err == nil {
		return "Provider Error", "An unknown error occurred."
	}
	formatted := formatProviderError(err)
	parts := strings.Split(formatted, " - ")
	if len(parts) >= 2 {
		if strings.HasPrefix(parts[0], "HTTP ") {
			title := parts[1]
			var body string
			if len(parts) >= 3 {
				body = strings.Join(parts[2:], " - ")
			} else {
				body = parts[0]
			}
			return title, body
		}
		title := parts[0]
		body := strings.Join(parts[1:], " - ")
		return title, body
	}
	return cmp.Or(stringext.Capitalize(err.Title), "Provider Error"), formatted
}

// publishRetry notifies the UI that a provider request failed and the
// agent is backing off before the next attempt.
func (a *sessionAgent) publishRetry(sessionID, sessionTitle, providerID string, err *fantasy.ProviderError, delay time.Duration, attempt, maxRetries int) {
	if a.notify == nil {
		return
	}
	if providerID == "" && a.largeModel != nil {
		m := a.largeModel.Get()
		providerID = m.ModelCfg.Provider
	}
	msg := formatProviderError(err)
	a.notify.Publish(pubsub.CreatedEvent, notify.Notification{
		SessionID:    sessionID,
		SessionTitle: sessionTitle,
		Type:         notify.TypeRetry,
		ProviderID:   providerID,
		Message:      msg,
		RetryDelay:   delay,
		Attempt:      attempt,
		MaxRetries:   maxRetries,
	})
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
