package backend

import (
	"context"
	"errors"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/notify"
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
// When requeued is true it instead stamps the requeue marker with
// ranRunID, simulating sessionAgent.Run's FIFO swap: the dispatched
// prompt went back into the session queue and the error belongs to the
// queued prompt that ran in its place.
type errorCoordinator struct {
	err           error
	markPublished bool
	requeued      bool
	ranRunID      string
}

func (c *errorCoordinator) Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	return nil, c.err
}

func (c *errorCoordinator) RunAccepted(ctx context.Context, accept *agent.AcceptedRun, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	if c.markPublished {
		agent.MarkRunCompletePublished(ctx)
	}
	if c.requeued {
		agent.MarkRunRequeued(ctx, c.ranRunID)
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
func (c *errorCoordinator) ClearQueue(string) []agent.QueuedMessage           { return nil }
func (c *errorCoordinator) PopQueuedMessage(string) (agent.QueuedMessage, bool) {
	return agent.QueuedMessage{}, false
}
func (c *errorCoordinator) Summarize(context.Context, string) error       { return nil }
func (c *errorCoordinator) Model() agent.Model                            { return agent.Model{} }
func (c *errorCoordinator) UpdateModels(context.Context) error            { return nil }
func (c *errorCoordinator) GenerateTitle(context.Context, string, string) {}

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

// TestRunAgent_RequeuedRunAttributesErrorToTheRunThatRan covers the FIFO
// swap's dispatch contract: when sessionAgent.Run requeues the
// dispatched prompt behind the session's queue and runs a queued prompt
// instead, the error it returns belongs to that queued prompt. runAgent
// must report it under that prompt's RunID and publish no terminal
// RunComplete for the dispatched one, which is still queued and owes
// exactly one terminal event when its own turn ends. Attributing either
// event to the dispatched RunID makes its `crush run` waiter exit on a
// foreign prompt's failure and yields a second terminal event for that
// RunID once the prompt actually runs.
func TestRunAgent_RequeuedRunAttributesErrorToTheRunThatRan(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	runErr := errors.New("provider rejected the request")
	ws := insertRunCompleteWorkspace(t, b, context.Background(),
		&errorCoordinator{err: runErr, requeued: true, ranRunID: "run-a"})

	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	terminals := ws.RunCompletions().Subscribe(subCtx)
	notifications := ws.AgentNotifications().Subscribe(subCtx)

	err := b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "S1", RunID: "run-b", Prompt: "hi"})
	require.NoError(t, err)

	ws.runWG.Wait()

	select {
	case ev := <-notifications:
		require.Equal(t, notify.TypeAgentError, ev.Payload.Type)
		require.Equal(t, "run-a", ev.Payload.RunID,
			"the error must be attributed to the prompt that actually ran")
		require.Equal(t, runErr.Error(), ev.Payload.Message)
	case <-time.After(2 * time.Second):
		t.Fatal("no agent error notification published for the failed turn")
	}

	select {
	case ev := <-terminals:
		t.Fatalf("terminal RunComplete published for a prompt that has not run: %+v", ev.Payload)
	case <-time.After(200 * time.Millisecond):
	}
}
