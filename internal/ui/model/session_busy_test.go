package model

import (
	"context"
	"errors"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
)

// countingWorkspace is a workspace.Workspace stub that counts every probe
// that is a synchronous HTTP round-trip in client/server mode, split per
// method so tests can pin exactly which probes ran. The embedded interface
// panics on anything unimplemented.
type countingWorkspace struct {
	workspace.Workspace

	ready          bool
	agentBusy      bool
	yolo           bool
	queued         []string
	queuedMessages []agent.QueuedMessage
	popErr         error
	model          workspace.AgentModel
	lspStates      map[string]workspace.LSPClientInfo
	lspDiags       map[string]lsp.DiagnosticCounts

	readyCalls      int
	agentBusyCalls  int
	queuedCalls     int
	queueListCalls  int
	permCalls       int
	permSetCalls    int
	clearQueueCalls int
	popQueueCalls   int
	cancelCalls     int
	modelCalls      int
	lspStateCalls   int
	lspDiagCalls    int
}

func (w *countingWorkspace) AgentIsReady() bool { w.readyCalls++; return w.ready }
func (w *countingWorkspace) AgentIsBusy() bool  { w.agentBusyCalls++; return w.agentBusy }

func (w *countingWorkspace) AgentReadyErr() error {
	w.readyCalls++
	if w.ready {
		return nil
	}
	return workspace.ErrAgentNotInitialized
}

func (w *countingWorkspace) AgentQueuedPrompts(string) int {
	w.queuedCalls++
	return len(w.queued)
}

func (w *countingWorkspace) AgentQueuedPromptsList(string) []string {
	w.queueListCalls++
	return w.queued
}

func (w *countingWorkspace) PermissionSkipRequests() bool { w.permCalls++; return w.yolo }

func (w *countingWorkspace) PermissionSetSkipRequests(skip bool) {
	w.permSetCalls++
	w.yolo = skip
}

func (w *countingWorkspace) AgentClearQueue(string) { w.clearQueueCalls++; w.queued = nil }
func (w *countingWorkspace) AgentPopQueuedMessage(string) (agent.QueuedMessage, bool, error) {
	w.popQueueCalls++
	if w.popErr != nil {
		return agent.QueuedMessage{}, false, w.popErr
	}
	if len(w.queuedMessages) == 0 {
		return agent.QueuedMessage{}, false, nil
	}
	last := len(w.queuedMessages) - 1
	queued := w.queuedMessages[last]
	w.queuedMessages = w.queuedMessages[:last]
	if len(w.queued) > 0 {
		w.queued = w.queued[:len(w.queued)-1]
	}
	return queued, true, nil
}

// AgentCancel mirrors production: agent.sessionAgent.Cancel ends the turn
// in progress and deliberately leaves the message queue alone, so w.queued
// must stay untouched here. Only AgentClearQueue discards queued prompts.
func (w *countingWorkspace) AgentCancel(string) { w.cancelCalls++ }

func (w *countingWorkspace) AgentModel() workspace.AgentModel {
	w.modelCalls++
	return w.model
}

func (w *countingWorkspace) LSPGetStates() map[string]workspace.LSPClientInfo {
	w.lspStateCalls++
	return w.lspStates
}

func (w *countingWorkspace) LSPGetDiagnosticCounts(name string) lsp.DiagnosticCounts {
	w.lspDiagCalls++
	return w.lspDiags[name]
}

func (w *countingWorkspace) ListMessages(context.Context, string) ([]message.Message, error) {
	return nil, nil
}

func (w *countingWorkspace) ListUserMessages(context.Context, string) ([]message.Message, error) {
	return nil, nil
}

func (w *countingWorkspace) WorkingDir() string { return "" }

func (w *countingWorkspace) LSPStart(context.Context, string) {}

func (w *countingWorkspace) Config() *config.Config { return nil }

// syncProbes sums every synchronous counter; Update/View must keep this at
// zero — the invariant is that no workspace call ever happens on the Update
// goroutine (which is also the render loop).
func (w *countingWorkspace) syncProbes() int {
	return w.readyCalls + w.agentBusyCalls +
		w.queuedCalls + w.queueListCalls + w.permCalls +
		w.modelCalls + w.lspStateCalls + w.lspDiagCalls
}

func (w *countingWorkspace) resetCounters() {
	w.readyCalls, w.agentBusyCalls = 0, 0
	w.queuedCalls, w.queueListCalls, w.permCalls = 0, 0, 0
	w.permSetCalls, w.clearQueueCalls, w.popQueueCalls, w.cancelCalls = 0, 0, 0, 0
	w.modelCalls, w.lspStateCalls, w.lspDiagCalls = 0, 0, 0
}

// newBusyUI builds a UI wired to the stub workspace with an active session
// "s1", enough state for Update to run end to end.
func newBusyUI(ws *countingWorkspace) *UI {
	com := common.DefaultCommon(ws)
	m := &UI{
		com:         com,
		status:      NewStatus(com, nil),
		chat:        NewChat(com, config.ScrollbarDefault),
		textarea:    textarea.New(),
		state:       uiChat,
		focus:       uiFocusEditor,
		width:       140,
		height:      45,
		session:     &session.Session{ID: "s1"},
		keyMap:      DefaultKeyMap(),
		dialog:      dialog.NewOverlay(),
		attachments: attachments.New(nil, attachments.Keymap{}),
	}
	// -1 is "not browsing history", the state production is in once
	// history has loaded or a prompt has been submitted. The struct's zero
	// value (0) would instead mean the editor is showing history entry 0.
	m.promptHistory.index = -1
	return m
}

// pinTTLs makes the TTL backstop inert for the duration of the test so
// assertions about event-driven refreshes cannot flake by straddling a TTL
// boundary (the tests using it must not call t.Parallel).
func pinTTLs(t *testing.T) {
	t.Helper()
	oldBusy, oldQueue, oldLSP := busyCacheTTL, promptQueueTTL, lspStatesTTL
	busyCacheTTL = time.Hour
	promptQueueTTL = time.Hour
	lspStatesTTL = time.Hour
	t.Cleanup(func() { busyCacheTTL, promptQueueTTL, lspStatesTTL = oldBusy, oldQueue, oldLSP })
}

// warmCaches marks all memoized workspace state fresh so only explicit
// invalidation (not startup staleness) can trigger refresh dispatches.
func warmCaches(m *UI, busy bool) {
	m.agentBusyCache.set(busy)
	m.yoloCache.set(false)
	m.agentReady = true
	m.promptQueueCheckedAt = time.Now()
	m.lspCheckedAt = time.Now()
}

// runCmds executes a command tree the way the Bubble Tea runtime would,
// feeding cache-refresh messages back into Update. Other leaf commands are
// executed (for their side effects on the stub) but their messages dropped.
func runCmds(m *UI, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			runCmds(m, c)
		}
	case busyStateMsg, promptQueueMsg, queuedMessagePoppedMsg, promptQueueClearedMsg,
		agentRunSubmittedMsg, lspStatesMsg, agentModelChangedMsg:
		_, next := m.Update(msg)
		runCmds(m, next)
	case util.InfoMsg:
		// Status banners go through Update exactly as they do in production
		// (ui.go's util.InfoMsg case). The one command it returns is the
		// status-clear tick, which blocks for the whole status TTL before
		// emitting util.ClearStatusMsg, so it is deliberately not run here.
		m.Update(msg)
	}
}

// plainMsg is an arbitrary tea.Msg standing in for keystroke/mouse/tick
// traffic through Update.
type plainMsg struct{}

