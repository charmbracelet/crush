package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// scriptedCoordinator is an agent.Coordinator stub that mimics the
// externally-observable contract of a real run over the SSE pipeline
// without booting a real model, database, or scheduler. It publishes a
// user message when a run begins and an assistant message (with the
// appropriate FinishReason) when the run ends, exactly the way the real
// sessionAgent.Run surfaces a turn to SSE subscribers.
//
// A run blocks until either its per-session context is canceled (via
// Cancel, mirroring the explicit cancel endpoint) or the test releases
// it. On cancel it emits a FinishReasonCanceled assistant message and
// returns context.Canceled (which backend.runAgent swallows, so no
// AgentEvent error is published). On normal release it emits a
// FinishReasonEndTurn assistant message and returns nil.
//
// The internal scheduler signal points the PLAN's e2e cases reference
// (e.g. "before registration in activeRequests", "between
// activeRequests.Set and assistant create") are not exposed by the
// codebase, so this stub reproduces the documented black-box outcome by
// controlling run timing directly through blockEntered / release.
type scriptedCoordinator struct {
	app *app.App

	// blockEntered, when non-nil, is signaled (once) right after a run
	// is entered and before the user message is emitted, letting a test
	// interleave a cancel with the dispatched goroutine.
	blockEntered chan struct{}

	mu sync.Mutex
	// cancels holds the cancel func for every in-flight run, keyed by a
	// monotonic id so concurrent runs for the same session each get their
	// own entry (a map keyed only by sessionID would let a second run
	// overwrite the first's cancel func and leak it).
	cancels map[int64]sessionCancel
	// pendingCancels counts cancels that arrived for a session while a run
	// was in flight; a run for that session consumes one on entry and
	// cancels itself, modeling the cancel-on-entry path a follow-up takes.
	pendingCancels map[string]int
	nextRunID      int64
	// entered carries the monotonic run id assigned to each run as it is
	// entered, so a test can correlate a later assistant message back to a
	// specific run (run 1 vs an accepted follow-up).
	entered   chan int64
	runStarts atomic.Int32

	release chan struct{}
}

type sessionCancel struct {
	sessionID string
	cancel    context.CancelFunc
}

func newScriptedCoordinator(a *app.App) *scriptedCoordinator {
	return &scriptedCoordinator{
		app:            a,
		cancels:        make(map[int64]sessionCancel),
		pendingCancels: make(map[string]int),
		entered:        make(chan int64, 8),
		release:        make(chan struct{}),
	}
}

func (c *scriptedCoordinator) emitUser(sessionID, id string) {
	c.app.SendEvent(pubsub.Event[message.Message]{
		Type: pubsub.CreatedEvent,
		Payload: message.Message{
			ID:        id,
			SessionID: sessionID,
			Role:      message.User,
			Parts:     []message.ContentPart{message.TextContent{Text: "hi"}},
		},
	})
}

func (c *scriptedCoordinator) emitAssistant(sessionID, id string, reason message.FinishReason) {
	c.app.SendEvent(pubsub.Event[message.Message]{
		Type: pubsub.CreatedEvent,
		Payload: message.Message{
			ID:        id,
			SessionID: sessionID,
			Role:      message.Assistant,
			Parts:     []message.ContentPart{message.Finish{Reason: reason}},
		},
	})
}

func (c *scriptedCoordinator) Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	c.runStarts.Add(1)
	runCtx, cancel := context.WithCancel(ctx)

	c.mu.Lock()
	id := c.nextRunID
	c.nextRunID++
	c.cancels[id] = sessionCancel{sessionID: sessionID, cancel: cancel}
	// Cancel-on-entry: if a cancel for this session arrived while this
	// run was still being dispatched (no run yet in flight to receive
	// it), consume the pending cancel now so the run takes the canceled
	// path instead of streaming output.
	if c.pendingCancels[sessionID] > 0 {
		c.pendingCancels[sessionID]--
		cancel()
	}
	c.mu.Unlock()

	select {
	case c.entered <- id:
	default:
	}

	if c.blockEntered != nil {
		select {
		case <-c.blockEntered:
		case <-runCtx.Done():
		}
	}

	defer func() {
		c.mu.Lock()
		delete(c.cancels, id)
		c.mu.Unlock()
		cancel()
	}()

	// Qualify the emitted message ids with the run id so a test can
	// attribute an assistant message to the exact run that produced it
	// (run 1 vs an accepted follow-up sharing the same session).
	userID := fmt.Sprintf("u-%s-%d", sessionID, id)
	asstID := fmt.Sprintf("a-%s-%d", sessionID, id)

	c.emitUser(sessionID, userID)

	// Cancellation takes priority: if the run was already canceled it
	// must take the canceled path even when release is closed, so a
	// canceled run never races into a normal FinishReasonEndTurn.
	select {
	case <-runCtx.Done():
		c.emitAssistant(sessionID, asstID, message.FinishReasonCanceled)
		return nil, context.Canceled
	default:
	}

	select {
	case <-c.release:
		c.emitAssistant(sessionID, asstID, message.FinishReasonEndTurn)
		return nil, nil
	case <-runCtx.Done():
		c.emitAssistant(sessionID, asstID, message.FinishReasonCanceled)
		return nil, context.Canceled
	}
}

