package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// runCoordinator is a configurable agent.Coordinator stub for the
// cancel/drop tests. Run blocks until either ctx is canceled (so it
// can observe explicit Cancel paths) or release fires (so the test
// can let a "still running" turn finish on its own). The most recent
// ctx and the error returned to the caller are recorded for
// assertions.
type runCoordinator struct {
	release  chan struct{}
	returnFn func(ctx context.Context) error

	mu         sync.Mutex
	gotCtx     context.Context
	ranCount   atomic.Int32
	entered    chan struct{} // closed exactly once when Run is first entered.
	enteredOne sync.Once
}

func newRunCoordinator(returnFn func(ctx context.Context) error) *runCoordinator {
	return &runCoordinator{
		release:  make(chan struct{}),
		returnFn: returnFn,
		entered:  make(chan struct{}),
	}
}

func (s *runCoordinator) Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	s.mu.Lock()
	s.gotCtx = ctx
	s.mu.Unlock()
	s.ranCount.Add(1)
	s.enteredOne.Do(func() { close(s.entered) })
	select {
	case <-s.release:
	case <-ctx.Done():
		// Only fires if the run is actually cancellable.
	}
	return nil, s.returnFn(ctx)
}

func (s *runCoordinator) RunAccepted(ctx context.Context, accept *agent.AcceptedRun, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	return s.Run(ctx, sessionID, prompt, attachments...)
}

func (s *runCoordinator) BeginAccepted(sessionID string) *agent.AcceptedRun {
	return nil
}
func (s *runCoordinator) Cancel(string) {}
func (s *runCoordinator) CancelAll()    {}
func (s *runCoordinator) IsBusy() bool  { return false }
func (s *runCoordinator) IsSessionBusy(string) bool {
	return false
}
func (s *runCoordinator) QueuedPrompts(string) int          { return 0 }
func (s *runCoordinator) QueuedPromptsList(string) []string { return nil }
func (s *runCoordinator) ClearQueue(string)                 {}
func (s *runCoordinator) Summarize(context.Context, string) error {
	return nil
}
func (s *runCoordinator) Model() agent.Model                            { return agent.Model{} }
func (s *runCoordinator) UpdateModels(context.Context) error            { return nil }
func (s *runCoordinator) ReloadSkills(context.Context) error           { return nil }
func (s *runCoordinator) GenerateTitle(context.Context, string, string) {}

func (s *runCoordinator) capturedCtx() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gotCtx
}

// buildAgentWorkspace returns a controller wired to a backend whose
// single workspace exposes the given coordinator. The workspace
// shutdown hook is overridden to avoid driving a real [app.App]
// through teardown when the test exits.
func buildAgentWorkspace(t *testing.T, coord agent.Coordinator) (*controllerV1, string) {
	t.Helper()
	b := backend.New(context.Background(), nil, nil)
	a := &app.App{AgentCoordinator: coord}

	ws := &backend.Workspace{
		ID:   uuid.New().String(),
		Path: t.TempDir(),
		App:  a,
	}
	backend.InsertWorkspaceForTest(b, ws)
	backend.SetWorkspaceShutdownFnForTest(ws, func() {})

	s := &Server{backend: b}
	return &controllerV1{backend: b, server: s}, ws.ID
}

func postAgent(t *testing.T, c *controllerV1, ctx context.Context, wsID, sessionID string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(proto.AgentMessage{SessionID: sessionID, Prompt: "hi"})
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/workspaces/"+wsID+"/agent", bytes.NewReader(body))
	req.SetPathValue("id", wsID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c.handlePostWorkspaceAgent(rec, req)
	return rec
}

// TestPostAgent_ReturnsOKOnContextCanceled verifies that when another
// client cancels the session mid-turn, the prompting client's POST is
// unaffected: SendMessage is fire-and-forget, so the handler returns
// 200 immediately without waiting for the turn. A run that later
// returns context.Canceled never surfaces as a 500 to the prompter;
// the FinishReasonCanceled marker reaches SSE subscribers via the
// assistant message instead.
func TestPostAgent_ReturnsOKOnContextCanceled(t *testing.T) {
	t.Parallel()

	coord := newRunCoordinator(func(context.Context) error {
		return context.Canceled
	})
	c, wsID := buildAgentWorkspace(t, coord)

	// The handler returns immediately, before the dispatched run is
	// released, because the run no longer owns the HTTP response.
	rec := postAgent(t, c, t.Context(), wsID, "S1")
	require.Equal(t, http.StatusAccepted, rec.Code, "fire-and-forget SendMessage must return 202 without waiting for the run")

	// The run is dispatched on a goroutine; let it return
	// context.Canceled. Nothing from that path reaches the (already
	// returned) handler.
	select {
	case <-coord.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatched run was never entered")
	}
	close(coord.release)

	// Wait for the dispatched run to fully return. Backend.runAgent
	// swallows context.Canceled, so it must not publish a
	// notify.TypeAgentError. Publishing would dereference the synthetic
	// workspace's nil notification broker and crash this goroutine,
	// which is the explicit guard that a cancel produces no top-level
	// error event.
	require.Eventually(t, func() bool {
		return coord.ranCount.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)
}

// TestHandleError_ContextCanceledFallsThroughTo500 documents the step 8
// cleanup: the old context.Canceled special case in handleError was
// removed because runtime cancellation of an agent run can no longer
// reach handleError. The agent-prompt handler returns 202 before the run
// starts (fire-and-forget SendMessage) and Backend.runAgent swallows
// context.Canceled. Any context.Canceled that still reaches handleError
// is therefore an unexpected synchronous error and falls through to the
// default 500 like any other.
func TestHandleError_ContextCanceledFallsThroughTo500(t *testing.T) {
	t.Parallel()

	c := &controllerV1{server: &Server{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

	c.handleError(rec, req, context.Canceled)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestPostAgent_DetachesRequestContext verifies that the dispatched run
// is bound to the workspace context, not the prompting client's HTTP
// request context. Canceling the request context must neither cancel
// the run nor be observed by the coordinator.
func TestPostAgent_DetachesRequestContext(t *testing.T) {
	t.Parallel()

	coord := newRunCoordinator(func(context.Context) error {
		return nil
	})
	c, wsID := buildAgentWorkspace(t, coord)

	reqCtx, cancelReq := context.WithCancel(context.Background())

	// The handler returns immediately; the run keeps executing on its
	// own goroutine bound to the workspace context.
	rec := postAgent(t, c, reqCtx, wsID, "S1")
	require.Equal(t, http.StatusAccepted, rec.Code)

	select {
	case <-coord.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatched run was never entered")
	}

	// Drop the prompting client. This must not reach the run.
	cancelReq()

	got := coord.capturedCtx()
	require.NotNil(t, got)
	// Compare by identity (pointer), not reflect.DeepEqual: deep
	// comparison would traverse context internals that the runtime
	// mutates concurrently.
	require.False(t, got == reqCtx, "run ctx must not be the request ctx")
	require.NoError(t, got.Err(), "run ctx must not inherit cancellation from the dropped request")

	// Release the run so it returns cleanly.
	close(coord.release)
	require.Eventually(t, func() bool {
		return coord.ranCount.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)
}