// TestUpdateDoesNotProbeWorkspacePerMessage pins the hot-path fix: Update
// used to call AgentQueuedPrompts (a synchronous HTTP GET in client/server
// mode) at the top of every message while the agent was busy, and the
// placeholder path probed AgentIsReady/AgentIsBusy/PermissionSkipRequests —
// every keystroke blocked the single Update goroutine on network round-
// trips. Now Update performs no synchronous workspace call at all; refreshes
// are dispatched as commands.
func TestUpdateDoesNotProbeWorkspacePerMessage(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)

	for range 25 {
		m.Update(plainMsg{})
	}
	require.Zero(t, ws.queuedCalls,
		"Update must not call AgentQueuedPrompts per message (HTTP per keystroke in client mode)")
	require.Zero(t, ws.syncProbes(),
		"Update must not make any synchronous workspace call")
}

// TestReadsNeverProbeWorkspace pins the read side of the invariant: the
// busy/yolo getters used by render paths serve the memoized value and never
// probe, so View can never block on HTTP.
func TestReadsNeverProbeWorkspace(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, agentBusy: true}
	m := newBusyUI(ws)

	for range 10 {
		m.isAgentBusy()
		m.yoloModeCached()
	}
	require.Zero(t, ws.syncProbes(), "cache reads must never probe the workspace")
}

// TestStreamingUpdatedEventsDoNotProbe pins the streaming path: per-chunk
// message UpdatedEvents arrive once per streamed token and must neither
// probe the workspace synchronously nor schedule busy/queue refreshes —
// only CreatedEvents (run boundaries) do.
func TestStreamingUpdatedEventsDoNotProbe(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, true)
	ws.resetCounters()

	for range 25 {
		m.Update(pubsub.Event[message.Message]{
			Type:    pubsub.UpdatedEvent,
			Payload: message.Message{ID: "m1", SessionID: "s1", Role: message.Assistant},
		})
	}
	require.Zero(t, ws.syncProbes(),
		"per-chunk UpdatedEvents must not probe the workspace")
	require.False(t, m.busyFetchInFlight,
		"per-chunk UpdatedEvents must not schedule a busy refresh")
	require.False(t, m.promptQueueInFlight,
		"per-chunk UpdatedEvents must not schedule a queue refresh")
}

// TestMessageCreatedEventRefreshesBusyAndQueue: a CreatedEvent is a run
// boundary and must invalidate the memoized busy state and fetch fresh
// busy/queue values off-thread.
func TestMessageCreatedEventRefreshesBusyAndQueue(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, agentBusy: true, queued: []string{"queued prompt"}}
	m := newBusyUI(ws)
	warmCaches(m, false)
	ws.resetCounters()

	_, cmd := m.Update(pubsub.Event[message.Message]{
		Type:    pubsub.CreatedEvent,
		Payload: message.Message{ID: "m1", SessionID: "s1", Role: message.User},
	})
	require.Zero(t, ws.syncProbes(), "the event handler itself must not probe synchronously")
	require.True(t, m.busyFetchInFlight, "CreatedEvent must schedule a busy refresh")
	require.True(t, m.promptQueueInFlight, "CreatedEvent must schedule a queue refresh")

	runCmds(m, cmd)
	require.True(t, m.isAgentBusy(), "refreshed busy state must land in the cache")
	require.Equal(t, 1, m.promptQueue, "refreshed queue count must land in the cache")
	require.False(t, m.busyFetchInFlight)
	require.False(t, m.promptQueueInFlight)
}

// TestAgentTerminalNotificationsRefreshBusy pins the busy→idle edge: the
// agent clears its active request before publishing TypeAgentFinished (and
// TypeAgentError) precisely so observers can re-probe. The handler must
// invalidate the memoized busy state and re-fetch busy + queue.
func TestAgentTerminalNotificationsRefreshBusy(t *testing.T) {
	pinTTLs(t)

	for _, typ := range []notify.Type{notify.TypeAgentFinished, notify.TypeAgentError} {
		t.Run(string(typ), func(t *testing.T) {
			ws := &countingWorkspace{ready: true} // agent now idle
			m := newBusyUI(ws)
			warmCaches(m, true) // stale: still busy
			ws.resetCounters()
			require.True(t, m.isAgentBusy())

			_, cmd := m.Update(pubsub.Event[notify.Notification]{
				Type:    pubsub.CreatedEvent,
				Payload: notify.Notification{Type: typ, SessionID: "s1"},
			})
			require.True(t, m.busyFetchInFlight, "terminal notification must schedule a busy refresh")
			require.True(t, m.promptQueueInFlight, "terminal notification must schedule a queue refresh")

			runCmds(m, cmd)
			require.False(t, m.isAgentBusy(),
				"busy→idle edge must reach the cache without waiting for the TTL")
		})
	}
}

// TestSessionSwitchRefreshesQueueAndBusy: switching sessions must drop the
// previous session's queue pill and memoized busy state and fetch the new
// session's, so esc never offers to clear the wrong queue.
func TestSessionSwitchRefreshesQueueAndBusy(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, queued: []string{"a", "b"}}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 5 // stale queue pill from the previous session
	m.promptQueueItems = []string{"x", "y", "z", "w", "v"}
	ws.resetCounters()

	_, cmd := m.Update(loadSessionMsg{session: &session.Session{ID: "s2"}})
	require.Zero(t, m.promptQueue, "switching sessions must drop the old session's queue pill")
	require.True(t, m.promptQueueInFlight, "session switch must schedule a queue refresh")
	require.True(t, m.busyFetchInFlight, "session switch must schedule a busy refresh")

	runCmds(m, cmd)
	require.Equal(t, 2, m.promptQueue, "the new session's queue must be fetched")
	require.Equal(t, []string{"a", "b"}, m.promptQueueItems)
}

// TestToggleYoloWritesThroughCache: both yolo toggle paths share
// toggleYoloMode, which must write the known new value through the cache —
// no invalidation, no re-probe.
func TestToggleYoloWritesThroughCache(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, yolo: false}
	m := newBusyUI(ws)

	got := m.toggleYoloMode()
	require.True(t, got)
	require.Equal(t, 1, ws.permSetCalls)
	readsAfterToggle := ws.permCalls
	require.Equal(t, 1, readsAfterToggle, "toggle reads the authoritative value exactly once")

	require.True(t, m.yoloModeCached(), "the new value must be served from the cache")
	require.True(t, m.yoloCache.fresh(busyCacheTTL), "write-through must stamp the cache fresh")
	m.yoloModeCached()
	require.Equal(t, readsAfterToggle, ws.permCalls, "reads after the toggle must not re-probe")

	got = m.toggleYoloMode()
	require.False(t, got)
	require.False(t, m.yoloModeCached())
}

// TestLocalYoloToggleSupersedesInFlightProbe pins the generation bump in
// toggleYoloMode: a busy/yolo probe dispatched before the toggle carries the
// old generation. Without advancing busyFetchGen its stale result would land
// with a still-matching generation and clobber the just-toggled value.
func TestLocalYoloToggleSupersedesInFlightProbe(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, yolo: false}
	m := newBusyUI(ws)
	warmCaches(m, false)

	// A busy/yolo probe carrying the pre-toggle generation is in flight.
	m.busyFetchInFlight = true
	staleGen := m.busyFetchGen

	require.True(t, m.toggleYoloMode())
	require.NotEqual(t, staleGen, m.busyFetchGen,
		"toggle must advance the busy generation to supersede in-flight probes")
	require.True(t, m.yoloModeCached(), "toggle must write the new value through the cache")

	// The stale probe (old generation, old yolo=false) lands.
	m.busyFetchInFlight = true
	cmds := m.applyBusyState(busyStateMsg{gen: staleGen, yolo: false})
	require.True(t, m.yoloModeCached(),
		"stale probe must not overwrite the freshly toggled value")
	require.NotEmpty(t, cmds, "stale probe must re-dispatch an authoritative refresh")
	require.True(t, m.busyFetchInFlight, "re-dispatched refresh must be in flight")
}

