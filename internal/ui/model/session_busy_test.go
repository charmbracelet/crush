package model

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/completions"
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
	model          workspace.AgentModel
	lspStates      map[string]workspace.LSPClientInfo
	lspDiags       map[string]lsp.DiagnosticCounts
	mcpStates      map[string]mcp.ClientInfo
	mcpPendingAuth []mcp.PendingAuthServer
	sessions       []session.Session
	children       []session.Session // Sub-agent sessions: GetSession only.
	sessionStatus  map[string]workspace.SessionStatus
	statusFails    bool            // Probes fail: the zero (unknown) status, as ClientWorkspace returns.
	listErr        error           // ListSessions fails with it.
	busySessions   map[string]bool // Per-session runs; nil means agentBusy.
	cfg            *config.Config
	acceptEdits    map[string]bool // Per-session accept-edits.
	acceptSetErr   error           // PermissionSetAcceptEdits fails with it.
	shellErr       error           // AgentRunShellCommand fails with it.
	skipSetErr     error           // PermissionSetSkipRequests fails with it.
	skipGetErr     error           // PermissionSkipRequests fails with it (value still returned).

	readyCalls      int
	agentBusyCalls  int
	queuedCalls     int
	queueListCalls  int
	permCalls       int
	permSetCalls    int
	acceptCalls     int // PermissionAcceptEdits probes.
	acceptSetCalls  int
	clearQueueCalls int
	cancelCalls     int
	summarizeCalls  int
	modelCalls      int
	lspStateCalls   int
	lspDiagCalls    int
	mcpStateCalls   int
	mcpPendingCalls int
	listCalls       int
	statusCalls     int
	getCalls        int
	lspStopCalls    int
	listHadDeadline bool
	listDeadline    time.Time

	// prefModelScopes records the scope of every UpdatePreferredModel call.
	prefModelScopes []config.Scope
}

func (w *countingWorkspace) UpdatePreferredModel(scope config.Scope, _ config.SelectedModelType, _ config.SelectedModel) error {
	w.prefModelScopes = append(w.prefModelScopes, scope)
	return nil
}

func (w *countingWorkspace) GetDefaultSmallModel(providerID string) config.SelectedModel {
	return config.SelectedModel{Provider: providerID, Model: "small"}
}

func (w *countingWorkspace) GetSession(_ context.Context, id string) (session.Session, error) {
	w.getCalls++
	for _, s := range slices.Concat(w.sessions, w.children) {
		if s.ID == id {
			return s, nil
		}
	}
	return session.Session{}, errors.New("session not found")
}

// ParseAgentToolSessionID mirrors the agent tool's "<message>$$<tool call>"
// session IDs. It is string parsing, not a probe, so it is not counted.
func (w *countingWorkspace) ParseAgentToolSessionID(id string) (string, string, bool) {
	return strings.Cut(id, "$$")
}

func (w *countingWorkspace) ListSessions(ctx context.Context) ([]session.Session, error) {
	w.listCalls++
	w.listDeadline, w.listHadDeadline = ctx.Deadline()
	if w.listErr != nil {
		return nil, w.listErr
	}
	return w.sessions, nil
}

func (w *countingWorkspace) AgentSessionStatus(id string) workspace.SessionStatus {
	w.statusCalls++
	if w.statusFails {
		return workspace.SessionStatus{}
	}
	st := w.sessionStatus[id]
	st.Known = true
	return st
}

func (w *countingWorkspace) AgentIsReady() bool { w.readyCalls++; return w.ready }
func (w *countingWorkspace) AgentIsBusy() bool  { w.agentBusyCalls++; return w.agentBusy }

// AgentIsSessionBusy answers from busySessions when set, else for the single
// session most tests use. It is deliberately uncounted: it is asked once on
// session load rather than on the per-keystroke paths whose probe counts the
// other stubs pin.
func (w *countingWorkspace) AgentIsSessionBusy(id string) bool {
	if w.busySessions != nil {
		return w.busySessions[id]
	}
	return w.agentBusy
}

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

func (w *countingWorkspace) PermissionSkipRequests() (bool, error) {
	w.permCalls++
	return w.yolo, w.skipGetErr
}

func (w *countingWorkspace) PermissionSetSkipRequests(skip bool) error {
	w.permSetCalls++
	if w.skipSetErr != nil {
		return w.skipSetErr
	}
	w.yolo = skip
	return nil
}

func (w *countingWorkspace) AgentRunShellCommand(context.Context, string, string, int, func(string), bool) (proto.ShellCommandResponse, error) {
	return proto.ShellCommandResponse{}, w.shellErr
}

func (w *countingWorkspace) PermissionAcceptEdits(id string) bool {
	w.acceptCalls++
	return w.acceptEdits[id]
}

func (w *countingWorkspace) PermissionSetAcceptEdits(id string, on bool) error {
	w.acceptSetCalls++
	if w.acceptSetErr != nil {
		return w.acceptSetErr
	}
	if w.acceptEdits == nil {
		w.acceptEdits = map[string]bool{}
	}
	w.acceptEdits[id] = on
	return nil
}

func (w *countingWorkspace) AgentSetMain(string) error { return nil }

func (w *countingWorkspace) AgentClearQueue(string) { w.clearQueueCalls++; w.queued = nil }
func (w *countingWorkspace) AgentCancel(string)     { w.cancelCalls++ }

func (w *countingWorkspace) AgentSummarize(context.Context, string) error {
	w.summarizeCalls++
	return nil
}

// CreateSession hands out the session a prompt sent from the landing page
// starts.
func (w *countingWorkspace) CreateSession(context.Context, string) (session.Session, error) {
	return session.Session{ID: "new"}, nil
}

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

func (w *countingWorkspace) MCPGetStates() map[string]mcp.ClientInfo {
	w.mcpStateCalls++
	return w.mcpStates
}

func (w *countingWorkspace) ListMCPPrompts(context.Context) ([]commands.MCPPrompt, error) {
	return nil, nil
}

func (w *countingWorkspace) MCPPendingAuth() []mcp.PendingAuthServer {
	w.mcpPendingCalls++
	return w.mcpPendingAuth
}

func (w *countingWorkspace) MCPAuthURL(string) string { return "" }

func (w *countingWorkspace) MCPServersDisabled(context.Context) ([]string, error) {
	return nil, nil
}

