package backend

import (
	"context"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// runRecorder is an agent.Coordinator whose RunAccepted blocks until the
// run's own context is cancelled and then records what the agent would
// see, keyed by the run's RunID. It embeds blockingCoordinator purely
// for the interface methods these tests do not exercise; its release
// channel is unused.
//
// gate, when non-nil, is waited on before the run looks at its context.
// It lets a test cancel a run before the dispatched goroutine has had a
// chance to observe it.
type runRecorder struct {
	*blockingCoordinator

	gate    chan struct{}
	started chan string
	endings *endMap
}

func newRunRecorder() *runRecorder {
	return &runRecorder{
		blockingCoordinator: newBlockingCoordinator(),
		started:             make(chan string, 8),
		endings:             &endMap{seen: make(map[string]error)},
	}
}

func (c *runRecorder) RunAccepted(ctx context.Context, accept *agent.AcceptedRun, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	runID := agent.RunIDFromContext(ctx)
	select {
	case c.started <- runID:
	default:
	}
	if c.gate != nil {
		<-c.gate
	}
	<-ctx.Done()
	c.endings.record(runID, context.Cause(ctx))
	return nil, ctx.Err()
}

// endMap collects what each run's context reported when it ended. The
// recorded value is context.Cause, so a test can prove no
// server-internal reason travels on the run context: net/http reports a
// cancelled request context's cause as the request error, and the agent
// keys its whole cancellation cleanup off context.Canceled.
type endMap struct {
	mu   sync.Mutex
	seen map[string]error
}

func (m *endMap) record(runID string, cause error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seen[runID] = cause
}

func (m *endMap) get(runID string) (error, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cause, ok := m.seen[runID]
	return cause, ok
}

// awaitEnded waits for the named run to observe its cancellation and
// returns what its context reported.
func (m *endMap) awaitEnded(t *testing.T, runID string) error {
	t.Helper()
	var cause error
	require.Eventually(t, func() bool {
		var ok bool
		cause, ok = m.get(runID)
		return ok
	}, 3*time.Second, 5*time.Millisecond, "run %s was never cancelled", runID)
	return cause
}

// awaitStarted waits for a dispatched run to enter the coordinator.
func awaitStarted(t *testing.T, c *runRecorder) string {
	t.Helper()
	select {
	case runID := <-c.started:
		return runID
	case <-time.After(3 * time.Second):
		t.Fatal("dispatched run was never entered")
		return ""
	}
}

// sendRun dispatches one prompt owned by clientID and returns its RunID.
func sendRun(t *testing.T, b *Backend, ws *Workspace, clientID string) string {
	t.Helper()
	runID := uuid.New().String()
	require.NoError(t, b.SendMessage(ws.ID, proto.AgentMessage{
		SessionID: "S1",
		RunID:     runID,
		ClientID:  clientID,
		Prompt:    "hi",
	}))
	return runID
}

// TestSendMessage_RetiringOwnerCancelsItsRuns covers the reported
// failure: a `crush run` exits, and the turn it asked for keeps running
// on the server because an attached TUI holds the workspace open. The
// run must end with the claim of the client that asked for it, and the
// workspace must survive for the client still using it.
func TestSendMessage_RetiringOwnerCancelsItsRuns(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	coord := newRunRecorder()
	ws := insertAgentWorkspace(t, b, coord)

	owner, other := newClientID(t), newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, owner))
	require.NoError(t, b.AttachClient(ws.ID, other))

	runID := sendRun(t, b, ws, owner)
	require.Equal(t, runID, awaitStarted(t, coord))

	require.NoError(t, b.RetireClient(owner))

	require.ErrorIs(t, coord.endings.awaitEnded(t, runID), context.Canceled)

	// The other client is still on the workspace, so it must not have
	// been torn down along with the run.
	_, err := b.GetWorkspace(ws.ID)
	require.NoError(t, err)
}

// TestSendMessage_UnownedRunSurvivesUnrelatedRetirement pins the
// ownership rule: a run nobody claimed (an in-process caller, or a
// client too old to send a client ID) is not another client's to end.
func TestSendMessage_UnownedRunSurvivesUnrelatedRetirement(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	coord := newRunRecorder()
	ws := insertAgentWorkspace(t, b, coord)

	other := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, other))

	runID := sendRun(t, b, ws, "")
	require.Equal(t, runID, awaitStarted(t, coord))

	require.NoError(t, b.RetireClient(other))
	require.Never(t, func() bool {
		_, ok := coord.endings.get(runID)
		return ok
	}, 200*time.Millisecond, 20*time.Millisecond,
		"an unowned run must not be ended by another client leaving")

	// Workspace shutdown still reaches it, which is the only bound an
	// unowned run has besides the maximum duration.
	ws.cancel()
	require.ErrorIs(t, coord.endings.awaitEnded(t, runID), context.Canceled)
}