// TestSendMessageSetsOptimisticBusy pins the esc-after-enter behavior:
// submitting a prompt optimistically marks the agent busy so an immediate
// esc routes to cancelAgent instead of reading a stale idle value and doing
// nothing.
func TestSendMessageSetsOptimisticBusy(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true} // workspace still reports idle
	m := newBusyUI(ws)
	warmCaches(m, false)

	require.False(t, m.isAgentBusy())
	cmd := m.sendMessage("hello") // returned cmds (AgentRun etc.) deliberately not run
	require.NotNil(t, cmd)
	require.True(t, m.isAgentBusy(),
		"sendMessage must optimistically mark the agent busy")

	// esc right after enter: isAgentBusy gates cancelAgent, first press
	// arms the double-press cancel.
	require.Zero(t, m.promptQueue)
	m.cancelAgent()
	require.True(t, m.isCanceling, "first esc press must arm cancellation")

	// Second press must actually cancel.
	m.cancelAgent()
	require.Equal(t, 1, ws.cancelCalls, "second esc press must cancel the agent")
}

// TestCancelAgentPreservesQueue verifies that queued prompts do not change
// Escape's double-press active-task cancellation behavior, and that the UI
// neither discards its cached queue nor asks the workspace to clear the
// agent queue. The agent side of the contract — Cancel leaving the queue
// intact — is pinned by agent.TestCancel_PreservesQueuedPrompts; the stub
// here only models it.
func TestCancelAgentPreservesQueue(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, queued: []string{"a"}}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 1
	m.promptQueueItems = []string{"a"}
	ws.resetCounters()

	cmd := m.cancelAgent()
	require.NotNil(t, cmd, "first esc press must arm the cancellation timer")
	require.True(t, m.isCanceling)
	require.Equal(t, []string{"a"}, ws.queued)
	require.Equal(t, 1, m.promptQueue)
	require.Equal(t, []string{"a"}, m.promptQueueItems)
	require.Zero(t, ws.clearQueueCalls, "esc must not clear queued prompts")
	require.Zero(t, ws.queuedCalls, "esc must not probe the queue")
	require.Zero(t, ws.queueListCalls, "esc must not probe the queue")

	m.cancelAgent()
	require.False(t, m.isCanceling)
	require.Equal(t, 1, ws.cancelCalls, "second esc press must cancel the agent")
	require.Equal(t, []string{"a"}, ws.queued)
	require.Zero(t, ws.clearQueueCalls, "canceling the agent must preserve queued prompts")
}

// cancelHelp returns the description the help panes show for the esc/cancel
// binding, or "" when no cancel binding is offered.
func cancelHelp(t *testing.T, binds []key.Binding) string {
	t.Helper()
	for _, b := range binds {
		if b.Help().Key == "esc" {
			return b.Help().Desc
		}
	}
	return ""
}

// TestCancelHelpNeverAdvertisesClearQueue pins the help text against the
// turn-scoped cancel semantics: esc cancels the active turn and leaves the
// queue alone, so neither help pane may claim it clears the queue just
// because prompts are queued. The armed state still shows the
// double-press hint.
func TestCancelHelpNeverAdvertisesClearQueue(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, agentBusy: true, queued: []string{"a", "b"}}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 2
	m.promptQueueItems = []string{"a", "b"}

	require.Equal(t, "cancel", cancelHelp(t, m.ShortHelp()),
		"short help must not advertise clearing the queue")
	full := m.FullHelp()
	require.NotEmpty(t, full)
	require.Equal(t, "cancel", cancelHelp(t, full[0]),
		"full help must not advertise clearing the queue")

	// First esc press arms cancellation: both panes switch to the
	// double-press hint, still never mentioning the queue.
	m.isCanceling = true
	require.Equal(t, "press again to cancel", cancelHelp(t, m.ShortHelp()))
	full = m.FullHelp()
	require.NotEmpty(t, full)
	require.Equal(t, "press again to cancel", cancelHelp(t, full[0]))
}

// TestBackstopRefreshesStaleCaches: when the memoized state outlives its TTL
// with no event edge, the Update tail schedules exactly one off-thread
// refresh (deduplicated while in flight) and the result lands as a message.
func TestBackstopRefreshesStaleCaches(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, agentBusy: true}
	m := newBusyUI(ws)
	// Caches start at their zero value: stale by definition.

	_, cmd := m.Update(plainMsg{})
	require.True(t, m.busyFetchInFlight, "stale caches must trigger a backstop refresh")
	require.Zero(t, ws.syncProbes(), "the backstop itself must not probe synchronously")

	// A second Update while the fetch is in flight must not stack another.
	before := m.busyFetchInFlight
	m.Update(plainMsg{})
	require.Equal(t, before, m.busyFetchInFlight)
	require.Zero(t, ws.syncProbes())

	runCmds(m, cmd)
	require.False(t, m.busyFetchInFlight)
	require.True(t, m.isAgentBusy(), "the backstop result must land in the cache")
	require.Equal(t, 1, ws.agentBusyCalls, "exactly one probe per backstop refresh")

	// Freshly refreshed caches must not re-dispatch.
	m.Update(plainMsg{})
	require.False(t, m.busyFetchInFlight, "fresh caches must not re-dispatch the backstop")
}

// TestSetSessionMessagesGatesAnimationsOnBusy verifies that reloading a
// session does not start spinner animations when the agent is not busy.
// A session that was killed mid-generation can persist an assistant message
// with no Finish part, which still reports isSpinning() even though nothing
// is running. Starting animations for it would leave a ghost "working"
// spinner after the session is reloaded.
func TestSetSessionMessagesGatesAnimationsOnBusy(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, agentBusy: false}
	m := newBusyUI(ws)
	warmCaches(m, false)

	// A message that looks unfinished (no Finish part, no content).
	msgs := []message.Message{
		{
			ID:        "m1",
			SessionID: "s1",
			Role:      message.Assistant,
			Parts: []message.ContentPart{
				message.ReasoningContent{Thinking: "thinking..."},
			},
		},
	}

	// When the agent is not busy, setSessionMessages must not start animations.
	cmd := m.setSessionMessages(msgs)
	require.Nil(t, cmd, "setSessionMessages must not start animations when agent is idle")

	// When the agent is busy, animations should start.
	warmCaches(m, true)
	cmd = m.setSessionMessages(msgs)
	require.NotNil(t, cmd, "setSessionMessages must start animations when agent is busy")
}