func (w *countingWorkspace) MCPServersEnabled(context.Context) ([]string, error) {
	return nil, nil
}

func (w *countingWorkspace) ListMessages(context.Context, string) ([]message.Message, error) {
	return nil, nil
}

func (w *countingWorkspace) ListUserMessages(context.Context, string) ([]message.Message, error) {
	return nil, nil
}

func (w *countingWorkspace) WorkingDir() string { return "" }

func (w *countingWorkspace) LSPStart(context.Context, string) {}

func (w *countingWorkspace) LSPStopAll(context.Context) { w.lspStopCalls++ }

func (w *countingWorkspace) ListAllUserMessages(context.Context) ([]message.Message, error) {
	return nil, nil
}

func (w *countingWorkspace) SetCurrentSession(context.Context, string) error { return nil }

func (w *countingWorkspace) Config() *config.Config { return w.cfg }

// syncProbes sums every synchronous counter; Update/View must keep this at
// zero — the invariant is that no workspace call ever happens on the Update
// goroutine (which is also the render loop).
func (w *countingWorkspace) syncProbes() int {
	return w.readyCalls + w.agentBusyCalls +
		w.queuedCalls + w.queueListCalls + w.permCalls +
		w.modelCalls + w.lspStateCalls + w.lspDiagCalls +
		w.mcpStateCalls + w.listCalls + w.statusCalls + w.getCalls +
		w.acceptCalls + w.acceptSetCalls
}

func (w *countingWorkspace) resetCounters() {
	w.readyCalls, w.agentBusyCalls = 0, 0
	w.queuedCalls, w.queueListCalls, w.permCalls = 0, 0, 0
	w.permSetCalls, w.clearQueueCalls, w.cancelCalls = 0, 0, 0
	w.modelCalls, w.lspStateCalls, w.lspDiagCalls = 0, 0, 0
	w.mcpStateCalls, w.mcpPendingCalls = 0, 0
	w.listCalls, w.statusCalls, w.getCalls = 0, 0, 0
	w.acceptCalls, w.acceptSetCalls = 0, 0
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
		completions: completions.New(com.Styles.Completions.Normal, com.Styles.Completions.Focused, com.Styles.Completions.Match),
	}
}

// pinTTLs makes the TTL backstop inert for the duration of the test so
// assertions about event-driven refreshes cannot flake by straddling a TTL
// boundary (the tests using it must not call t.Parallel).
func pinTTLs(t *testing.T) {
	t.Helper()
	oldBusy, oldQueue, oldLSP, oldMCP, oldList := busyCacheTTL, promptQueueTTL, lspStatesTTL, mcpStatesTTL, sessionListTTL
	busyCacheTTL = time.Hour
	promptQueueTTL = time.Hour
	lspStatesTTL = time.Hour
	mcpStatesTTL = time.Hour
	sessionListTTL = time.Hour
	t.Cleanup(func() {
		busyCacheTTL, promptQueueTTL, lspStatesTTL, mcpStatesTTL, sessionListTTL = oldBusy, oldQueue, oldLSP, oldMCP, oldList
	})
}

// warmCaches marks all memoized workspace state fresh so only explicit
// invalidation (not startup staleness) can trigger refresh dispatches.
func warmCaches(m *UI, busy bool) {
	m.agentBusyCache.set(busy)
	m.yoloCache.set(false)
	m.agentReady = true
	m.promptQueueCheckedAt = time.Now()
	m.lspCheckedAt = time.Now()
	m.mcpCheckedAt = time.Now()
	m.sessionListCheckedAt = time.Now()
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
	case busyStateMsg, promptQueueMsg, agentRunSubmittedMsg, lspStatesMsg, agentModelChangedMsg, mcpStateChangedMsg, sessionListMsg, childParentMsg, promptRootMsg:
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

// TestToggleYoloKeepsModeWhenWriteFails: if the workspace write fails (client
// timeout against a hung server), the mode, cache and server value stay as
// they were and Ctrl+Y reports the error instead of a false "enabled" banner.
func TestToggleYoloKeepsModeWhenWriteFails(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, yolo: false, skipSetErr: errors.New("context deadline exceeded")}
	m := newBusyUI(ws)
	m.yoloCache.set(false)

	infos := runModeCmds(m, m.handleKeyPressMsg(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl}))
	require.False(t, ws.yolo)
	require.False(t, m.yoloModeCached(), "a failed write must not flip the cache")
	require.Len(t, infos, 1)
	require.Equal(t, util.InfoTypeError, infos[0].Type)
	require.Contains(t, infos[0].Msg, "context deadline exceeded")

	require.False(t, m.toggleYoloMode(), "the other toggle paths keep the mode too")
	require.False(t, m.yoloModeCached())
}

// TestYoloReadErrorFailsClosed: YOLO is on but the read times out. Ctrl+Y and
// Shift+Tab must neither write nor flip the UI to "off"; the error shows.
func TestYoloReadErrorFailsClosed(t *testing.T) {
	pinTTLs(t)
	readErr := errors.New("context deadline exceeded")
	for name, press := range map[string]tea.KeyPressMsg{
		"ctrl+y":    {Code: 'y', Mod: tea.ModCtrl},
		"shift+tab": {Code: tea.KeyTab, Mod: tea.ModShift},
	} {
		t.Run(name, func(t *testing.T) {
			ws := &countingWorkspace{ready: true, yolo: true}
			m := newBusyUI(ws)
			m.mode = uiInputModeCode
			m.yoloCache.set(true)
			ws.skipGetErr = readErr

			infos := runModeCmds(m, m.handleKeyPressMsg(press))
			require.Zero(t, ws.permSetCalls, "no write on an unknown mode")
			require.True(t, ws.yolo)
			require.True(t, m.yoloModeCached(), "the UI keeps showing YOLO")
			require.Len(t, infos, 1)
			require.Equal(t, util.InfoTypeError, infos[0].Type)
			require.Contains(t, infos[0].Msg, "context deadline exceeded")
		})
	}
}