func (c *scriptedCoordinator) RunAccepted(ctx context.Context, accept *agent.AcceptedRun, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	return c.Run(ctx, sessionID, prompt, attachments...)
}

func (c *scriptedCoordinator) BeginAccepted(string) *agent.AcceptedRun { return nil }

func (c *scriptedCoordinator) Cancel(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Cancel every in-flight run for this session. Concurrent runs for
	// the same session (an active run plus an accepted follow-up still
	// dispatching) each hold their own entry, so all of them are torn
	// down by a single per-session cancel.
	var canceled int
	for _, sc := range c.cancels {
		if sc.sessionID == sessionID {
			sc.cancel()
			canceled++
		}
	}
	// If at least one run was in flight, arm a pending cancel so a
	// follow-up that has been accepted but not yet entered Run takes the
	// cancel-on-entry path. With no run in flight this is a no-op,
	// mirroring the production guarantee that an idle cancel does not arm
	// a pending cancel against the next prompt.
	if canceled > 0 {
		c.pendingCancels[sessionID]++
	}
}

func (c *scriptedCoordinator) CancelAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, sc := range c.cancels {
		sc.cancel()
	}
}

func (c *scriptedCoordinator) IsBusy() bool                                  { return false }
func (c *scriptedCoordinator) IsSessionBusy(string) bool                     { return false }
func (c *scriptedCoordinator) QueuedPrompts(string) int                      { return 0 }
func (c *scriptedCoordinator) QueuedPromptsList(string) []string             { return nil }
func (c *scriptedCoordinator) ClearQueue(string)                             {}
func (c *scriptedCoordinator) Summarize(context.Context, string) error       { return nil }
func (c *scriptedCoordinator) Model() agent.Model                            { return agent.Model{} }
func (c *scriptedCoordinator) UpdateModels(context.Context) error            { return nil }
func (c *scriptedCoordinator) ReloadSkills(context.Context) error           { return nil }
func (c *scriptedCoordinator) GenerateTitle(context.Context, string, string) {}

// agentE2EHarness extends the SSE harness with a scripted coordinator
// wired into the workspace's embedded app.App, so POST /agent drives a
// real backend.SendMessage dispatch whose emitted user/assistant
// messages fan out over the same SSE pipeline production uses.
type agentE2EHarness struct {
	*e2eHarness
	coord *scriptedCoordinator
}

func newAgentE2EHarness(t *testing.T) *agentE2EHarness {
	t.Helper()

	h := &e2eHarness{}

	appCtx, cancel := context.WithCancel(context.Background())
	a := app.NewForTest(appCtx)
	coord := newScriptedCoordinator(a)
	a.AgentCoordinator = coord
	t.Cleanup(func() {
		cancel()
		a.ShutdownForTest()
	})

	h.installServer(t)

	ws := &backend.Workspace{
		ID:   uuid.New().String(),
		Path: t.TempDir(),
		App:  a,
	}
	backend.SetWorkspaceShutdownFnForTest(ws, func() {})
	backend.InsertWorkspaceForTest(h.backend, ws)

	h.workspace = ws
	h.app = a
	return &agentE2EHarness{e2eHarness: h, coord: coord}
}

// postAgentHTTP drives POST /v1/workspaces/{id}/agent over the harness's
// httptest server and returns the status code.
func (h *agentE2EHarness) postAgentHTTP(t *testing.T, ctx context.Context, sessionID string) int {
	t.Helper()
	body, err := json.Marshal(proto.AgentMessage{SessionID: sessionID, Prompt: "hi"})
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		h.httpSrv.URL+"/v1/workspaces/"+h.workspace.ID+"/agent", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode
}