// TestStaleBusyRefreshDiscardedAndReDispatched pins the generation guard for
// busy/permission state: a probe started before a newer state transition
// (here an optimistic busy write) must not overwrite the newer value when it
// lands, and the authoritative refresh must not be lost merely because the
// older probe was in flight — the stale result re-dispatches it.
func TestStaleBusyRefreshDiscardedAndReDispatched(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, false)

	// A busy probe is in flight; capture the generation it was dispatched
	// with, then a newer transition (optimistic send) supersedes it.
	m.busyFetchInFlight = true
	staleGen := m.busyFetchGen
	m.agentBusyCache.set(true) // optimistic busy
	m.busyFetchGen++           // newer state transition

	// The stale probe (agent reported idle) lands with the old generation.
	cmds := m.applyBusyState(busyStateMsg{gen: staleGen, agentBusy: false})
	require.True(t, m.isAgentBusy(),
		"a stale busy result must not overwrite the newer optimistic busy state")
	require.NotEmpty(t, cmds,
		"a stale busy result must re-dispatch the authoritative refresh")
	require.True(t, m.busyFetchInFlight, "the re-dispatched probe must be in flight")

	// The fresh probe (matching generation) is applied normally.
	freshGen := m.busyFetchGen
	m.applyBusyState(busyStateMsg{gen: freshGen, agentBusy: false})
	require.False(t, m.isAgentBusy(), "a current-generation result must land in the cache")
}

// TestStalePromptQueueDiscardedAndReDispatched pins the generation guard for
// the queue: a fetch started before a newer transition (here a queue clear)
// must not repopulate the cleared queue, and it must re-dispatch the
// authoritative fetch instead of being applied.
func TestStalePromptQueueDiscardedAndReDispatched(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, queued: []string{"real"}}
	m := newBusyUI(ws)
	warmCaches(m, false)
	m.promptQueue = 1
	m.promptQueueItems = []string{"real"}

	// A fetch is in flight; capture its generation, then a newer transition
	// (esc clears the queue) supersedes it.
	m.promptQueueInFlight = true
	staleGen := m.promptQueueGen
	m.invalidatePromptQueue()
	m.promptQueue = 0
	m.promptQueueItems = nil

	// The stale fetch (still saw one prompt) lands for the same session.
	cmds := m.applyPromptQueue(promptQueueMsg{
		forSession: "s1",
		gen:        staleGen,
		prompts:    []string{"stale"},
	})
	require.Zero(t, m.promptQueue,
		"a stale queue result must not repopulate the cleared queue")
	require.Empty(t, m.promptQueueItems)
	require.NotEmpty(t, cmds,
		"a stale queue result must re-dispatch the authoritative fetch")
	require.True(t, m.promptQueueInFlight, "the re-dispatched fetch must be in flight")
}

// TestStalePromptQueuePreservesSessionScoping pins that the generation guard
// does not weaken session scoping: a fetch scoped to a different session is
// still discarded and re-fetched even when its generation would otherwise
// match.
func TestStalePromptQueuePreservesSessionScoping(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws) // active session "s1"
	warmCaches(m, false)
	m.promptQueueInFlight = true
	gen := m.promptQueueGen

	cmds := m.applyPromptQueue(promptQueueMsg{
		forSession: "other",
		gen:        gen,
		prompts:    []string{"from other session"},
	})
	require.Zero(t, m.promptQueue,
		"a result from a different session must never populate the queue")
	require.NotEmpty(t, cmds, "a session-mismatched result must re-fetch for the current session")
}

// TestRenderHelpersDoNotProbeWorkspace pins the render-path side of the
// invariant for the model and LSP info: selectedLargeModel, lspInfo, and
// lspErrorCount render from memoized state only. They run on every frame
// (landing view, sidebar, compact header), and the probes behind them
// (AgentIsReady, AgentModel, LSPGetStates, LSPGetDiagnosticCounts) are
// synchronous HTTP round-trips in client/server mode.
func TestRenderHelpersDoNotProbeWorkspace(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	m.agentReady = true
	m.lspStates = map[string]workspace.LSPClientInfo{
		"gopls": {Name: "gopls", State: lsp.StateReady, DiagnosticCount: 3},
	}
	m.lspDiagnostics = map[string]lsp.DiagnosticCounts{
		"gopls": {Error: 2, Warning: 1},
	}

	for range 10 {
		require.NotNil(t, m.selectedLargeModel())
		m.lspInfo(40, 5, true)
		require.Equal(t, 3, m.lspErrorCount())
	}

	// modelInfo reaches provider config only through the memoized model;
	// with the agent not ready it renders the empty state.
	m.agentReady = false
	for range 10 {
		m.modelInfo(40)
	}

	require.Zero(t, ws.syncProbes(), "render helpers must never probe the workspace")
}

// TestBusyRefreshCarriesReadyAndModel: the off-thread busy probe must also
// deliver the coordinator's readiness and selected model so the sidebar and
// landing view render them without per-frame probes.
func TestBusyRefreshCarriesReadyAndModel(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{
		ready: true,
		model: workspace.AgentModel{ModelCfg: config.SelectedModel{Model: "test-model", Provider: "prov"}},
	}
	m := newBusyUI(ws)
	require.Nil(t, m.selectedLargeModel(), "before any probe the model is unknown")

	_, cmd := m.Update(plainMsg{}) // stale caches: the backstop dispatches
	runCmds(m, cmd)

	require.True(t, m.agentReady, "the probe must land readiness in the cache")
	sel := m.selectedLargeModel()
	require.NotNil(t, sel)
	require.Equal(t, "test-model", sel.ModelCfg.Model, "the probe must land the model in the cache")
}

// TestAgentModelChangedRefreshesModel: after a model change
// (selection/thinking/reasoning cmds sequence agentModelChangedCmd), the
// handler must re-fetch ready/model off-thread — no synchronous probe — and
// the fresh model must replace the memoized one.
func TestAgentModelChangedRefreshesModel(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{
		ready: true,
		model: workspace.AgentModel{ModelCfg: config.SelectedModel{Model: "new-model"}},
	}
	m := newBusyUI(ws)
	warmCaches(m, false)
	m.agentModel = workspace.AgentModel{ModelCfg: config.SelectedModel{Model: "old-model"}}
	ws.resetCounters()

	_, cmd := m.Update(agentModelChangedMsg{})
	require.Zero(t, ws.syncProbes(), "the model-change handler must not probe synchronously")
	require.True(t, m.busyFetchInFlight, "a model change must schedule a ready/model refresh")

	runCmds(m, cmd)
	require.Equal(t, "new-model", m.agentModel.ModelCfg.Model,
		"the refreshed model must land in the cache")
}

// TestMCPStateChangedRefreshesModel pins the fourth UpdateAgentModel call
// site: an MCP state change rebuilds the agent, which can change the
// effective model, so the memoized ready/model state must be re-fetched
// off-thread afterwards — the edge the updateAgentModelCmd helper exists to
// make unforgettable.
func TestMCPStateChangedRefreshesModel(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{
		ready: true,
		model: workspace.AgentModel{ModelCfg: config.SelectedModel{Model: "post-mcp-model"}},
	}
	m := newBusyUI(ws)
	warmCaches(m, false)
	m.agentModel = workspace.AgentModel{ModelCfg: config.SelectedModel{Model: "pre-mcp-model"}}
	ws.resetCounters()

	// handleStateChanged sequences the rebuild with agentModelChangedCmd;
	// tea.Sequence's wrapper msg is unexported, so drive the two steps the
	// way the runtime would: run the cmd (the stub records the call), then
	// deliver the invalidation message.
	_ = m.handleStateChanged()()
	_, cmd := m.Update(agentModelChangedMsg{})
	require.True(t, m.busyFetchInFlight, "an MCP state change must schedule a ready/model refresh")
	runCmds(m, cmd)

	require.True(t, m.agentReady)
	require.Equal(t, "post-mcp-model", m.agentModel.ModelCfg.Model,
		"an MCP state change must refresh the memoized model")
}