// TestBusyProbeYoloReadErrorKeepsCachedValue: a failed YOLO read in the busy
// probe is unknown, not off.
func TestBusyProbeYoloReadErrorKeepsCachedValue(t *testing.T) {
	pinTTLs(t)
	ws := &countingWorkspace{ready: true, yolo: true}
	m := newBusyUI(ws)
	runCmds(m, m.dispatchBusyRefresh())
	require.True(t, m.yoloModeCached(), "precondition: probed ON")

	ws.skipGetErr, ws.yolo = errors.New("timeout"), false // The fake would say "off" with the error.
	m.invalidateBusyCaches()
	m.yoloCache.set(true)
	runCmds(m, m.dispatchBusyRefresh())
	require.True(t, m.yoloModeCached(), "an unknown read must not flip the UI to off")
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

// TestSetSessionMessagesGatesAnimationsOnBusy verifies that reloading a
// session does not run spinner animations when the agent is not busy.
// A session that was killed mid-generation can persist an assistant message
// with no Finish part, which still reports Spinning() even though nothing
// is running. Arming the clock for it would leave a ghost "working"
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

	// When the agent is not busy, setSessionMessages must freeze the
	// animation clock so the ghost spinner stays still.
	cmd := m.setSessionMessages(msgs)
	require.Nil(t, cmd, "setSessionMessages must not start animations when agent is idle")
	require.False(t, m.chat.animAllowed, "an idle session reload must freeze the animation clock")
	require.Nil(t, m.chat.EnsureAnimating(), "a frozen clock must not arm while idle")
	require.False(t, m.chat.animRunning)

	// When the session's agent is busy, the clock may run for the same
	// message.
	warmCaches(m, true)
	m.probedSession, m.probedSessionBusy = "s1", true
	cmd = m.setSessionMessages(msgs)
	require.Nil(t, cmd, "setSessionMessages must not arm the clock itself")
	require.True(t, m.chat.animAllowed, "setSessionMessages must allow animations when agent is busy")
	require.NotNil(t, m.chat.EnsureAnimating(), "a visible spinning message must arm the clock")
	require.True(t, m.chat.animRunning)
}

// TestBusyProbeReEnablesFrozenAnimationClock pins the boot race: a session
// loaded before the first busy probe lands reads the zero-value memoized
// cache and freezes the animation clock even though the agent is working,
// so the spinner rendered on boot never ticks. When the authoritative
// busy result arrives it must re-enable the clock so the Update tail can
// arm it for the freshly rendered spinner.
func TestBusyProbeReEnablesFrozenAnimationClock(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, agentBusy: true}
	m := newBusyUI(ws)

	// An unfinished assistant turn (no Finish part) renders with a spinner.
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

	// Boot: the busy cache still reads its zero value, so the session load
	// freezes the animation clock even though the agent is working.
	require.False(t, m.isAgentBusy(), "boot: the memoized busy state is not populated yet")
	cmd := m.setSessionMessages(msgs)
	require.Nil(t, cmd)
	require.False(t, m.chat.animAllowed, "the boot reload must freeze the clock off the unpopulated cache")
	require.Nil(t, m.chat.EnsureAnimating(), "the frozen clock must not arm")
	require.False(t, m.chat.animRunning)

	// The authoritative probe lands: the agent is busy. The gate must
	// re-open so the visible spinner ticks.
	m.applyBusyState(busyStateMsg{gen: m.busyFetchGen, ready: true, agentBusy: true, forSession: "s1", sessionBusy: true})
	require.True(t, m.chat.animAllowed, "a busy probe must re-enable the animation gate")
	require.NotNil(t, m.chat.EnsureAnimating(), "a visible spinning item must arm the clock once allowed")
	require.True(t, m.chat.animRunning)

	// An idle probe must not re-freeze the clock on its own: the ghost
	// guard belongs to the session reload, not to the probe.
	m.applyBusyState(busyStateMsg{gen: m.busyFetchGen, ready: true, agentBusy: false, forSession: "s1", sessionBusy: false})
	require.True(t, m.chat.animAllowed, "an idle probe must leave the gate alone")
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

// TestMCPStateChangedRefreshesModel pins two edges of an MCP state change:
// the memoized sidebar states refresh off-thread (an authoritative re-fetch
// rather than the event payload), and the agent rebuild — the fourth
// UpdateAgentModel call site — can change the effective model, so the
// memoized ready/model state must be re-fetched off-thread afterwards; the
// updateAgentModelCmd helper makes that unforgettable.
func TestMCPStateChangedRefreshesModel(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{
		ready: true,
		model: workspace.AgentModel{ModelCfg: config.SelectedModel{Model: "post-mcp-model"}},
		mcpStates: map[string]mcp.ClientInfo{
			"ctx": {Name: "ctx", State: mcp.StateConnected, Counts: mcp.Counts{Tools: 4}},
		},
	}
	m := newBusyUI(ws)
	warmCaches(m, false)
	m.agentModel = workspace.AgentModel{ModelCfg: config.SelectedModel{Model: "pre-mcp-model"}}
	ws.resetCounters()

	_, cmd := m.Update(pubsub.Event[mcp.Event]{
		Type:    pubsub.UpdatedEvent,
		Payload: mcp.Event{Type: mcp.EventStateChanged, Name: "ctx", State: mcp.StateConnected},
	})
	require.Zero(t, ws.syncProbes(), "the MCP event handler must not probe synchronously")
	require.True(t, m.mcpFetchInFlight, "an MCP state change must schedule a state refresh")
	runCmds(m, cmd)
	require.False(t, m.mcpFetchInFlight)
	require.Equal(t, mcp.StateConnected, m.mcpStates["ctx"].State, "fetched states must land in the cache")

	// The rebuild is sequenced with agentModelChangedCmd; tea.Sequence's
	// wrapper msg is unexported, so deliver the invalidation message the
	// way the runtime would once the sequence finishes.
	_, cmd = m.Update(agentModelChangedMsg{})
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
	require.Contains(t, yoloPrompt, "!",
		"the yolo prompt icon must render after a remote toggle")

	// Flipping back off must restore the normal prompt.
	m.applyBusyState(busyStateMsg{gen: m.busyFetchGen, yolo: false})
	require.False(t, m.yoloModeCached())
	require.Equal(t, normalPrompt, ansi.Strip(m.textarea.View()),
		"toggling yolo off must restore the normal editor prompt")
}

// probedUI builds a UI on session s1 while the sessions in busy run, with
// the busy probe landed as it does at runtime: agentBusy reports a run in
// any session, sessionBusy only s1's.
func probedUI(t *testing.T, busy ...string) (*UI, *countingWorkspace) {
	t.Helper()
	ws := &countingWorkspace{ready: true, agentBusy: len(busy) > 0, busySessions: map[string]bool{}}
	for _, id := range busy {
		ws.busySessions[id] = true
	}
	m := newBusyUI(ws)
	warmCaches(m, false)
	runCmds(m, m.dispatchBusyRefresh())
	require.Equal(t, len(busy) > 0, m.isAgentBusy())
	require.Equal(t, "s1", m.probedSession)
	return m, ws
}