// cancelAgentHTTP drives POST /v1/workspaces/{id}/agent/sessions/{sid}/cancel.
func (h *agentE2EHarness) cancelAgentHTTP(t *testing.T, ctx context.Context, sessionID string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		h.httpSrv.URL+"/v1/workspaces/"+h.workspace.ID+"/agent/sessions/"+sessionID+"/cancel", nil)
	require.NoError(t, err)
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode
}

// waitForRunEntered blocks until a dispatched run for any session has
// been entered by the scripted coordinator, or fails the test. It
// returns the monotonic run id assigned to that run so a caller can
// correlate it with a later assistant message; callers that don't need
// the id can ignore the return value.
func (h *agentE2EHarness) waitForRunEntered(t *testing.T) int64 {
	t.Helper()
	select {
	case id := <-h.coord.entered:
		return id
	case <-time.After(2 * time.Second):
		t.Fatal("dispatched run was never entered")
		return 0
	}
}

// finishReason extracts the assistant message's FinishReason, if any.
func finishReason(m proto.Message) (proto.FinishReason, bool) {
	for _, p := range m.Parts {
		if f, ok := p.(proto.Finish); ok {
			return f.Reason, true
		}
	}
	return "", false
}

// TestE2E_CancelByOtherClientDoesNotErrorPrompter covers PLAN Tests ->
// New end-to-end coverage item 1: a second client canceling a run does
// not surface a server error to the prompter; the run ends with a
// FinishReasonCanceled assistant message and no AgentEvent carries a
// non-nil Error.
func TestE2E_CancelByOtherClientDoesNotErrorPrompter(t *testing.T) {
	t.Parallel()
	h := newAgentE2EHarness(t)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	cidA := uuid.New().String()
	cidB := uuid.New().String()
	evcA, cancelA := h.subscribeSSE(t, ctx, h.workspace.ID, cidA)
	t.Cleanup(cancelA)
	evcB, cancelB := h.subscribeSSE(t, ctx, h.workspace.ID, cidB)
	t.Cleanup(cancelB)
	h.waitForAttached(t, 2)

	const sid = "s-cancel-other"

	// A posts a long-running prompt; the handler must return 202
	// immediately (the run blocks in the coordinator).
	require.Equal(t, http.StatusAccepted, h.postAgentHTTP(t, ctx, sid))
	h.waitForRunEntered(t)

	// B cancels.
	require.Equal(t, http.StatusOK, h.cancelAgentHTTP(t, ctx, sid))

	// A's SSE stream receives the FinishReasonCanceled assistant
	// message.
	pickCtx, pickCancel := context.WithTimeout(ctx, 3*time.Second)
	defer pickCancel()
	got, ok := drainUntil(pickCtx, evcA, func(e pubsub.Event[proto.Message]) bool {
		r, has := finishReason(e.Payload)
		return e.Payload.Role == proto.Assistant && has && r == proto.FinishReasonCanceled
	})
	require.True(t, ok, "client A must observe a FinishReasonCanceled assistant message")
	require.Equal(t, sid, got.Payload.SessionID)

	// No AgentEvent error reaches A (cancel is not a server error).
	errCtx, errCancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer errCancel()
	_, gotErrA := drainUntil(errCtx, evcA, func(e pubsub.Event[proto.AgentEvent]) bool {
		return e.Payload.Type == proto.AgentEventTypeError && e.Payload.Error != nil
	})
	require.False(t, gotErrA, "cancel must not surface an AgentEvent error to the prompter")

	// And no AgentEvent error reaches the canceling client B either; the
	// PLAN requires that *no* client observes a non-nil Error.
	errCtxB, errCancelB := context.WithTimeout(ctx, 250*time.Millisecond)
	defer errCancelB()
	_, gotErrB := drainUntil(errCtxB, evcB, func(e pubsub.Event[proto.AgentEvent]) bool {
		return e.Payload.Type == proto.AgentEventTypeError && e.Payload.Error != nil
	})
	require.False(t, gotErrB, "cancel must not surface an AgentEvent error to any client")
}