// TestSendMessage_ExpiredOwnerClaimCancelsItsRuns covers the client that
// never comes back: the stream drops, the detach grace expires, and the
// timer that removes the claim is what reaps the run.
func TestSendMessage_ExpiredOwnerClaimCancelsItsRuns(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	b.SetDetachGrace(20 * time.Millisecond)
	coord := newRunRecorder()
	ws := insertAgentWorkspace(t, b, coord)

	owner, other := newClientID(t), newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, owner))
	require.NoError(t, b.AttachClient(ws.ID, other))

	runID := sendRun(t, b, ws, owner)
	require.Equal(t, runID, awaitStarted(t, coord))

	// The stream drops without a release, so the claim (and the run)
	// live on until the grace expires.
	b.DetachClient(ws.ID, owner)
	require.ErrorIs(t, coord.endings.awaitEnded(t, runID), context.Canceled)
}

// TestSendMessage_ReconnectWithinGraceKeepsTheRun is the other half of
// the same window: a momentary stream drop must not throw away a turn in
// progress, because the client is coming back.
func TestSendMessage_ReconnectWithinGraceKeepsTheRun(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	b.SetDetachGrace(2 * time.Second)
	coord := newRunRecorder()
	ws := insertAgentWorkspace(t, b, coord)

	owner := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, owner))

	runID := sendRun(t, b, ws, owner)
	require.Equal(t, runID, awaitStarted(t, coord))

	b.DetachClient(ws.ID, owner)
	require.NoError(t, b.AttachClient(ws.ID, owner))

	require.Never(t, func() bool {
		_, ok := coord.endings.get(runID)
		return ok
	}, 200*time.Millisecond, 20*time.Millisecond,
		"a reconnect inside the detach grace must keep the run")
}

// TestSendMessage_CleanExitCancelsAtTheFinalDetach walks a quitting
// client's real order: release the claim first, then let the stream end.
// The release alone must not end the run — the client is still reading —
// but the final detach must.
func TestSendMessage_CleanExitCancelsAtTheFinalDetach(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	b.SetDetachGrace(2 * time.Second)
	coord := newRunRecorder()
	ws := insertAgentWorkspace(t, b, coord)

	owner, other := newClientID(t), newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, owner))
	require.NoError(t, b.AttachClient(ws.ID, other))

	runID := sendRun(t, b, ws, owner)
	require.Equal(t, runID, awaitStarted(t, coord))

	require.NoError(t, b.releaseHold(ws.ID, owner))
	require.Never(t, func() bool {
		_, ok := coord.endings.get(runID)
		return ok
	}, 100*time.Millisecond, 20*time.Millisecond,
		"releasing the claim while a stream is open must not end the run")

	// A released client skips the reconnect grace, so this removes the
	// claim immediately.
	b.DetachClient(ws.ID, owner)
	require.ErrorIs(t, coord.endings.awaitEnded(t, runID), context.Canceled)
}

// TestSendMessage_MaxRunDurationEndsTheRun bounds every run, owned or
// not: a turn stuck on something that never returns cannot live until
// the server restarts.
//
// It also pins the cancellation shape the agent depends on. The bound is
// a cancel, not a deadline, because sessionAgent.Run drives its whole
// cleanup path off errors.Is(err, context.Canceled) — a
// context.DeadlineExceeded would be recorded as a provider error and
// rendered as one.
func TestSendMessage_MaxRunDurationEndsTheRun(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	b.SetMaxRunDuration(20 * time.Millisecond)
	coord := newRunRecorder()
	ws := insertAgentWorkspace(t, b, coord)

	runID := sendRun(t, b, ws, "")
	require.Equal(t, runID, awaitStarted(t, coord))

	// Read the run context while the handle is still registered: the
	// registry drops it as soon as the run returns.
	runCtx := registeredRunCtx(t, ws, runID)

	require.ErrorIs(t, coord.endings.awaitEnded(t, runID), context.Canceled)
	require.ErrorIs(t, runCtx.Err(), context.Canceled,
		"the duration bound must present as a cancel, not a deadline")
	require.NotErrorIs(t, runCtx.Err(), context.DeadlineExceeded)
}