// unfinishedAssistant is an assistant message with no Finish part, which
// renders with a spinner.
func unfinishedAssistant(id, sessionID string) message.Message {
	return message.Message{
		ID:        id,
		SessionID: sessionID,
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.ReasoningContent{Thinking: "thinking..."}},
	}
}

// TestNewSessionAllowedWhileOtherSessionBusy: a run in another session must
// not block ctrl+n or the command palette's New Session. The busy warning and
// the new session were exclusive branches, so reaching the landing page means
// no warning fired.
func TestNewSessionAllowedWhileOtherSessionBusy(t *testing.T) {
	t.Parallel()

	for _, focus := range []uiFocusState{uiFocusEditor, uiFocusMain} {
		m, _ := probedUI(t, "s2")
		m.focus = focus
		m.handleKeyPressMsg(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
		require.Equal(t, uiLanding, m.state, "focus %d", focus)
		require.Nil(t, m.session, "focus %d", focus)
	}

	m, _ := probedUI(t, "s2")
	m.dialog.OpenDialog(&actionDialog{action: dialog.ActionNewSession{}})
	m.handleDialogMsg(struct{}{})
	require.Equal(t, uiLanding, m.state, "command palette")
}

// TestEscDoesNotArmCancelWhenOnlyOtherSessionBusy: esc cancels the current
// session's run, so a run elsewhere must not arm it (nor show its help); the
// current session's own run still does.
func TestEscDoesNotArmCancelWhenOnlyOtherSessionBusy(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		busy    string
		wantArm bool
	}{{"s2", false}, {"s1", true}} {
		m, ws := probedUI(t, tc.busy)
		isEsc := func(b key.Binding) bool { return b.Help().Key == "esc" }
		require.Equal(t, tc.wantArm, slices.ContainsFunc(m.ShortHelp(), isEsc), "short help, run in %s", tc.busy)
		require.Equal(t, tc.wantArm, slices.ContainsFunc(slices.Concat(m.FullHelp()...), isEsc), "full help, run in %s", tc.busy)
		m.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
		require.Equal(t, tc.wantArm, m.isCanceling, "run in %s", tc.busy)
		require.Zero(t, ws.cancelCalls, "a first esc never cancels")
	}
}

// TestPlaceholderReadyWhenOnlyOtherSessionBusy: the editor placeholder
// tracks the session on screen, not a run elsewhere.
func TestPlaceholderReadyWhenOnlyOtherSessionBusy(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ busy, want string }{{"s2", "ready"}, {"s1", "working"}} {
		m, _ := probedUI(t, tc.busy)
		m.readyPlaceholder, m.workingPlaceholder = "ready", "working"
		m.Update(plainMsg{})
		require.Equal(t, tc.want, m.textarea.Placeholder, "run in %s", tc.busy)
	}
}

// TestModelSwitchStillBlockedWhileAnySessionBusy: every session shares the
// model, so a run anywhere still blocks switching it.
func TestModelSwitchStillBlockedWhileAnySessionBusy(t *testing.T) {
	t.Parallel()

	m, _ := probedUI(t, "s2")
	cmd := m.handleSelectModel(dialog.ActionSelectModel{})
	require.NotNil(t, cmd)
	require.Equal(t, util.NewWarnMsg("Agent is busy, please wait..."), cmd())
}

// TestSessionSwitchDoesNotInheritBusyState: switching away from a running
// session must not treat the next one as running, neither from the previous
// session's busy probe nor from its spinners still in the chat. Allowing
// animations then would tick the ghost spinner of a run killed in s2.
func TestSessionSwitchDoesNotInheritBusyState(t *testing.T) {
	t.Parallel()

	m, _ := probedUI(t, "s1")
	m.setSessionMessages([]message.Message{unfinishedAssistant("m1", "s1")})
	require.True(t, m.chat.HasLiveSpinner(), "precondition: s1's turn spins")

	_, cmd := m.Update(loadSessionMsg{
		session:  &session.Session{ID: "s2"},
		messages: []message.Message{unfinishedAssistant("m2", "s2")},
	})
	require.False(t, m.chat.animAllowed, "s2's ghost spinner must stay still")
	m.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.False(t, m.isCanceling, "esc must not arm a cancel for idle s2")

	runCmds(m, cmd)
	require.Equal(t, "s2", m.probedSession, "the switch must probe the new session")
	require.False(t, m.probedSessionBusy)
}

// TestBusyProbeStartsTodoSpinnerOnlyForCurrentSession: the todo spinner
// belongs to the session on screen. The probe that first reports its run
// must start it, and a run elsewhere must not.
func TestBusyProbeStartsTodoSpinnerOnlyForCurrentSession(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		busy string
		want bool
	}{{"s2", false}, {"s1", true}} {
		ws := &countingWorkspace{ready: true, agentBusy: true, busySessions: map[string]bool{tc.busy: true}}
		m := newBusyUI(ws)
		m.session.Todos = []session.Todo{{Content: "t", Status: session.TodoStatusInProgress}}
		warmCaches(m, false)
		runCmds(m, m.dispatchBusyRefresh())
		require.Equal(t, tc.want, m.todoIsSpinning, "run in %s", tc.busy)
	}
}

// TestTodoSpinnerIgnoresOtherSessionRun: the todo spinner tracks the session
// on screen, both when it loads and when its messages change, so a run
// elsewhere must not start it.
func TestTodoSpinnerIgnoresOtherSessionRun(t *testing.T) {
	t.Parallel()

	m, _ := probedUI(t, "s2")
	todos := []session.Todo{{Content: "t", Status: session.TodoStatusInProgress}}
	m.Update(loadSessionMsg{session: &session.Session{ID: "s1", Todos: todos}})
	require.False(t, m.todoIsSpinning, "loading the idle session must not spin its todo")

	m.Update(pubsub.Event[message.Message]{
		Type:    pubsub.DeletedEvent,
		Payload: message.Message{ID: "gone", SessionID: "s1"},
	})
	require.False(t, m.todoIsSpinning, "a message event in the idle session must not spin its todo")
}