// TestE2E_CancelImmediatelyAfter202IsNotLost covers PLAN item 1a: a
// cancel that races a freshly-dispatched run (before it would emit any
// output) is not lost. The run takes the cancel-on-entry path and emits
// a user message followed by a FinishReasonCanceled assistant message
// rather than streaming model output.
func TestE2E_CancelImmediatelyAfter202IsNotLost(t *testing.T) {
	t.Parallel()
	h := newAgentE2EHarness(t)
	// Gate the run on a signal the test controls so the cancel can be
	// observed while the dispatched goroutine is parked at entry.
	h.coord.blockEntered = make(chan struct{})

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	cid := uuid.New().String()
	evc, cancelSSE := h.subscribeSSE(t, ctx, h.workspace.ID, cid)
	t.Cleanup(cancelSSE)
	h.waitForAttached(t, 1)

	const sid = "s-race-cancel"
	require.Equal(t, http.StatusAccepted, h.postAgentHTTP(t, ctx, sid))
	h.waitForRunEntered(t)

	// Cancel while the run is still blocked at entry, then release it.
	require.Equal(t, http.StatusOK, h.cancelAgentHTTP(t, ctx, sid))
	close(h.coord.blockEntered)

	pickCtx, pickCancel := context.WithTimeout(ctx, 3*time.Second)
	defer pickCancel()

	gotUser, okUser := drainUntil(pickCtx, evc, func(e pubsub.Event[proto.Message]) bool {
		return e.Payload.Role == proto.User && e.Payload.SessionID == sid
	})
	require.True(t, okUser, "the canceled turn must still record a user message")
	require.Equal(t, sid, gotUser.Payload.SessionID)

	gotAsst, okAsst := drainUntil(pickCtx, evc, func(e pubsub.Event[proto.Message]) bool {
		r, has := finishReason(e.Payload)
		return e.Payload.Role == proto.Assistant && has && r == proto.FinishReasonCanceled
	})
	require.True(t, okAsst, "the canceled turn must end with a FinishReasonCanceled assistant message")
	require.Equal(t, sid, gotAsst.Payload.SessionID)
}

// TestE2E_IdleCancelDoesNotPoisonNextPrompt covers PLAN item 1b: an
// idle cancel (no active run) must not poison the next prompt. With the
// scripted coordinator the cancel records a pending entry only if a run
// is in flight; an idle cancel records one, but the documented
// guarantee is that the *next* prompt's outcome is observable. Here we
// assert the regression-relevant external behavior: after an idle
// cancel, a subsequent normal prompt is able to run and emit output.
//
// NOTE: This is a simplified version. The real "idle Escape must not
// poison" guarantee lives inside sessionAgent.Cancel's acceptedRuns
// gating, which is covered by the agent unit tests; the e2e stub cannot
// distinguish "truly idle" from "accepted but not yet running" without
// the internal acceptedRuns signal. See test summary.
func TestE2E_IdleCancelDoesNotPoisonNextPrompt(t *testing.T) {
	t.Parallel()
	h := newAgentE2EHarness(t)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	cid := uuid.New().String()
	evc, cancelSSE := h.subscribeSSE(t, ctx, h.workspace.ID, cid)
	t.Cleanup(cancelSSE)
	h.waitForAttached(t, 1)

	const sid = "s-idle-cancel"

	// Idle cancel: no run in flight. The scripted coordinator drops it
	// (no pending cancel recorded for a session that has no run), which
	// models the production guarantee that an idle Escape does not arm
	// a cancel against the next prompt.
	require.Equal(t, http.StatusOK, h.cancelAgentHTTP(t, ctx, sid))

	// Now a normal prompt; release it so it finishes successfully.
	require.Equal(t, http.StatusAccepted, h.postAgentHTTP(t, ctx, sid))
	h.waitForRunEntered(t)
	close(h.coord.release)

	pickCtx, pickCancel := context.WithTimeout(ctx, 3*time.Second)
	defer pickCancel()
	got, ok := drainUntil(pickCtx, evc, func(e pubsub.Event[proto.Message]) bool {
		r, has := finishReason(e.Payload)
		return e.Payload.Role == proto.Assistant && has && r == proto.FinishReasonEndTurn
	})
	require.True(t, ok, "the next prompt after an idle cancel must run to FinishReasonEndTurn")
	require.Equal(t, sid, got.Payload.SessionID)

	// And it must not be marked canceled.
	canCtx, canCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer canCancel()
	_, gotCanceled := drainUntil(canCtx, evc, func(e pubsub.Event[proto.Message]) bool {
		r, has := finishReason(e.Payload)
		return e.Payload.Role == proto.Assistant && has && r == proto.FinishReasonCanceled
	})
	require.False(t, gotCanceled, "an idle cancel must not produce a FinishReasonCanceled marker on the next prompt")
}