// TestSendMessage_CancelReasonStaysOffTheRunContext is a regression
// guard found in end-to-end testing. net/http reports the *cause* of a
// cancelled request context as the request's error, so attaching the
// server's reason with context.WithCancelCause sent that reason out
// through the provider call and back into the agent in place of
// context.Canceled. The agent then recorded the turn as a provider
// error, with a "there was an error while executing the tool" repair
// text, instead of a cancellation.
func TestSendMessage_CancelReasonStaysOffTheRunContext(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	coord := newRunRecorder()
	ws := insertAgentWorkspace(t, b, coord)

	owner := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, owner))

	runID := sendRun(t, b, ws, owner)
	require.Equal(t, runID, awaitStarted(t, coord))
	require.NoError(t, b.RetireClient(owner))

	// Recorded as context.Cause of the run context: it must be exactly
	// context.Canceled, never a server-side explanation.
	ended := coord.endings.awaitEnded(t, runID)
	require.Equal(t, context.Canceled, ended,
		"a run context must carry no cancel cause of its own")
}

// TestCancelRun_EndsOnlyTheNamedRun keeps the explicit endpoint narrow:
// two clients can be prompting the same session, so ending one run must
// not touch the other.
func TestCancelRun_EndsOnlyTheNamedRun(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	coord := newRunRecorder()
	ws := insertAgentWorkspace(t, b, coord)

	first := sendRun(t, b, ws, newClientID(t))
	require.Equal(t, first, awaitStarted(t, coord))
	second := sendRun(t, b, ws, newClientID(t))
	require.Equal(t, second, awaitStarted(t, coord))

	require.NoError(t, b.CancelRun(ws.ID, first))
	require.ErrorIs(t, coord.endings.awaitEnded(t, first), context.Canceled)
	_, ended := coord.endings.get(second)
	require.False(t, ended, "cancelling one run must not end another on the same session")
}

// TestCancelRun_UnknownRunSucceeds keeps the endpoint usable from a
// deferred cleanup that races normal completion.
func TestCancelRun_UnknownRunSucceeds(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws := insertAgentWorkspace(t, b, newRunRecorder())

	require.NoError(t, b.CancelRun(ws.ID, uuid.New().String()))
	require.NoError(t, b.CancelRun(ws.ID, ""))
	require.ErrorIs(t, b.CancelRun("nope", uuid.New().String()), ErrWorkspaceNotFound)
}

// TestSendMessage_CancelBeforeTheRunStartsIsNotLost pins why the run is
// registered by SendMessage rather than by the goroutine it dispatches:
// otherwise a cancel arriving in between would be dropped and the run
// would go on to work for nobody.
func TestSendMessage_CancelBeforeTheRunStartsIsNotLost(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	coord := newRunRecorder()
	coord.gate = make(chan struct{})
	ws := insertAgentWorkspace(t, b, coord)

	runID := sendRun(t, b, ws, newClientID(t))
	require.NoError(t, b.CancelRun(ws.ID, runID))

	// The run only now gets to look at its context.
	close(coord.gate)
	require.ErrorIs(t, coord.endings.awaitEnded(t, runID), context.Canceled)
}

// TestSendMessage_FinishedRunLeavesNoHandle guards against the registry
// growing for the life of the server: a workspace is long-lived and
// serves many runs.
func TestSendMessage_FinishedRunLeavesNoHandle(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	coord := newBlockingCoordinator()
	close(coord.release) // the run returns as soon as it is entered.
	ws := insertAgentWorkspace(t, b, coord)

	runID := sendRun(t, b, ws, newClientID(t))
	require.Eventually(t, func() bool {
		ws.runsMu.Lock()
		defer ws.runsMu.Unlock()
		return len(ws.runs) == 0
	}, 3*time.Second, 5*time.Millisecond, "the run handle must be released when the run returns")
	require.NoError(t, b.CancelRun(ws.ID, runID))
}

// lifetimeRecorder is an agent.Coordinator that captures the run
// lifetime the dispatcher attached to each run's context, then blocks
// until that run ends.
type lifetimeRecorder struct {
	*blockingCoordinator

	mu   sync.Mutex
	seen map[string]*agent.RunLifetime
}

func newLifetimeRecorder() *lifetimeRecorder {
	return &lifetimeRecorder{
		blockingCoordinator: newBlockingCoordinator(),
		seen:                make(map[string]*agent.RunLifetime),
	}
}