// TestSummarizeAllowedWhileOtherSessionBusy: summarizing rewrites the
// current session only, so only its own run blocks it, and a request for
// any other session is refused since its run is not what the gate checked.
func TestSummarizeAllowedWhileOtherSessionBusy(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		busy     string
		wantWarn bool
	}{{"s2", false}, {"s1", true}} {
		m, ws := probedUI(t, tc.busy)
		m.dialog.OpenDialog(&actionDialog{action: dialog.ActionSummarize{SessionID: "s1"}})
		_, warned := m.handleDialogMsg(struct{}{})().(util.InfoMsg)
		require.Equal(t, tc.wantWarn, warned, "run in %s", tc.busy)
		require.Equal(t, !tc.wantWarn, ws.summarizeCalls == 1, "run in %s", tc.busy)
	}

	m, ws := probedUI(t)
	m.dialog.OpenDialog(&actionDialog{action: dialog.ActionSummarize{SessionID: "s2"}})
	info, warned := m.handleDialogMsg(struct{}{})().(util.InfoMsg)
	require.True(t, warned, "a request for another session")
	require.NotContains(t, info.Msg, "busy", "no run was checked for that session")
	require.Zero(t, ws.summarizeCalls)
}

// TestNewSessionKeepsWorkspaceLSPs: LSPs belong to the workspace, shared by
// every session and every TUI on it, so a new session (Alt+N or the command
// palette) while another session runs must not stop them.
func TestNewSessionKeepsWorkspaceLSPs(t *testing.T) {
	t.Parallel()

	m, ws := probedUI(t, "s2")
	runCmds(m, m.handleKeyPressMsg(tea.KeyPressMsg{Code: 'n', Mod: tea.ModAlt}))
	require.Equal(t, uiLanding, m.state)

	m2, ws2 := probedUI(t, "s2")
	m2.dialog.OpenDialog(&actionDialog{action: dialog.ActionNewSession{}})
	runCmds(m2, m2.handleDialogMsg(struct{}{}))
	require.Equal(t, uiLanding, m2.state, "command palette")

	require.Zero(t, ws.lspStopCalls+ws2.lspStopCalls, "a new session must not stop the workspace's LSPs")
}

// runModeCmds is runCmds plus the input-mode results (mode switch,
// accept-edits write), returning the info banners the tree emitted.
func runModeCmds(m *UI, cmd tea.Cmd) []util.InfoMsg {
	if cmd == nil {
		return nil
	}
	var infos []util.InfoMsg
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			infos = append(infos, runModeCmds(m, c)...)
		}
	case util.InfoMsg:
		infos = append(infos, msg)
	case modeSwitchedMsg, acceptEditsSetMsg, busyStateMsg:
		_, next := m.Update(msg)
		infos = append(infos, runModeCmds(m, next)...)
	}
	return infos
}

// withAcceptEditsStep puts code+accept-edits back into the Shift+Tab cycle
// for one test. It flips a package var, so the test must not be parallel.
func withAcceptEditsStep(t *testing.T) {
	t.Helper()
	acceptEditsInShiftTab = true
	t.Cleanup(func() { acceptEditsInShiftTab = false })
}

// TestShiftTabDefaultCycleHasNoAcceptEdits: in Phase 1 Shift+Tab keeps the
// pre-A10 cycle code -> plan -> code+YOLO -> code; accept-edits is never
// written nor announced.
func TestShiftTabDefaultCycleHasNoAcceptEdits(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, false)
	m.mode = uiInputModeCode

	for i, want := range []struct {
		mode        uiInputMode
		yolo        bool
		placeholder string
	}{
		{uiInputModePlan, false, "Let's plan"},
		{uiInputModeCode, true, "Go crazy"},
		{uiInputModeCode, false, m.readyPlaceholder},
	} {
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
		infos := runModeCmds(m, cmd)
		require.Equal(t, want.mode, m.mode, "press %d", i+1)
		require.Equal(t, want.yolo, ws.yolo, "press %d", i+1)
		require.Equal(t, want.placeholder, m.textarea.Placeholder, "press %d", i+1)
		require.NotContains(t, infos, util.InfoMsg{Type: util.InfoTypeInfo, Msg: acceptEditsBannerMsg}, "press %d", i+1)
	}
	require.Zero(t, ws.acceptSetCalls, "Shift+Tab must not write accept-edits")
	require.Empty(t, ws.acceptEdits)
}

// TestShiftTabCycleIncludesAcceptEdits: with the step enabled, Shift+Tab
// walks code -> code+accept-edits -> plan -> code+YOLO -> code, never
// probing the workspace on the Update goroutine.
func TestShiftTabCycleIncludesAcceptEdits(t *testing.T) {
	pinTTLs(t)
	withAcceptEditsStep(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, false)
	m.mode = uiInputModeCode

	for i, want := range []struct {
		mode        uiInputMode
		acceptEdits bool
		yolo        bool
		placeholder string
	}{
		{uiInputModeCode, true, false, "Accepting edits"},
		{uiInputModePlan, false, false, "Let's plan"},
		{uiInputModeCode, false, true, "Go crazy"},
		{uiInputModeCode, false, false, m.readyPlaceholder},
	} {
		ws.resetCounters()
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
		require.Zero(t, ws.acceptCalls+ws.acceptSetCalls, "press %d: accept-edits IO must run off the Update goroutine", i+1)
		infos := runModeCmds(m, cmd)

		require.Equal(t, want.mode, m.mode, "press %d", i+1)
		require.Equal(t, want.acceptEdits, ws.acceptEdits["s1"], "press %d", i+1)
		require.Equal(t, want.acceptEdits, m.acceptEditsCached(), "press %d", i+1)
		require.Equal(t, want.yolo, ws.yolo, "press %d", i+1)
		require.Equal(t, want.placeholder, m.textarea.Placeholder, "press %d", i+1)
		if i == 0 {
			require.Contains(t, infos, util.InfoMsg{Type: util.InfoTypeInfo, Msg: acceptEditsBannerMsg})
		}
	}
	require.Len(t, ws.acceptEdits, 1, "only the current session is touched")
}