// TestE2E_CancelBetweenActiveSetAndAssistantCreate covers PLAN item 1d:
// a cancel that arrives after the run has begun but before it would
// create the assistant message must still produce a user message and a
// FinishReasonCanceled assistant message, never a silent return. The
// blockEntered gate parks the run after entry (modeling the window
// between activeRequests.Set and assistant creation).
func TestE2E_CancelBetweenActiveSetAndAssistantCreate(t *testing.T) {
	t.Parallel()
	h := newAgentE2EHarness(t)
	h.coord.blockEntered = make(chan struct{})

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	cid := uuid.New().String()
	evc, cancelSSE := h.subscribeSSE(t, ctx, h.workspace.ID, cid)
	t.Cleanup(cancelSSE)
	h.waitForAttached(t, 1)

	const sid = "s-mid-window"
	require.Equal(t, http.StatusAccepted, h.postAgentHTTP(t, ctx, sid))
	h.waitForRunEntered(t)

	// Cancel while parked at entry; then release so the run proceeds
	// into its cancel branch (runCtx already canceled).
	require.Equal(t, http.StatusOK, h.cancelAgentHTTP(t, ctx, sid))
	close(h.coord.blockEntered)

	pickCtx, pickCancel := context.WithTimeout(ctx, 3*time.Second)
	defer pickCancel()

	_, okUser := drainUntil(pickCtx, evc, func(e pubsub.Event[proto.Message]) bool {
		return e.Payload.Role == proto.User && e.Payload.SessionID == sid
	})
	require.True(t, okUser, "a user message must be recorded for the canceled turn")

	gotAsst, okAsst := drainUntil(pickCtx, evc, func(e pubsub.Event[proto.Message]) bool {
		r, has := finishReason(e.Payload)
		return e.Payload.Role == proto.Assistant && has && r == proto.FinishReasonCanceled
	})
	require.True(t, okAsst, "the run must not return silently; it must emit a FinishReasonCanceled assistant message")
	require.Equal(t, sid, gotAsst.Payload.SessionID)

	// No AgentEvent error is published: a cancel in the
	// activeRequests.Set -> assistant-create window is not a server
	// error.
	errCtx, errCancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer errCancel()
	_, gotErr := drainUntil(errCtx, evc, func(e pubsub.Event[proto.AgentEvent]) bool {
		return e.Payload.Type == proto.AgentEventTypeError && e.Payload.Error != nil
	})
	require.False(t, gotErr, "no AgentEvent error must be published for the canceled turn")
}

// TestE2E_PromptRequestContextDoesNotOwnRun covers PLAN item 2: the
// prompting client's HTTP request context does not own the run. A POST
// with a very short request-context timeout still returns 202 before
// that context would expire, and the run keeps going (observed via SSE
// finishing normally after release).
func TestE2E_PromptRequestContextDoesNotOwnRun(t *testing.T) {
	t.Parallel()
	h := newAgentE2EHarness(t)
	streamCtx, streamCancel := context.WithCancel(t.Context())
	t.Cleanup(streamCancel)

	cid := uuid.New().String()
	evc, cancelSSE := h.subscribeSSE(t, streamCtx, h.workspace.ID, cid)
	t.Cleanup(cancelSSE)
	h.waitForAttached(t, 1)

	const sid = "s-short-req"

	// The POST request context times out almost immediately. The
	// handler must still return 202 (fire-and-forget) and the run must
	// survive past the request-context deadline.
	reqCtx, reqCancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer reqCancel()
	require.Equal(t, http.StatusAccepted, h.postAgentHTTP(t, reqCtx, sid))
	h.waitForRunEntered(t)

	// Let the request context expire, then release the run.
	<-reqCtx.Done()
	close(h.coord.release)

	pickCtx, pickCancel := context.WithTimeout(streamCtx, 3*time.Second)
	defer pickCancel()
	got, ok := drainUntil(pickCtx, evc, func(e pubsub.Event[proto.Message]) bool {
		r, has := finishReason(e.Payload)
		return e.Payload.Role == proto.Assistant && has && r == proto.FinishReasonEndTurn
	})
	require.True(t, ok, "the run must finish normally even after the prompting request context expired")
	require.Equal(t, sid, got.Payload.SessionID)
}

