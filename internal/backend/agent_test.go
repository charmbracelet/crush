package backend

import (
	"context"
	"sync/atomic"
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

// blockingCoordinator is a minimal agent.Coordinator whose RunAccepted
// blocks until release is closed. It records that RunAccepted was
// entered so tests can observe the dispatched goroutine. Every other
// method returns a zero value.
type blockingCoordinator struct {
	entered  chan struct{}
	release  chan struct{}
	runCount atomic.Int32
}

func newBlockingCoordinator() *blockingCoordinator {
	return &blockingCoordinator{
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
}

func (c *blockingCoordinator) Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	return nil, nil
}

func (c *blockingCoordinator) RunAccepted(ctx context.Context, accept *agent.AcceptedRun, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	c.runCount.Add(1)
	select {
	case c.entered <- struct{}{}:
	default:
	}
	<-c.release
	return nil, nil
}

func (c *blockingCoordinator) BeginAccepted(sessionID string) *agent.AcceptedRun { return nil }
func (c *blockingCoordinator) Cancel(string)                                     {}
func (c *blockingCoordinator) CancelAll()                                        {}
func (c *blockingCoordinator) IsBusy() bool                                      { return false }
func (c *blockingCoordinator) IsSessionBusy(string) bool                         { return false }
func (c *blockingCoordinator) QueuedPrompts(string) int                          { return 0 }
func (c *blockingCoordinator) QueuedPromptsList(string) []string                 { return nil }
func (c *blockingCoordinator) ClearQueue(string)                                 {}
func (c *blockingCoordinator) Summarize(context.Context, string) error           { return nil }
func (c *blockingCoordinator) Model() agent.Model                                { return agent.Model{} }
func (c *blockingCoordinator) UpdateModels(context.Context) error                { return nil }
func (c *blockingCoordinator) GenerateTitle(context.Context, string, string)     {}
func (c *blockingCoordinator) SetMainAgent(string) error                         { return nil }

// insertAgentWorkspace installs a synthetic workspace with the given
// coordinator (or none) and a workspace run context, mirroring the
// fields CreateWorkspace initializes.
func insertAgentWorkspace(t *testing.T, b *Backend, coord agent.Coordinator) *Workspace {
	t.Helper()
	ws := &Workspace{
		ID:           uuid.New().String(),
		Path:         t.TempDir(),
		resolvedPath: t.TempDir(),
		clients:      make(map[string]*clientState),
		shutdownFn:   func() {},
	}
	ws.App = &app.App{AgentCoordinator: coord}
	ws.ctx, ws.cancel = context.WithCancel(b.ctx)
	b.mu.Lock()
	b.workspaces.Set(ws.ID, ws)
	b.pathIndex[ws.resolvedPath] = ws.ID
	b.mu.Unlock()
	return ws
}

func TestSendMessage_WorkspaceNotFound(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	err := b.SendMessage("nope", proto.AgentMessage{SessionID: "S1", Prompt: "hi"})
	require.ErrorIs(t, err, ErrWorkspaceNotFound)
}

func TestSendMessage_AgentNotInitialized(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	ws := insertAgentWorkspace(t, b, nil)
	err := b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "S1", Prompt: "hi"})
	require.ErrorIs(t, err, ErrAgentNotInitialized)
}

func TestSendMessage_EmptyPrompt(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	ws := insertAgentWorkspace(t, b, newBlockingCoordinator())
	err := b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "S1", Prompt: ""})
	require.ErrorIs(t, err, agent.ErrEmptyPrompt)
}

func TestSendMessage_SessionMissing(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	ws := insertAgentWorkspace(t, b, newBlockingCoordinator())
	err := b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "", Prompt: "hi"})
	require.ErrorIs(t, err, agent.ErrSessionMissing)
}

func TestSendMessage_WorkspaceClosing(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	ws := insertAgentWorkspace(t, b, newBlockingCoordinator())
	ws.runMu.Lock()
	ws.closing = true
	ws.runMu.Unlock()
	err := b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "S1", Prompt: "hi"})
	require.ErrorIs(t, err, ErrWorkspaceClosing)
}

// TestSendMessage_SuccessIncrementsRunWG asserts the happy path returns
// nil synchronously and dispatches a tracked goroutine: while
// RunAccepted blocks, runWG.Wait must not complete (the ticket is
// outstanding); after release it drains.
func TestSendMessage_SuccessIncrementsRunWG(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	coord := newBlockingCoordinator()
	ws := insertAgentWorkspace(t, b, coord)

	err := b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "S1", Prompt: "hi"})
	require.NoError(t, err)

	select {
	case <-coord.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatched goroutine never entered RunAccepted")
	}
	require.Equal(t, int32(1), coord.runCount.Load())

	waited := make(chan struct{})
	go func() {
		ws.runWG.Wait()
		close(waited)
	}()

	select {
	case <-waited:
		t.Fatal("runWG.Wait completed while the run was still in flight; ticket was not added")
	case <-time.After(100 * time.Millisecond):
	}

	close(coord.release)

	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("runWG.Wait did not complete after the run returned")
	}
}