// TestAcceptEditsFollowsSession: the cached accept-edits state belongs to
// the session it was read for, so switching sessions shows that session's
// own state.
func TestAcceptEditsFollowsSession(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, acceptEdits: map[string]bool{"s1": true}}
	m := newBusyUI(ws)
	warmCaches(m, false)
	runModeCmds(m, m.dispatchBusyRefresh())
	require.True(t, m.acceptEditsCached())

	m.session = &session.Session{ID: "s2"}
	require.False(t, m.acceptEditsCached(), "s1's state must not show on s2")
	m.invalidateBusyCaches()
	runModeCmds(m, m.dispatchBusyRefresh())
	require.False(t, m.acceptEditsCached())

	m.session = &session.Session{ID: "s1"}
	m.invalidateBusyCaches()
	runModeCmds(m, m.dispatchBusyRefresh())
	require.True(t, m.acceptEditsCached())

	m.session = nil
	require.False(t, m.acceptEditsCached())
}

// TestAcceptEditsToggleSurvivesStaleProbe: a busy probe that read the
// workspace before the toggle's write landed must not revert the toggle.
func TestAcceptEditsToggleSurvivesStaleProbe(t *testing.T) {
	pinTTLs(t)
	withAcceptEditsStep(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, false)
	m.mode = uiInputModeCode

	cmd := m.toggleInputMode()
	require.True(t, m.acceptEditsCached())
	gen := m.busyFetchGen
	m.Update(busyStateMsg{gen: gen, forSession: "s1", ready: true}) // read before the write
	require.True(t, m.acceptEditsCached(), "a probe racing the toggle must not revert it")

	runModeCmds(m, cmd)
	require.True(t, ws.acceptEdits["s1"])
	m.Update(busyStateMsg{gen: gen, forSession: "s1", ready: true}) // started before the write landed
	require.True(t, m.acceptEditsCached())
}

// TestProbedIdleSessionIgnoresLeftoverSpinner: once the probe has answered
// for the session on screen, its answer decides. An item a finished run left
// spinning then neither keeps the session busy nor costs the walk over every
// chat item that turnInFlight's spinner fallback takes on each Update.
func TestProbedIdleSessionIgnoresLeftoverSpinner(t *testing.T) {
	t.Parallel()

	m, _ := probedUI(t)
	m.setSessionMessages([]message.Message{unfinishedAssistant("m1", "s1")})
	m.chat.SetAnimationsAllowed(true)
	require.True(t, m.chat.HasLiveSpinner(), "precondition: a spinner is left over")
	require.False(t, m.isCurrentSessionBusy(), "the probe said s1 is idle")

	// A turn sent after that probe outranks it.
	m.pending.sessionID = "s1"
	require.True(t, m.isCurrentSessionBusy())
	m.pending.sessionID = ""

	// Before any probe for s1 lands, the spinner is the only signal.
	m.probedSession = ""
	require.True(t, m.isCurrentSessionBusy())
}