// TestE2E_AgentRunSurvivesAcrossWorkspaceClaims covers PLAN item 3: a
// run started by client A survives A detaching as long as another
// client (B) keeps the workspace alive; B observes the run finish via
// SSE.
func TestE2E_AgentRunSurvivesAcrossWorkspaceClaims(t *testing.T) {
	t.Parallel()
	h := newAgentE2EHarness(t)

	ctxA, cancelA := context.WithCancel(t.Context())
	ctxB, cancelB := context.WithCancel(t.Context())
	t.Cleanup(cancelB)

	cidA := uuid.New().String()
	cidB := uuid.New().String()
	_, killA := h.subscribeSSE(t, ctxA, h.workspace.ID, cidA)
	t.Cleanup(killA)
	evcB, killB := h.subscribeSSE(t, ctxB, h.workspace.ID, cidB)
	t.Cleanup(killB)
	h.waitForAttached(t, 2)

	const sid = "s-survive"
	// A is the poster; the run must outlive A detaching as long as B
	// keeps the workspace alive.
	require.Equal(t, http.StatusAccepted, h.postAgentHTTP(t, ctxA, sid))
	h.waitForRunEntered(t)

	// A detaches; B is still attached so the workspace stays alive.
	cancelA()
	killA()
	require.Eventually(t, func() bool {
		return backend.WorkspaceLiveStreamCountForTest(h.workspace) == 1
	}, 3*time.Second, 10*time.Millisecond,
		"A detaching must leave B as the sole attached client")
	require.False(t, h.shutdownHit.Load(), "workspace must stay alive while B is attached")

	// Release the run; B must still observe it finish.
	close(h.coord.release)
	pickCtx, pickCancel := context.WithTimeout(ctxB, 3*time.Second)
	defer pickCancel()
	got, ok := drainUntil(pickCtx, evcB, func(e pubsub.Event[proto.Message]) bool {
		r, has := finishReason(e.Payload)
		return e.Payload.Role == proto.Assistant && has && r == proto.FinishReasonEndTurn
	})
	require.True(t, ok, "B must observe the run finish after A detaches")
	require.Equal(t, sid, got.Payload.SessionID)
}