// TestLSPEventRefreshIsOffThreadAndDeduped pins the LSP side of the
// invariant: an LSP event must not fetch states synchronously in Update
// (LSPGetStates + per-server LSPGetDiagnosticCounts are HTTP round-trips in
// client/server mode, and diagnostics events arrive per edited file). It
// schedules one off-thread fetch, dedups while one is in flight, and
// re-dispatches a queued refresh when the in-flight fetch lands.
func TestLSPEventRefreshIsOffThreadAndDeduped(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{
		ready:     true,
		lspStates: map[string]workspace.LSPClientInfo{"gopls": {Name: "gopls", DiagnosticCount: 3}},
		lspDiags:  map[string]lsp.DiagnosticCounts{"gopls": {Error: 2, Warning: 1}},
	}
	m := newBusyUI(ws)
	warmCaches(m, false)
	ws.resetCounters()

	_, cmd := m.Update(pubsub.Event[workspace.LSPEvent]{
		Payload: workspace.LSPEvent{Type: workspace.LSPEventDiagnosticsChanged, Name: "gopls"},
	})
	require.Zero(t, ws.syncProbes(), "the LSP event handler must not probe synchronously")
	require.True(t, m.lspFetchInFlight, "an LSP event must schedule an off-thread refresh")

	// A second event while the fetch is in flight queues a re-fetch instead
	// of stacking another dispatch.
	m.Update(pubsub.Event[workspace.LSPEvent]{
		Payload: workspace.LSPEvent{Type: workspace.LSPEventDiagnosticsChanged, Name: "gopls"},
	})
	require.Zero(t, ws.syncProbes())
	require.True(t, m.lspRefreshQueued, "an event during an in-flight fetch must queue a re-fetch")

	runCmds(m, cmd)
	require.False(t, m.lspFetchInFlight)
	require.False(t, m.lspRefreshQueued, "the queued flag must clear once the re-dispatched fetch lands")
	require.Equal(t, 3, m.lspStates["gopls"].DiagnosticCount, "fetched states must land in the cache")
	require.Equal(t, 2, m.lspDiagnostics["gopls"].Error, "fetched severity counts must land in the cache")
	require.Equal(t, 3, m.lspErrorCount())
	require.Equal(t, 2, ws.lspStateCalls, "one fetch plus the queued re-fetch")
}

// TestRemoteYoloToggleUpdatesEditorPrompt pins the second fix: when an
// asynchronous busy-state refresh reports a yolo mode different from the
// cached one (a remote toggle), applyBusyState must update the textarea
// prompt function too, not just the cache — otherwise the prompt icon/style
// keeps rendering the old mode.
func TestRemoteYoloToggleUpdatesEditorPrompt(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	m.textarea.Focus()
	m.textarea.SetWidth(40)
	m.yoloCache.set(false)
	m.setEditorPrompt(false)
	normalPrompt := ansi.Strip(m.textarea.View())

	// A remote toggle flips yolo on; delivered via an off-thread refresh.
	m.applyBusyState(busyStateMsg{gen: m.busyFetchGen, yolo: true})
	require.True(t, m.yoloModeCached(), "the refresh must write the new yolo value through the cache")
	yoloPrompt := ansi.Strip(m.textarea.View())
	require.NotEqual(t, normalPrompt, yoloPrompt,
		"a remote yolo toggle must change the rendered editor prompt")
	require.Contains(t, yoloPrompt, "Y",
		"the yolo prompt icon must render after a remote toggle")

	// Flipping back off must restore the normal prompt.
	m.applyBusyState(busyStateMsg{gen: m.busyFetchGen, yolo: false})
	require.False(t, m.yoloModeCached())
	require.Equal(t, normalPrompt, ansi.Strip(m.textarea.View()),
		"toggling yolo off must restore the normal editor prompt")
}

func TestPopQueuedMessageKeysRestoreNewestMessage(t *testing.T) {
	pinTTLs(t)

	for _, tc := range []struct {
		name string
		mod  tea.KeyMod
	}{
		{name: "shift up", mod: tea.ModShift},
		{name: "alt up", mod: tea.ModAlt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attachment := message.Attachment{FileName: "notes.txt", MimeType: "text/plain"}
			ws := &countingWorkspace{
				ready:  true,
				queued: []string{"older", "newest"},
				queuedMessages: []agent.QueuedMessage{
					{Prompt: "older"},
					{Prompt: "newest", Attachments: []message.Attachment{attachment}},
				},
			}
			m := newBusyUI(ws)
			warmCaches(m, true)
			m.promptQueue = 2
			m.promptQueueItems = []string{"older", "newest"}

			_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tc.mod})
			require.Zero(t, ws.popQueueCalls, "pop must run asynchronously")
			runCmds(m, cmd)

			require.Equal(t, 1, ws.popQueueCalls)
			require.Equal(t, "newest", m.textarea.Value())
			require.True(t, m.isAtEditorEnd())
			require.Equal(t, []message.Attachment{attachment}, m.attachments.List())
			require.Equal(t, -1, m.promptHistory.index)
			require.Equal(t, "newest", m.promptHistory.draft)
			require.Equal(t, []string{"older"}, m.promptQueueItems)
			require.Equal(t, 1, m.promptQueue)
			require.Equal(t, []string{"older"}, ws.queued)
		})
	}
}

func TestPopQueuedMessageRejectsNonEmptyInput(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{
		ready:          true,
		queued:         []string{"queued"},
		queuedMessages: []agent.QueuedMessage{{Prompt: "queued"}},
	}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 1
	m.promptQueueItems = []string{"queued"}
	m.textarea.SetValue("draft")

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	runCmds(m, cmd)

	require.Zero(t, ws.popQueueCalls)
	require.Equal(t, "draft", m.textarea.Value())
	require.Equal(t, util.InfoTypeWarn, m.status.msg.Type)
	require.Equal(t, "Can't pop queued message: input field is not empty.", m.status.msg.Msg)
}

// TestPopQueuedMessageKeepsTextTypedWhileInFlight pins the non-destructive
// apply path. The "input field is not empty" guard runs at key-press time,
// but the pop is an HTTP round-trip in client/server mode and the message is
// already gone from the agent queue by the time the result lands — so
// neither the draft nor the popped prompt may be dropped.
func TestPopQueuedMessageKeepsTextTypedWhileInFlight(t *testing.T) {
	pinTTLs(t)

	for _, tc := range []struct {
		name     string
		bangMode bool
		draft    string
		want     string
	}{
		{name: "plain draft", draft: "typed while waiting", want: "typed while waiting\nqueued prompt"},
		// In bang mode the leading "!" is stripped from the value, so
		// re-syncing bang mode from the merged text would silently turn a
		// shell command back into a prompt.
		{name: "bang draft", bangMode: true, draft: "ls -la", want: "ls -la\nqueued prompt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := &countingWorkspace{
				ready:          true,
				queued:         []string{"queued prompt"},
				queuedMessages: []agent.QueuedMessage{{Prompt: "queued prompt"}},
			}
			m := newBusyUI(ws)
			warmCaches(m, true)
			m.promptQueue = 1
			m.promptQueueItems = []string{"queued prompt"}
			m.bangMode = tc.bangMode

			_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
			// The user starts typing before the pop result arrives.
			m.textarea.SetValue(tc.draft)
			runCmds(m, cmd)

			require.Equal(t, 1, ws.popQueueCalls)
			require.Equal(t, tc.want, m.textarea.Value())
			require.True(t, m.isAtEditorEnd())
			require.Equal(t, tc.bangMode, m.bangMode)
			require.Equal(t, -1, m.promptHistory.index)
			require.Equal(t, tc.want, m.promptHistory.draft)
			require.Equal(t, util.InfoTypeWarn, m.status.msg.Type)
			require.Equal(t,
				"Input field was not empty: appended the popped queued message below your text.",
				m.status.msg.Msg)
			require.Empty(t, m.promptQueueItems)
			require.Empty(t, ws.queued)
		})
	}
}

