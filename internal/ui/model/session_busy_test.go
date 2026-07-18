package model

import (
	"context"
	"testing"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/workspace"
)

// countingWorkspace is a workspace.Workspace stub that counts every probe
// that is a synchronous HTTP round-trip in client/server mode, split per
// method so tests can pin exactly which probes ran. The embedded interface
// panics on anything unimplemented.
type countingWorkspace struct {
	workspace.Workspace

	ready     bool
	agentBusy bool
	yolo      bool
	queued    []string

	readyCalls      int
	agentBusyCalls  int
	queuedCalls     int
	queueListCalls  int
	permCalls       int
	permSetCalls    int
	clearQueueCalls int
	cancelCalls     int
}

func (w *countingWorkspace) AgentIsReady() bool { w.readyCalls++; return w.ready }
func (w *countingWorkspace) AgentIsBusy() bool  { w.agentBusyCalls++; return w.agentBusy }

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
func (w *countingWorkspace) AgentCancel(string)     { w.cancelCalls++ }

func (w *countingWorkspace) ListMessages(context.Context, string) ([]message.Message, error) {
	return nil, nil
}

func (w *countingWorkspace) ListUserMessages(context.Context, string) ([]message.Message, error) {
	return nil, nil
}

func (w *countingWorkspace) LSPStart(context.Context, string) {}

func (w *countingWorkspace) Config() *config.Config { return nil }

// syncProbes sums every synchronous counter; Update/View must keep this at
// zero — the busy/queue invariant is that no workspace call ever happens on
// the Update goroutine.
func (w *countingWorkspace) syncProbes() int {
	return w.readyCalls + w.agentBusyCalls +
		w.queuedCalls + w.queueListCalls + w.permCalls
}

func (w *countingWorkspace) resetCounters() {
	w.readyCalls, w.agentBusyCalls = 0, 0
	w.queuedCalls, w.queueListCalls, w.permCalls = 0, 0, 0
	w.permSetCalls, w.clearQueueCalls, w.cancelCalls = 0, 0, 0
}

// newBusyUI builds a UI wired to the stub workspace with an active session
// "s1", enough state for Update to run end to end.
func newBusyUI(ws *countingWorkspace) *UI {
	com := common.DefaultCommon(ws)
	return &UI{
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
}

// pinTTLs makes the TTL backstop inert for the duration of the test so
// assertions about event-driven refreshes cannot flake by straddling a TTL
// boundary (the tests using it must not call t.Parallel).
func pinTTLs(t *testing.T) {
	t.Helper()
	oldBusy, oldQueue := busyCacheTTL, promptQueueTTL
	busyCacheTTL = time.Hour
	promptQueueTTL = time.Hour
	t.Cleanup(func() { busyCacheTTL, promptQueueTTL = oldBusy, oldQueue })
}

// warmCaches marks all memoized workspace state fresh so only explicit
// invalidation (not startup staleness) can trigger refresh dispatches.
func warmCaches(m *UI, busy bool) {
	m.agentBusyCache.set(busy)
	m.yoloCache.set(false)
	m.promptQueueCheckedAt = time.Now()
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
	case busyStateMsg, promptQueueMsg, agentRunSubmittedMsg:
		_, next := m.Update(msg)
		runCmds(m, next)
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

// TestCancelAgentClearsQueueFromCachedCount: the queue-clear decision must
// come from the memoized count — no synchronous AgentQueuedPrompts probe —
// and clearing must zero the cached count immediately.
func TestCancelAgentClearsQueueFromCachedCount(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, queued: []string{"a"}}
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.promptQueue = 1
	m.promptQueueItems = []string{"a"}
	ws.resetCounters()

	cmd := m.cancelAgent()
	require.Nil(t, cmd)
	require.Equal(t, 1, ws.clearQueueCalls, "esc with a queue must clear it")
	require.Zero(t, ws.queuedCalls, "the decision must use the cached count, not a probe")
	require.Zero(t, ws.queueListCalls, "the decision must use the cached count, not a probe")
	require.Zero(t, m.promptQueue, "the cached count must be zeroed immediately")
	require.Empty(t, m.promptQueueItems)
	require.False(t, m.isCanceling, "clearing the queue must not arm cancellation")
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