// TestHalfPressedCancelDoesNotFollowSessionChange: the first esc arms a
// cancel of the session on screen. Leaving that session (a new session or a
// switch) must disarm it, so a single esc never cancels the next session's
// run, and the armed timer must not disarm a later press.
func TestHalfPressedCancelDoesNotFollowSessionChange(t *testing.T) {
	t.Parallel()

	esc := tea.KeyPressMsg{Code: tea.KeyEscape}

	t.Run("new session", func(t *testing.T) {
		t.Parallel()
		m, ws := probedUI(t, "s1")
		m.handleKeyPressMsg(esc)
		require.True(t, m.isCanceling, "precondition: esc armed s1's cancel")

		m.handleKeyPressMsg(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
		require.False(t, m.isCanceling, "a new session must disarm the cancel")

		m.sendMessage("hello")
		require.Equal(t, "new", m.currentSessionID())
		m.handleKeyPressMsg(esc)
		require.Zero(t, ws.cancelCalls, "one esc must not cancel the new session's run")
		require.True(t, m.isCanceling, "the esc arms the new session's cancel instead")
	})

	t.Run("switch", func(t *testing.T) {
		t.Parallel()
		m, ws := probedUI(t, "s1", "s2")
		m.handleKeyPressMsg(esc)
		require.True(t, m.isCanceling, "precondition: esc armed s1's cancel")
		staleTimer := cancelTimerExpiredMsg{gen: m.cancelTimerGen}

		_, cmd := m.Update(loadSessionMsg{session: &session.Session{ID: "s2"}})
		require.False(t, m.isCanceling, "a switch must disarm the cancel")

		runCmds(m, cmd)
		require.True(t, m.isCurrentSessionBusy(), "precondition: s2 runs")
		m.handleKeyPressMsg(esc)
		require.Zero(t, ws.cancelCalls, "one esc must not cancel s2's run")
		require.True(t, m.isCanceling)

		m.Update(staleTimer)
		require.True(t, m.isCanceling, "s1's timer must not disarm s2's press")
		m.Update(cancelTimerExpiredMsg{gen: m.cancelTimerGen})
		require.False(t, m.isCanceling, "s2's own timer disarms it")
	})
}

// TestInitializeProjectWarnsAboutInitializing: the busy warning names the
// action that was refused.
func TestInitializeProjectWarnsAboutInitializing(t *testing.T) {
	t.Parallel()

	m, _ := probedUI(t, "s2")
	m.dialog.OpenDialog(&actionDialog{action: dialog.ActionInitializeProject{}})
	require.Equal(t, util.NewWarnMsg("Agent is busy, please wait before initializing the project..."),
		m.handleDialogMsg(struct{}{})())
}

// inputModeBusyWarning is the warning Shift+Tab shows when a step would
// change plan mode or YOLO while a session runs.
var inputModeBusyWarning = util.InfoMsg{
	Type: util.InfoTypeWarn,
	Msg:  "Agent is busy, please wait before switching input mode...",
}

// newBusyCycleUI builds a UI on session s1 in code mode with sessions
// running per busySessions, probed so the caches match the workspace.
func newBusyCycleUI(ws *countingWorkspace) *UI {
	ws.ready, ws.agentBusy = true, true
	m := newBusyUI(ws)
	warmCaches(m, true)
	m.mode = uiInputModeCode
	runModeCmds(m, m.dispatchBusyRefresh())
	return m
}

// TestShiftTabEnablesAcceptEditsWhileBusy: accept-edits belongs to one
// session, so turning it on is not held back by a run in any session.
func TestShiftTabEnablesAcceptEditsWhileBusy(t *testing.T) {
	pinTTLs(t)
	withAcceptEditsStep(t)
	for name, busy := range map[string]string{"another session busy": "s2", "current session busy": "s1"} {
		t.Run(name, func(t *testing.T) {
			ws := &countingWorkspace{busySessions: map[string]bool{busy: true}}
			m := newBusyCycleUI(ws)
			require.True(t, m.isAgentBusy())
			require.Equal(t, busy == "s1", m.isCurrentSessionBusy())

			ws.resetCounters()
			_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
			require.Zero(t, ws.syncProbes(), "no workspace call on the Update goroutine")
			infos := runModeCmds(m, cmd)
			require.NotContains(t, infos, inputModeBusyWarning)
			require.Contains(t, infos, util.InfoMsg{Type: util.InfoTypeInfo, Msg: acceptEditsBannerMsg})
			require.True(t, ws.acceptEdits["s1"])
			require.True(t, m.acceptEditsCached())
			require.Equal(t, uiInputModeCode, m.mode)
			require.False(t, ws.yolo)
		})
	}
}

// TestShiftTabTurnsAcceptEditsOffWhileBusy: revoking accept-edits only
// touches this session and is always safe, so while a session runs
// Shift+Tab turns it off (back to code) instead of warning, even though the
// default cycle has no step to turn it on (here it was set through the API).
func TestShiftTabTurnsAcceptEditsOffWhileBusy(t *testing.T) {
	pinTTLs(t)
	for name, busy := range map[string]string{"another session busy": "s2", "current session busy": "s1"} {
		t.Run(name, func(t *testing.T) {
			ws := &countingWorkspace{busySessions: map[string]bool{busy: true}, acceptEdits: map[string]bool{"s1": true}}
			m := newBusyCycleUI(ws)
			require.True(t, m.acceptEditsCached())

			ws.resetCounters()
			_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
			require.Zero(t, ws.syncProbes(), "no workspace call on the Update goroutine")
			infos := runModeCmds(m, cmd)
			require.NotContains(t, infos, inputModeBusyWarning)
			require.Contains(t, infos, util.InfoMsg{Type: util.InfoTypeInfo, Msg: "input mode: code"})
			require.False(t, ws.acceptEdits["s1"])
			require.False(t, m.acceptEditsCached())
			require.Equal(t, uiInputModeCode, m.mode)
			require.False(t, ws.yolo)
			require.False(t, m.modeSwitching)

			// The next step would be plan, which a run holds back.
			_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
			infos = runModeCmds(m, cmd)
			require.Contains(t, infos, inputModeBusyWarning)
			require.False(t, ws.acceptEdits["s1"])
			require.Equal(t, uiInputModeCode, m.mode)
		})
	}
}

// TestShiftTabAcceptEditsWriteFails: a write that did not land (an old
// server, a transport error) reports its error instead of the banner, and
// the editor stops claiming accept-edits once re-probed.
func TestShiftTabAcceptEditsWriteFails(t *testing.T) {
	pinTTLs(t)
	withAcceptEditsStep(t)

	ws := &countingWorkspace{ready: true, acceptSetErr: errors.New("failed to set accept edits: status code 404")}
	m := newBusyUI(ws)
	warmCaches(m, false)
	m.mode = uiInputModeCode

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	infos := runModeCmds(m, cmd)
	require.Contains(t, infos, util.InfoMsg{Type: util.InfoTypeError, Msg: "failed to set accept edits: status code 404"})
	require.NotContains(t, infos, util.InfoMsg{Type: util.InfoTypeInfo, Msg: acceptEditsBannerMsg})
	require.False(t, ws.acceptEdits["s1"])
	require.False(t, m.acceptEditsCached())
	require.Equal(t, m.readyPlaceholder, m.textarea.Placeholder)
	require.False(t, m.acceptEditsSetting)
}

// TestShiftTabWhileAcceptEditsWriteInFlight: a second press while the
// first one's write is in flight says so, not that the agent is busy.
func TestShiftTabWhileAcceptEditsWriteInFlight(t *testing.T) {
	pinTTLs(t)
	withAcceptEditsStep(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, false)
	m.mode = uiInputModeCode

	_, first := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	require.True(t, m.acceptEditsSetting)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	infos := runModeCmds(m, cmd)
	require.Contains(t, infos, util.InfoMsg{Type: util.InfoTypeWarn, Msg: "Changing mode, please wait..."})
	require.NotContains(t, infos, inputModeBusyWarning)

	runModeCmds(m, first)
	require.True(t, ws.acceptEdits["s1"], "the first press still lands")
}

// TestShiftTabBlockedWhileBusyWhenPlanOrYoloWouldChange: every step that
// flips workspace-wide plan mode or YOLO shows the busy warning while a
// session runs and changes nothing.
func TestShiftTabBlockedWhileBusyWhenPlanOrYoloWouldChange(t *testing.T) {
	pinTTLs(t)
	for name, setup := range map[string]func(*UI, *countingWorkspace){
		// The user's own YOLO skips the accept-edits step: next is plan.
		"user YOLO -> plan": func(m *UI, ws *countingWorkspace) {
			ws.yolo = true
		},
		"plan -> code+YOLO": func(m *UI, ws *countingWorkspace) {
			m.mode = uiInputModePlan
		},
		"code+YOLO -> code": func(m *UI, ws *countingWorkspace) {
			ws.yolo, m.cycleYolo = true, true
		},
	} {
		t.Run(name, func(t *testing.T) {
			ws := &countingWorkspace{busySessions: map[string]bool{"s2": true}}
			m := newBusyCycleUI(ws)
			setup(m, ws)
			runModeCmds(m, m.dispatchBusyRefresh())
			mode, yolo, accept := m.mode, ws.yolo, ws.acceptEdits["s1"]

			ws.resetCounters()
			_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
			require.Zero(t, ws.syncProbes(), "no workspace call on the Update goroutine")
			infos := runModeCmds(m, cmd)
			require.Contains(t, infos, inputModeBusyWarning)
			require.Equal(t, mode, m.mode)
			require.Equal(t, yolo, ws.yolo)
			require.Equal(t, accept, ws.acceptEdits["s1"])
			require.Equal(t, accept, m.acceptEditsCached())
			require.False(t, m.modeSwitching)
		})
	}
}

// runBang starts a bang command in the current session, wrapping its cancel
// so the test sees whether it was called. The command itself never runs.
func runBang(m *UI, killed *bool) {
	m.runShellCommand("sleep 60")
	cancel := m.bangCancel
	m.bangCancel = func() { *killed = true; cancel() }
}

// TestBangInOtherSessionDoesNotBusyCurrentSession: a shell command keeps its
// own session busy, not the one on screen after a switch, so esc there
// cannot kill it. It still counts for workspace-wide gates.
func TestBangInOtherSessionDoesNotBusyCurrentSession(t *testing.T) {
	t.Parallel()

	m, ws := probedUI(t)
	var killed bool
	runBang(m, &killed)
	_, cmd := m.Update(loadSessionMsg{session: &session.Session{ID: "s2"}})
	runCmds(m, cmd)

	require.False(t, m.isCurrentSessionBusy(), "s2 runs nothing")
	require.True(t, m.isAgentBusy(), "s1's shell command still runs")
	m.readyPlaceholder, m.workingPlaceholder = "ready", "working"
	m.Update(plainMsg{})
	require.Equal(t, "ready", m.textarea.Placeholder)
	esc := tea.KeyPressMsg{Code: tea.KeyEscape}
	m.handleKeyPressMsg(esc)
	m.handleKeyPressMsg(esc)
	require.False(t, killed, "esc in s2 must not kill s1's shell command")
	require.Zero(t, ws.cancelCalls)

	// Back in s1, esc-esc does kill it.
	_, cmd = m.Update(loadSessionMsg{session: &session.Session{ID: "s1"}})
	runCmds(m, cmd)
	require.True(t, m.isCurrentSessionBusy())
	m.handleKeyPressMsg(esc)
	m.handleKeyPressMsg(esc)
	require.True(t, killed)
}

// TestShellResultStaysInItsSession: a shell command's result belongs to the
// session it ran in (which stores it), so it neither lands in the chat of
// the session on screen nor releases a shell command running there.
func TestShellResultStaysInItsSession(t *testing.T) {
	t.Parallel()

	m, _ := probedUI(t)
	var killedS1, killedS2 bool
	runBang(m, &killedS1)
	_, cmd := m.Update(loadSessionMsg{session: &session.Session{ID: "s2"}})
	runCmds(m, cmd)
	runBang(m, &killedS2)
	require.Equal(t, 1, m.chat.Len(), "precondition: s2 shows its own pending command")

	m.Update(shellResultMsg{PendingID: "elsewhere", SessionID: "s1", Command: "sleep 60", Output: "s1 output"})
	require.Equal(t, 1, m.chat.Len(), "s1's result must not show in s2")
	require.False(t, killedS2, "s1's result must not cancel s2's command")
	require.True(t, m.isCurrentSessionBusy(), "s2's command still runs")

	m.Update(shellResultMsg{PendingID: "unknown", SessionID: "s2", Command: "echo", Output: "s2 output"})
	require.Equal(t, 2, m.chat.Len(), "a result for the session on screen still shows")
	require.True(t, killedS2, "its own result releases s2's command")
	require.False(t, m.isAgentBusy())
}

// TestBangTransportErrorFreesShellSlot: a shell command whose run fails with
// a non-cancel error (client/server transport/HTTP failure) must still come
// back as a shellResultMsg, or bangCancel stays set and the whole workspace
// reads busy (model switch blocked) until another bang command replaces it.
func TestBangTransportErrorFreesShellSlot(t *testing.T) {
	t.Parallel()

	m, ws := probedUI(t)
	ws.shellErr = errors.New("connection refused")
	cmd := m.runShellCommand("echo hi")
	require.True(t, m.isAgentBusy(), "precondition: the command holds the shell slot")

	// The batch holds a stream reader that blocks until the run closes its
	// channel, so run every cmd concurrently and take the shell result.
	batch, ok := cmd().(tea.BatchMsg)
	require.True(t, ok)
	msgs := make(chan tea.Msg, len(batch))
	for _, c := range batch {
		go func() { msgs <- c() }()
	}
	var res shellResultMsg
	for range batch {
		select {
		case msg := <-msgs:
			if r, ok := msg.(shellResultMsg); ok {
				res = r
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out collecting shell cmds")
		}
	}
	require.Equal(t, "s1", res.SessionID, "the failure must come back as a shell result")
	require.Equal(t, 1, res.ExitCode)

	m.Update(res)
	require.False(t, m.isAgentBusy(), "the shell slot must be freed")
	require.Nil(t, m.bangCancel)
	item, ok := m.chat.MessageItem(res.PendingID).(*chat.ShellItem)
	require.True(t, ok)
	require.Contains(t, ansi.Strip(item.RawRender(80)), "connection refused", "the error shows in the session's chat")
}

// TestRunCompleteReprobesCurrentSession: a cancelled run publishes
// RunComplete but no AgentFinished/AgentError notification, so RunComplete
// itself must re-probe, or the session stays busy until the TTL backstop.
func TestRunCompleteReprobesCurrentSession(t *testing.T) {
	pinTTLs(t)

	m, ws := probedUI(t, "s1")
	require.True(t, m.isCurrentSessionBusy(), "precondition: s1 runs")
	ws.busySessions["s1"], ws.agentBusy = false, false

	_, cmd := m.Update(pubsub.Event[notify.RunComplete]{Payload: notify.RunComplete{SessionID: "s1", Cancelled: true}})
	require.True(t, m.busyFetchInFlight, "RunComplete must schedule a busy refresh")
	runCmds(m, cmd)
	require.False(t, m.isCurrentSessionBusy())
	require.False(t, m.isAgentBusy())
}

// TestEscCancelsOnlyItsSessionsWork: esc-esc in a running session cancels
// that session's run, not a shell command running in another session.
func TestEscCancelsOnlyItsSessionsWork(t *testing.T) {
	t.Parallel()

	m, ws := probedUI(t, "s2")
	var killed bool
	runBang(m, &killed)
	_, cmd := m.Update(loadSessionMsg{session: &session.Session{ID: "s2"}})
	runCmds(m, cmd)
	require.True(t, m.isCurrentSessionBusy(), "precondition: s2 runs")

	esc := tea.KeyPressMsg{Code: tea.KeyEscape}
	m.handleKeyPressMsg(esc)
	m.handleKeyPressMsg(esc)
	require.Equal(t, 1, ws.cancelCalls, "s2's run is cancelled")
	require.False(t, killed, "s1's shell command keeps running")
}