func TestPopQueuedMessageAppendsAttachmentsAndSynchronizesBangMode(t *testing.T) {
	pinTTLs(t)

	existing := message.Attachment{FileName: "existing.txt", MimeType: "text/plain"}
	restored := message.Attachment{FileName: "restored.png", MimeType: "image/png"}
	ws := &countingWorkspace{
		ready:          true,
		queued:         []string{"!echo queued"},
		queuedMessages: []agent.QueuedMessage{{Prompt: "!echo queued", Attachments: []message.Attachment{restored}}},
	}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 1
	m.promptQueueItems = []string{"!echo queued"}
	m.attachments.Update(existing)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModAlt})
	runCmds(m, cmd)

	require.True(t, m.bangMode)
	require.Equal(t, "echo queued", m.textarea.Value())
	require.True(t, m.isAtEditorEnd())
	require.Equal(t, []message.Attachment{existing, restored}, m.attachments.List())
}

// TestPopQueuedMessageSkipsAttachmentsAlreadyOnEditor pins the identity used
// when a restore collides with chips the editor already holds: an attachment
// that is already there byte for byte is not added again (two identical
// chips would send the same file twice), while a same-named attachment
// carrying different bytes is kept, since paste_<n>.txt names are only
// unique within one editor's list and the restore may never drop content.
func TestPopQueuedMessageSkipsAttachmentsAlreadyOnEditor(t *testing.T) {
	pinTTLs(t)

	notes := message.Attachment{
		FilePath: "/tmp/notes.txt",
		FileName: "notes.txt",
		MimeType: "text/plain",
		Content:  []byte("hello"),
	}
	diagram := message.Attachment{
		FilePath: "/tmp/diagram.png",
		FileName: "diagram.png",
		MimeType: "image/png",
		Content:  []byte("png bytes"),
	}
	typedPaste := message.Attachment{
		FilePath: "paste_1.txt",
		FileName: "paste_1.txt",
		MimeType: "text/plain",
		Content:  []byte("typed later"),
	}
	queuedPaste := typedPaste
	queuedPaste.Content = []byte("queued earlier")

	for _, tc := range []struct {
		name     string
		held     message.Attachment
		restored []message.Attachment
		want     []message.Attachment
	}{
		{
			name:     "identical attachment is not duplicated",
			held:     notes,
			restored: []message.Attachment{notes, diagram},
			want:     []message.Attachment{notes, diagram},
		},
		{
			name:     "same name with different bytes is kept",
			held:     typedPaste,
			restored: []message.Attachment{queuedPaste},
			want:     []message.Attachment{typedPaste, queuedPaste},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := &countingWorkspace{
				ready:          true,
				queued:         []string{"queued prompt"},
				queuedMessages: []agent.QueuedMessage{{Prompt: "queued prompt", Attachments: tc.restored}},
			}
			m := newBusyUI(ws)
			warmCaches(m, true)
			m.promptQueue = 1
			m.promptQueueItems = []string{"queued prompt"}
			m.attachments.Update(tc.held)

			_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
			runCmds(m, cmd)

			require.Equal(t, "queued prompt", m.textarea.Value())
			require.Equal(t, tc.want, m.attachments.List())
		})
	}
}

// TestPopQueuedMessageEmptyQueueIsAnsweredFromCache pins that a pop with
// nothing queued costs no workspace round-trip (it is an HTTP POST in
// client/server mode, and the memoized count already answers it) and still
// tells the user why the editor stayed empty, so the chord does not read as
// a broken binding.
func TestPopQueuedMessageEmptyQueueIsAnsweredFromCache(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 0
	m.promptQueueItems = nil
	gen := m.promptQueueGen

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	runCmds(m, cmd)

	require.Zero(t, ws.popQueueCalls, "the memoized count must answer this without a round-trip")
	require.Equal(t, util.InfoTypeInfo, m.status.msg.Type)
	require.Equal(t, "No queued messages.", m.status.msg.Msg)
	require.Empty(t, m.textarea.Value())
	require.Empty(t, m.attachments.List())
	require.Equal(t, gen, m.promptQueueGen)
	require.False(t, m.queuedPopInFlight, "a short-circuited pop must not mark itself in flight")
}

// TestPopQueuedMessageStaleCountReportsEmptyQueue pins the other half: the
// memoized count is the gate, so it can be one round-trip stale (the agent
// dequeued the last prompt meanwhile). The server's "not found" stays the
// authority — it must report the empty queue and supersede the wrong count
// rather than leave the pill promising a message that is gone.
func TestPopQueuedMessageStaleCountReportsEmptyQueue(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 1
	m.promptQueueItems = []string{"already dequeued"}
	staleGen := m.promptQueueGen

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	runCmds(m, cmd)

	require.Equal(t, 1, ws.popQueueCalls)
	require.Equal(t, util.InfoTypeInfo, m.status.msg.Type)
	require.Equal(t, "No queued messages.", m.status.msg.Msg)
	require.Empty(t, m.textarea.Value())
	require.Greater(t, m.promptQueueGen, staleGen,
		"a queue the server says is empty must supersede the memoized one")
	require.Zero(t, m.promptQueue, "the re-fetch must drop the phantom queue pill")
	require.Empty(t, m.promptQueueItems)
	require.False(t, m.queuedPopInFlight, "an empty-queue result must clear the in-flight mark")
}

// TestPopQueuedMessageEmptyResultAfterSessionSwitchKeepsCurrentQueue: a pop
// that found nothing says nothing about the session the user switched to, so
// it must not invalidate or empty that session's queue.
func TestPopQueuedMessageEmptyResultAfterSessionSwitchKeepsCurrentQueue(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, queued: []string{"a"}}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 1
	m.promptQueueItems = []string{"a"}
	gen := m.promptQueueGen

	require.Nil(t, m.applyQueuedMessagePop(queuedMessagePoppedMsg{forSession: "s0"}))
	require.Equal(t, []string{"a"}, m.promptQueueItems)
	require.Equal(t, 1, m.promptQueue)
	require.Equal(t, gen, m.promptQueueGen)
	require.Zero(t, ws.queueListCalls, "another session's empty pop must not re-fetch this queue")
}

// helpDesc returns the description the help panes show for the binding
// rendered under helpKey, or "" when no such binding is offered.
func helpDesc(groups [][]key.Binding, helpKey string) string {
	for _, group := range groups {
		for _, b := range group {
			if b.Help().Key == helpKey {
				return b.Help().Desc
			}
		}
	}
	return ""
}

