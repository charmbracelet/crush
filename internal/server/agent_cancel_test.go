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
func (s *runCoordinator) Model() agent.Model                 { return agent.Model{} }
func (s *runCoordinator) UpdateModels(context.Context) error { return nil }

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
// client cancels the session mid-turn, the prompting client's still
// open POST receives 200 (not 500). The agent surfaces the
// FinishReasonCanceled marker to every SSE subscriber via the
// assistant message; the HTTP response from the prompter should not
// double as an error signal.
func TestPostAgent_ReturnsOKOnContextCanceled(t *testing.T) {
	t.Parallel()

	coord := newRunCoordinator(func(context.Context) error {
		return context.Canceled
	})
	c, wsID := buildAgentWorkspace(t, coord)

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- postAgent(t, c, t.Context(), wsID, "S1")
	}()

	// Wait until Run is in flight, then release it to return
	// context.Canceled.
	select {
	case <-coord.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("coordinator Run was never entered")
	}
	close(coord.release)

	select {
	case rec := <-done:
		require.Equal(t, http.StatusOK, rec.Code, "context.Canceled from another client's cancel must not surface as 500")
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return after coordinator returned context.Canceled")
	}
}

// TestPostAgent_DetachesRequestContext verifies that canceling the
// prompting client's HTTP request context does not cancel the
// in-flight agent run. The coordinator must observe a context whose
// Done channel never fires from the request side; only the explicit
// cancel endpoint may end the run.
func TestPostAgent_DetachesRequestContext(t *testing.T) {
	t.Parallel()

	coord := newRunCoordinator(func(context.Context) error {
		return nil
	})
	c, wsID := buildAgentWorkspace(t, coord)

	reqCtx, cancelReq := context.WithCancel(context.Background())

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- postAgent(t, c, reqCtx, wsID, "S1")
	}()

	// Wait until Run is in flight, then drop the prompting client.
	select {
	case <-coord.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("coordinator Run was never entered")
	}
	cancelReq()

	// The captured ctx must be detached: context.WithoutCancel
	// returns a ctx with Done() == nil so request cancellation cannot
	// propagate.
	got := coord.capturedCtx()
	require.NotNil(t, got)
	require.Nil(t, got.Done(), "coordinator ctx must be detached from r.Context() via context.WithoutCancel")
	require.NoError(t, got.Err(), "coordinator ctx must not inherit cancellation from the dropped request")

	// Confirm Run is still running: it should not have completed
	// just because the request ctx was canceled.
	select {
	case <-done:
		t.Fatal("handler returned before run completed; request ctx cancellation leaked into the run")
	case <-time.After(50 * time.Millisecond):
	}

	// Release the run; the handler should now complete cleanly.
	close(coord.release)
	select {
	case rec := <-done:
		// Writing to a recorder whose request ctx was canceled
		// still works; in production the TCP write would silently
		// fail, which is fine because the run already completed and
		// SSE subscribers have the result.
		require.Equal(t, http.StatusOK, rec.Code)
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return after release")
	}
	require.Equal(t, int32(1), coord.ranCount.Load())
}