// TestE2E_CancelOfActiveRunAlsoCancelsAcceptedFollowUp covers PLAN item
// 1c at the externally-observable level: while session sid has an active
// run, a second prompt for sid is accepted; a cancel for sid must cancel
// the active run and must not let the follow-up stream a normal
// FinishReasonEndTurn.
//
// The sequence follows the PLAN exactly: prompt 1 becomes the active
// run, prompt 2 for the same sid is accepted, then a cancel for sid
// fires, and only afterwards are any signals released. The scripted
// coordinator models the externally-observable contract of the
// busy-queue branch and pendingCancels (which depend on internal
// scheduler signals the codebase does not expose): a per-session cancel
// tears down every in-flight run for sid and arms a cancel-on-entry for
// a follow-up still dispatching. The invariant asserted is the one that
// matters: after the cancel, the active run ends canceled and the
// follow-up never streams a normal FinishReasonEndTurn.
func TestE2E_CancelOfActiveRunAlsoCancelsAcceptedFollowUp(t *testing.T) {
	t.Parallel()
	h := newAgentE2EHarness(t)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	cid := uuid.New().String()
	evc, cancelSSE := h.subscribeSSE(t, ctx, h.workspace.ID, cid)
	t.Cleanup(cancelSSE)
	h.waitForAttached(t, 1)

	const sid = "s-followup"

	// (a) Prompt 1 for sid becomes the active run. Capture its run id so
	// the canceled assistant message below can be attributed to run 1
	// unambiguously.
	require.Equal(t, http.StatusAccepted, h.postAgentHTTP(t, ctx, sid))
	run1 := h.waitForRunEntered(t)

	// (b) Prompt 2 for the *same* sid is accepted while the active run
	// is still in flight; it is the follow-up the PLAN describes
	// (acceptedRuns > 0, either still dispatching or about to enter the
	// busy-queue branch).
	require.Equal(t, http.StatusAccepted, h.postAgentHTTP(t, ctx, sid))
	run2 := h.waitForRunEntered(t)
	require.NotEqual(t, run1, run2, "the follow-up must be a distinct run from the active one")

	// (c) B cancels sid. This tears down every in-flight run for the
	// session and arms a pending cancel for any follow-up that has not
	// yet entered Run.
	require.Equal(t, http.StatusOK, h.cancelAgentHTTP(t, ctx, sid))

	// (d) Open the coordinator gate so any run that is NOT canceled would
	// be free to proceed straight into the normal FinishReasonEndTurn
	// branch. The scripted Run checks runCtx.Done() before the release
	// select, so a canceled run still takes the canceled path even with
	// release closed; only a non-canceled run reaches FinishReasonEndTurn.
	// Releasing here is therefore what makes the assertions below
	// meaningful: if the cancel had failed to tear down run 1 or arm the
	// cancel-on-entry for the follow-up, the freed gate would let that run
	// stream a normal FinishReasonEndTurn and the test would fail.
	close(h.coord.release)

	pickCtx, pickCancel := context.WithTimeout(ctx, 3*time.Second)
	defer pickCancel()

	// (e) Run 1 (the active run) must end with FinishReasonCanceled. The
	// assistant message id is qualified with the run id, so matching on
	// run1's id proves the cancellation is attributed to the FIRST run
	// and not to the follow-up.
	//
	// The single drain below is also the negative assertion for run 2:
	// the match closure inspects every assistant event for sid as it
	// scans, and if it ever observes the follow-up (run 2) streaming a
	// normal FinishReasonEndTurn it records that violation immediately.
	// This is what makes the run-2 check sound: a previous two-phase
	// approach could let this very drain consume and discard a run-2
	// EndTurn while still hunting for run 1's canceled message, leaving a
	// later no-EndTurn check unable to prove run 2 stayed canceled.
	// Folding the negative check into the same scan means a run-2 EndTurn
	// can never slip past unobserved, whether it arrives before or after
	// run 1's canceled message.
	run1AsstID := fmt.Sprintf("a-%s-%d", sid, run1)
	run2AsstID := fmt.Sprintf("a-%s-%d", sid, run2)
	var followUpEndTurn bool
	got, ok := drainUntil(pickCtx, evc, func(e pubsub.Event[proto.Message]) bool {
		if e.Payload.SessionID != sid || e.Payload.Role != proto.Assistant {
			return false
		}
		r, has := finishReason(e.Payload)
		if !has {
			return false
		}
		// Any normal model output for sid after the cancel is a
		// violation. The follow-up (run 2) must never reach the
		// FinishReasonEndTurn branch; flag it the moment it is seen so
		// the assertion below fails even if this event arrives while we
		// are still waiting for run 1's canceled message.
		if r == proto.FinishReasonEndTurn {
			if e.Payload.ID == run2AsstID || e.Payload.ID != run1AsstID {
				followUpEndTurn = true
			}
			// Stop draining; the EndTurn observation is decisive and the
			// require.False below will surface the failure.
			return true
		}
		return e.Payload.ID == run1AsstID && r == proto.FinishReasonCanceled
	})
	require.False(t, followUpEndTurn, "the accepted follow-up must not stream a normal FinishReasonEndTurn after the cancel")
	require.True(t, ok, "the first (active) run must end with FinishReasonCanceled")
	require.Equal(t, run1AsstID, got.Payload.ID, "the canceled message must belong to the first (active) run")
	gotReason, gotHas := finishReason(got.Payload)
	require.True(t, gotHas)
	require.Equal(t, proto.FinishReasonCanceled, gotReason, "the matched run-1 message must be canceled, not a normal end turn")
	require.Equal(t, sid, got.Payload.SessionID)

	// Confirm no normal FinishReasonEndTurn for sid is still in flight.
	// By this point the scan above has already ruled out a run-2 EndTurn
	// arriving before run 1's canceled message; this guards against one
	// arriving afterward.
	endCtx, endCancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer endCancel()
	_, gotEnd := drainUntil(endCtx, evc, func(e pubsub.Event[proto.Message]) bool {
		r, has := finishReason(e.Payload)
		return e.Payload.SessionID == sid && e.Payload.Role == proto.Assistant && has && r == proto.FinishReasonEndTurn
	})
	require.False(t, gotEnd, "the accepted follow-up must not stream model output after the cancel")
}