// TestPopQueuedMessageHelpListsBindingWhenQueued pins the discoverability of
// the pop binding: an unlisted chord is the feature's only entry point, so
// the full help pane must name it whenever prompts are queued in editor
// focus — including while a draft is in the editor, which is exactly when a
// user wonders how to get back to a queued prompt. It must stay out of chat
// focus, where the same chord scrolls the conversation, and out of the
// status line, which is already 113 columns wide: another entry there would
// push "ctrl+g more" (the way to reach the full pane) off a 120- or
// 140-column terminal.
func TestPopQueuedMessageHelpListsBindingWhenQueued(t *testing.T) {
	pinTTLs(t)

	const desc = "edit last queued message"

	ws := &countingWorkspace{ready: true, queued: []string{"older", "newest"}}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 2
	m.promptQueueItems = []string{"older", "newest"}

	require.Equal(t, desc, helpDesc(m.FullHelp(), "shift+↑"),
		"full help must list the pop binding while prompts are queued")
	require.Empty(t, helpDesc([][]key.Binding{m.ShortHelp()}, "shift+↑"),
		"the status line must stay short enough to keep its own tail")

	m.textarea.SetValue("draft")
	require.Equal(t, desc, helpDesc(m.FullHelp(), "shift+↑"),
		"a draft must not hide the binding: it is how the user recovers one")
	m.textarea.SetValue("")

	// Chat focus: the editor never sees the key press there.
	m.focus = uiFocusMain
	require.NotEqual(t, desc, helpDesc(m.FullHelp(), "shift+↑"),
		"full help must not list the editor pop in chat focus")
	m.focus = uiFocusEditor

	// Nothing queued: nothing to pop.
	m.promptQueue = 0
	m.promptQueueItems = nil
	require.Empty(t, helpDesc(m.FullHelp(), "shift+↑"),
		"full help must not list the pop with an empty queue")
}

// TestPopQueuedMessageIsSingleFlight pins the guard against key autorepeat:
// the pop is destructive at the agent layer and nothing in the model changes
// until its result lands, so a second dispatch would remove a second message
// that the apply path then overwrites in the editor — losing it for good.
func TestPopQueuedMessageIsSingleFlight(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{
		ready:          true,
		queued:         []string{"older", "newest"},
		queuedMessages: []agent.QueuedMessage{{Prompt: "older"}, {Prompt: "newest"}},
	}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 2
	m.promptQueueItems = []string{"older", "newest"}

	_, first := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	require.True(t, m.queuedPopInFlight, "the dispatched pop must be marked in flight")
	_, second := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	runCmds(m, first)
	runCmds(m, second)

	require.Equal(t, 1, ws.popQueueCalls, "autorepeat must not pop twice")
	require.False(t, m.queuedPopInFlight, "the result must clear the in-flight mark")
	require.Equal(t, "newest", m.textarea.Value())
	require.Equal(t, []string{"older"}, m.promptQueueItems)
	require.Equal(t, 1, m.promptQueue)
	require.Equal(t, []string{"older"}, ws.queued)

	// A later press is allowed once the first pop settled.
	m.textarea.SetValue("")
	_, third := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	runCmds(m, third)

	require.Equal(t, 2, ws.popQueueCalls)
	require.Equal(t, "older", m.textarea.Value())
	require.Empty(t, ws.queued)
}

// TestPopQueuedMessageReportsWorkspaceError pins the transport-failure path:
// the error may be raised after the server already removed the message
// (response read/decode failure, dropped connection), so the queue state is
// unknown from here — it must be re-fetched, and the banner must not imply
// the message is still queued.
func TestPopQueuedMessageReportsWorkspaceError(t *testing.T) {
	pinTTLs(t)

	// The server popped "newest" and then the response was lost.
	ws := &countingWorkspace{ready: true, popErr: errors.New("pop failed"), queued: []string{"older"}}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 2
	m.promptQueueItems = []string{"older", "newest"}
	staleGen := m.promptQueueGen

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModAlt})
	runCmds(m, cmd)

	require.Equal(t, 1, ws.popQueueCalls)
	require.Equal(t, util.InfoTypeError, m.status.msg.Type)
	require.Equal(t, "pop failed (it may have left the queue anyway)", m.status.msg.Msg)
	require.False(t, m.queuedPopInFlight,
		"a failed pop must clear the in-flight mark so the key is not wedged")
	require.Greater(t, m.promptQueueGen, staleGen,
		"an unknown queue state must supersede the memoized one")
	require.Equal(t, []string{"older"}, m.promptQueueItems,
		"the re-fetch must replace the queue the pop may have shortened")
	require.Equal(t, 1, m.promptQueue)
}

// TestPopQueuedMessageParksResultAfterSessionSwitch pins the session-switch
// race: the pop is destructive at the agent layer and queued prompts live
// nowhere else (they are not persisted until they run), so a result that
// lands after a session switch may not be dropped. It also may not be
// restored into the current session's editor — the prompt was written for
// another session. It is parked and handed back on return.
func TestPopQueuedMessageParksResultAfterSessionSwitch(t *testing.T) {
	pinTTLs(t)

	restored := message.Attachment{FileName: "restored.png", MimeType: "image/png"}
	ws := &countingWorkspace{
		ready:  true,
		queued: []string{"older", "newest"},
		queuedMessages: []agent.QueuedMessage{
			{Prompt: "older"},
			{Prompt: "newest", Attachments: []message.Attachment{restored}},
		},
	}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 2
	m.promptQueueItems = []string{"older", "newest"}

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	// The user switches sessions while the pop is in flight.
	m.session = &session.Session{ID: "s2"}
	runCmds(m, cmd)

	require.Equal(t, 1, ws.popQueueCalls)
	require.Empty(t, m.textarea.Value(),
		"another session's prompt must not land in the current editor")
	require.Empty(t, m.attachments.List())
	require.Equal(t, util.InfoTypeWarn, m.status.msg.Type)
	require.Equal(t,
		"Session changed: the popped message will be restored when you return to that session.",
		m.status.msg.Msg)
	require.Equal(t,
		agent.QueuedMessage{Prompt: "newest", Attachments: []message.Attachment{restored}},
		m.queuedPopOrphans["s1"],
		"the popped message must be parked for the session it came from")

	_, cmd = m.Update(loadSessionMsg{session: &session.Session{ID: "s1"}})
	runCmds(m, cmd)

	require.Equal(t, "newest", m.textarea.Value(), "returning must hand the message back")
	require.True(t, m.isAtEditorEnd())
	require.Equal(t, []message.Attachment{restored}, m.attachments.List())
	require.Equal(t, util.InfoTypeWarn, m.status.msg.Type)
	require.Equal(t,
		"Restored the queued message you popped before switching sessions.",
		m.status.msg.Msg)
	require.Empty(t, m.queuedPopOrphans, "a restored message must not be parked twice")
}

// TestParkedPopRestoreKeepsEditorDraft pins that the parked restore is
// non-destructive: the editor is shared across sessions, so text typed while
// the message was parked must survive it coming back.
func TestParkedPopRestoreKeepsEditorDraft(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.session = &session.Session{ID: "s2"}
	m.queuedPopOrphans = map[string]agent.QueuedMessage{"s1": {Prompt: "newest"}}
	m.textarea.SetValue("draft")

	_, cmd := m.Update(loadSessionMsg{session: &session.Session{ID: "s1"}})
	runCmds(m, cmd)

	require.Equal(t, "draft\nnewest", m.textarea.Value())
	require.True(t, m.isAtEditorEnd())
	require.Equal(t, util.InfoTypeWarn, m.status.msg.Type)
	require.Equal(t,
		"Appended the queued message you popped before switching sessions below your text.",
		m.status.msg.Msg)
	require.Empty(t, m.queuedPopOrphans)
}

