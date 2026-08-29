package backend

import (
	"context"
	"errors"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// errorCoordinator is a minimal agent.Coordinator whose RunAccepted
// returns a configurable error. When markPublished is true it stamps
// the run-complete marker on the context before returning, simulating a
// real coordinator that already published the run's authoritative
// terminal RunComplete (so runAgent must not emit a duplicate fallback).
type errorCoordinator struct {
	err           error
	markPublished bool
}

func (c *errorCoordinator) Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	return nil, c.err
}

func (c *errorCoordinator) RunAccepted(ctx context.Context, accept *agent.AcceptedRun, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	if c.markPublished {
		agent.MarkRunCompletePublished(ctx)
	}
	return nil, c.err
}

func (c *errorCoordinator) BeginAccepted(sessionID string) *agent.AcceptedRun { return nil }
func (c *errorCoordinator) Cancel(string)                                     {}
func (c *errorCoordinator) CancelAll()                                        {}
func (c *errorCoordinator) IsBusy() bool                                      { return false }
func (c *errorCoordinator) IsSessionBusy(string) bool                         { return false }
func (c *errorCoordinator) QueuedPrompts(string) int                          { return 0 }
func (c *errorCoordinator) QueuedPromptsList(string) []string                 { return nil }
func (c *errorCoordinator) ClearQueue(string)                                 {}
func (c *errorCoordinator) Summarize(context.Context, string) error           { return nil }
func (c *errorCoordinator) Model() agent.Model                                { return agent.Model{} }
func (c *errorCoordinator) UpdateModels(context.Context) error                { return nil }
func (c *errorCoordinator) ReloadSkills(context.Context) error                { return nil }
func (c *errorCoordinator) GenerateTitle(context.Context, string, string)     {}

// insertRunCompleteWorkspace installs a workspace backed by a real
// app.App (so the runCompletions broker exists) with the given
// coordinator and a workspace run context derived from base.
func insertRunCompleteWorkspace(t *testing.T, b *Backend, base context.Context, coord agent.Coordinator) *Workspace {
	t.Helper()
	a := app.NewForTest(base)
	a.AgentCoordinator = coord
	t.Cleanup(a.ShutdownForTest)
	ws := &Workspace{
		ID:           uuid.New().String(),
		Path:         t.TempDir(),
		resolvedPath: t.TempDir(),
		clients:      make(map[string]*clientState),
		shutdownFn:   func() {},
	}
	ws.App = a
	ws.ctx, ws.cancel = context.WithCancel(base)
	b.mu.Lock()
	b.workspaces.Set(ws.ID, ws)
	b.pathIndex[ws.resolvedPath] = ws.ID
	b.mu.Unlock()
	return ws
}

// TestRunAgent_PreRunErrorPublishesTerminalRunComplete proves that an
// error returned from RunAccepted before the coordinator could publish
// its own terminal event (e.g. a readyWg or UpdateModels failure,
// modeled here by a stub coordinator) still yields a reliable terminal
// RunComplete for the run's RunID. Without it, a `crush run` caller
// blocking on that RunID would hang because the lossy TypeAgentError
// event is not a guaranteed terminal signal.
func TestRunAgent_PreRunErrorPublishesTerminalRunComplete(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	runErr := errors.New("update models failed")
	ws := insertRunCompleteWorkspace(t, b, context.Background(), &errorCoordinator{err: runErr})

	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := ws.RunCompletions().Subscribe(subCtx)

	err := b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "S1", RunID: "run-1", Prompt: "hi"})
	require.NoError(t, err)

	select {
	case ev := <-ch:
		require.Equal(t, "run-1", ev.Payload.RunID,
			"the terminal RunComplete must carry the dispatched RunID")
		require.Equal(t, "S1", ev.Payload.SessionID)
		require.Equal(t, runErr.Error(), ev.Payload.Error,
			"the fallback terminal event must be marked errored")
		require.False(t, ev.Payload.Cancelled)
	case <-time.After(2 * time.Second):
		t.Fatal("no terminal RunComplete published for a pre-run error; a run waiter would hang")
	}
}

// TestRunAgent_NoFallbackWhenCoordinatorPublished ensures the fallback
// is suppressed when the coordinator already emitted the run's
// authoritative terminal RunComplete, so callers never observe a
// duplicate terminal event for the same RunID.
func TestRunAgent_NoFallbackWhenCoordinatorPublished(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	runErr := errors.New("stream failed after publishing terminal event")
	ws := insertRunCompleteWorkspace(t, b, context.Background(),
		&errorCoordinator{err: runErr, markPublished: true})

	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := ws.RunCompletions().Subscribe(subCtx)

	err := b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "S1", RunID: "run-1", Prompt: "hi"})
	require.NoError(t, err)

	// Wait for the dispatched run goroutine to return so any publish
	// has already happened.
	ws.runWG.Wait()

	select {
	case ev := <-ch:
		t.Fatalf("runAgent published a duplicate terminal RunComplete: %+v", ev.Payload)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestRunAgent_CancellationPublishesNoErrorTerminal verifies that a
// context.Canceled result from RunAccepted produces no errored terminal
// RunComplete from runAgent: cancellation is sessionAgent.Run's
// responsibility (it publishes the cancelled marker) and the dispatcher
// must not synthesize an error terminal for it.
func TestRunAgent_CancellationPublishesNoErrorTerminal(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	ws := insertRunCompleteWorkspace(t, b, context.Background(),
		&errorCoordinator{err: context.Canceled})

	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := ws.RunCompletions().Subscribe(subCtx)

	err := b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "S1", RunID: "run-1", Prompt: "hi"})
	require.NoError(t, err)

	ws.runWG.Wait()

	select {
	case ev := <-ch:
		t.Fatalf("cancellation must not publish a terminal RunComplete: %+v", ev.Payload)
	case <-time.After(200 * time.Millisecond):
	}
}