func (c *lifetimeRecorder) RunAccepted(ctx context.Context, accept *agent.AcceptedRun, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	c.mu.Lock()
	c.seen[agent.RunIDFromContext(ctx)] = agent.RunLifetimeFromContext(ctx)
	c.mu.Unlock()
	<-ctx.Done()
	return nil, ctx.Err()
}

func (c *lifetimeRecorder) lifetime(t *testing.T, runID string) *agent.RunLifetime {
	t.Helper()
	var l *agent.RunLifetime
	require.Eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		var ok bool
		l, ok = c.seen[runID]
		return ok
	}, 3*time.Second, 5*time.Millisecond, "run %s never entered the coordinator", runID)
	return l
}

// TestSendMessage_RunCarriesItsOwnLifetime pins the wiring every queued
// prompt depends on. A prompt dispatched into a busy session is queued
// and only runs later, in the frame of the turn it queued behind, so the
// agent needs this run's own context to run it under and a rendezvous
// that keeps this dispatch on the stack until it has. Without them the
// queued prompt inherited the cancellation scope of an unrelated,
// already-finished run, and its own run — handle deregistered, ceiling
// timer stopped — could no longer be reached at all.
func TestSendMessage_RunCarriesItsOwnLifetime(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	coord := newLifetimeRecorder()
	ws := insertAgentWorkspace(t, b, coord)

	runID := sendRun(t, b, ws, newClientID(t))
	lifetime := coord.lifetime(t, runID)
	require.NotNil(t, lifetime, "every dispatched run must carry its own lifetime")
	require.NoError(t, lifetime.Ctx.Err(), "a live run's lifetime context must be live")
	require.Equal(t, runID, agent.RunIDFromContext(lifetime.Ctx),
		"the lifetime must carry the run's own decorated context")
	require.NoError(t, registeredRunCtx(t, ws, runID).Err(),
		"the run must still be registered while its prompt could be queued")

	// Every per-run end path goes through that context, so it reaches a
	// prompt still queued under it.
	require.NoError(t, b.CancelRun(ws.ID, runID))
	select {
	case <-lifetime.Ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("cancelling a run must cancel the context its queued prompt would run under")
	}
}

// registeredRunCtx returns the live context of the named registered
// run, read from the registry so the test sees the same context the
// cancel paths act on.
func registeredRunCtx(t *testing.T, ws *Workspace, runID string) context.Context {
	t.Helper()
	ws.runsMu.Lock()
	defer ws.runsMu.Unlock()
	for h := range ws.runs {
		if h.runID == runID {
			return h.ctx
		}
	}
	t.Fatalf("run %s is not registered", runID)
	return nil
}

// TestDurationFromEnv_MaxRunDuration covers the operator override,
// including the disable-the-bound case.
func TestDurationFromEnv_MaxRunDuration(t *testing.T) {
	t.Setenv("CRUSH_SERVER_MAX_RUN_DURATION", "90")
	require.Equal(t, 90*time.Second, durationFromEnv("CRUSH_SERVER_MAX_RUN_DURATION", DefaultMaxRunDuration))

	t.Setenv("CRUSH_SERVER_MAX_RUN_DURATION", "0")
	require.Zero(t, durationFromEnv("CRUSH_SERVER_MAX_RUN_DURATION", DefaultMaxRunDuration))

	t.Setenv("CRUSH_SERVER_MAX_RUN_DURATION", "not-a-number")
	require.Equal(t, DefaultMaxRunDuration, durationFromEnv("CRUSH_SERVER_MAX_RUN_DURATION", DefaultMaxRunDuration))
}

// TestSendMessage_UnboundedWhenMaxRunDurationIsZero documents that the
// bound is optional.
func TestSendMessage_UnboundedWhenMaxRunDurationIsZero(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	b.SetMaxRunDuration(0)
	coord := newRunRecorder()
	ws := insertAgentWorkspace(t, b, coord)

	runID := sendRun(t, b, ws, "")
	require.Equal(t, runID, awaitStarted(t, coord))

	ws.runsMu.Lock()
	var armed bool
	for h := range ws.runs {
		if h.runID == runID {
			armed = h.maxDuration != nil
		}
	}
	ws.runsMu.Unlock()
	require.False(t, armed, "a zero maximum duration must not arm a timer")

	ws.cancel()
	require.ErrorIs(t, coord.endings.awaitEnded(t, runID), context.Canceled)
}