func TestPopQueuedMessageSupersedesInFlightQueueRefresh(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{
		ready:          true,
		queued:         []string{"older", "newest"},
		queuedMessages: []agent.QueuedMessage{{Prompt: "older"}, {Prompt: "newest"}},
	}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 2
	m.promptQueueItems = []string{"older", "newest"}
	m.promptQueueInFlight = true
	staleGen := m.promptQueueGen

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	runCmds(m, cmd)

	require.Equal(t, []string{"older"}, m.promptQueueItems)
	require.Greater(t, m.promptQueueGen, staleGen)
	cmds := m.applyPromptQueue(promptQueueMsg{
		forSession: "s1",
		gen:        staleGen,
		prompts:    []string{"older", "newest"},
	})
	require.Equal(t, []string{"older"}, m.promptQueueItems)
	require.NotEmpty(t, cmds)
}

// actionDialog is a dialog stub that answers every message with a fixed
// action, standing in for the commands dialog so a command action can be
// driven through the real overlay routing (Overlay.Update -> handleDialogMsg)
// without building the command list.
type actionDialog struct {
	id     string
	action dialog.Action
}

func (d *actionDialog) ID() string                      { return d.id }
func (d *actionDialog) HandleMsg(tea.Msg) dialog.Action { return d.action }

func (d *actionDialog) Draw(uv.Screen, uv.Rectangle) *tea.Cursor { return nil }

// TestClearQueueCommandDiscardsQueue pins the commands-dialog escape
// hatch, which discards the queue regardless of whether the agent is busy
// (idle esc only covers the stopped case, and shift+up pops one message at
// a time). The clear is a synchronous HTTP round-trip in client/server
// mode, so Update must dispatch it rather than call it.
func TestClearQueueCommandDiscardsQueue(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, queued: []string{"a", "b"}}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 2
	m.promptQueueItems = []string{"a", "b"}
	staleGen := m.promptQueueGen
	m.dialog.OpenDialog(&actionDialog{id: dialog.CommandsID, action: dialog.ActionClearQueue{}})

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Zero(t, ws.clearQueueCalls, "the clear must run off the Update goroutine")
	require.False(t, m.dialog.ContainsDialog(dialog.CommandsID),
		"selecting the command must close the dialog")

	runCmds(m, cmd)

	require.Equal(t, 1, ws.clearQueueCalls)
	require.Empty(t, ws.queued, "the agent queue must be discarded")
	require.Empty(t, m.promptQueueItems)
	require.Zero(t, m.promptQueue)
	require.Greater(t, m.promptQueueGen, staleGen,
		"the clear must supersede queue reads started before it")
	require.Equal(t, 1, ws.queueListCalls,
		"the emptied queue must be confirmed by one authoritative re-fetch")
	require.Equal(t, util.InfoTypeInfo, m.status.msg.Type)
	require.Equal(t, "Queued messages cleared.", m.status.msg.Msg)
}

// TestClearQueueResultAfterSessionSwitchKeepsCurrentQueue: the clear is
// dispatched for one session and its result lands one round-trip later, by
// which time the user may have switched — it says nothing about the queue now
// on screen, so it must not empty it.
func TestClearQueueResultAfterSessionSwitchKeepsCurrentQueue(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, queued: []string{"a"}}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 1
	m.promptQueueItems = []string{"a"}

	require.Nil(t, m.applyPromptQueueCleared(promptQueueClearedMsg{forSession: "s0"}))
	require.Equal(t, []string{"a"}, m.promptQueueItems,
		"another session's clear must not empty this session's queue")
	require.Equal(t, 1, m.promptQueue)
}

// TestIdleEscapeClearsQueue covers the state a cancel leaves behind:
// cancellation is turn-scoped, so stopping the agent keeps the queue, and
// the queue then has no owner — the next submission would drag it along.
// Once the agent is idle, esc is therefore the bulk discard for it. The
// clear is an HTTP round-trip in client/server mode, so Update must
// dispatch it, and nothing may arm the double-press cancel: there is no
// turn to stop.
func TestIdleEscapeClearsQueue(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, queued: []string{"a", "b"}}
	m := newBusyUI(ws)
	warmCaches(m, false)
	m.promptQueue = 2
	m.promptQueueItems = []string{"a", "b"}
	ws.resetCounters()

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Zero(t, ws.clearQueueCalls, "the clear must run off the Update goroutine")
	require.False(t, m.isCanceling, "esc must not arm cancellation when the agent is idle")
	require.Zero(t, ws.cancelCalls, "there is no turn to cancel")

	runCmds(m, cmd)

	require.Equal(t, 1, ws.clearQueueCalls)
	require.Empty(t, ws.queued, "the agent queue must be discarded")
	require.Empty(t, m.promptQueueItems)
	require.Zero(t, m.promptQueue)
	require.Equal(t, util.InfoTypeInfo, m.status.msg.Type)
	require.Equal(t, "Queued messages cleared.", m.status.msg.Msg)
}

// TestIdleEscapeWithoutQueueClearsNothing keeps esc inert on an idle
// session with nothing queued: the binding acts only when the UI is
// showing a queue to discard.
func TestIdleEscapeWithoutQueueClearsNothing(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, false)
	ws.resetCounters()

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	runCmds(m, cmd)

	require.Zero(t, ws.clearQueueCalls)
	require.Zero(t, ws.cancelCalls)
	require.False(t, m.isCanceling)
}

// TestBusyEscapeCancelsAndKeepsQueue pins the precedence: while the agent
// is busy, esc still means cancel-the-turn (double press) and must not
// discard the queue — that is the whole point of the turn-scoped cancel.
func TestBusyEscapeCancelsAndKeepsQueue(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, agentBusy: true, queued: []string{"a"}}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 1
	m.promptQueueItems = []string{"a"}
	ws.resetCounters()

	// The first press only arms the double-press window; the command it
	// returns is the arming timer, deliberately not run here (it blocks for
	// the whole window before emitting its expiry).
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.NotNil(t, cmd, "the first esc press must return the arming timer")
	require.True(t, m.isCanceling, "the first esc press must arm cancellation")
	require.Zero(t, ws.clearQueueCalls, "canceling must preserve the queue")
	require.Equal(t, []string{"a"}, m.promptQueueItems)

	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	runCmds(m, cmd)
	require.Equal(t, 1, ws.cancelCalls, "the second esc press must cancel the agent")
	require.Zero(t, ws.clearQueueCalls, "canceling must preserve the queue")
	require.Equal(t, []string{"a"}, m.promptQueueItems)
}

// TestIdleEscapeDuringHistoryNavigationKeepsQueue: esc keeps its more
// local meaning while the user is browsing prompt history — it restores
// the draft — so a queue must not be discarded by the same press.
func TestIdleEscapeDuringHistoryNavigationKeepsQueue(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, queued: []string{"a"}}
	m := newBusyUI(ws)
	warmCaches(m, false)
	m.promptQueue = 1
	m.promptQueueItems = []string{"a"}
	m.promptHistory.messages = []string{"older prompt"}
	m.promptHistory.index = 0
	m.promptHistory.draft = "draft"
	m.textarea.SetValue("older prompt")
	ws.resetCounters()

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	runCmds(m, cmd)

	require.Zero(t, ws.clearQueueCalls, "esc must leave history navigation first")
	require.Equal(t, []string{"a"}, m.promptQueueItems)
	require.Equal(t, "draft", m.textarea.Value(), "esc must restore the draft")
	require.Equal(t, -1, m.promptHistory.index)

	// A second press, now that history navigation is over, clears.
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	runCmds(m, cmd)
	require.Equal(t, 1, ws.clearQueueCalls)
	require.Empty(t, m.promptQueueItems)
}
