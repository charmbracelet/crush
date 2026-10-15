package backend

import (
	"context"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

// runCancelRecorder ghi lại mọi CancelRun.
type runCancelRecorder struct {
	*busyCoordinator
	mu       sync.Mutex
	canceled []string
}

func (c *runCancelRecorder) CancelRun(sessionID, runID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.canceled = append(c.canceled, sessionID+"/"+runID)
}

func (c *runCancelRecorder) seen() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.canceled...)
}

// boundBackend dựng workspace bận với coordinator ghi CancelRun, và một
// client có claim (đã tạo workspace và mở SSE).
func boundBackend(t *testing.T) (*Backend, *Workspace, *runCancelRecorder, string) {
	t.Helper()
	b, ws, busy, _, _ := newBusyBackend(t, time.Hour)
	rec := &runCancelRecorder{busyCoordinator: busy}
	ws.App = &app.App{AgentCoordinator: rec}
	cid := newClientID(t)
	b.registerClient(ws, cid)
	require.NoError(t, b.AttachClient(ws.ID, cid))
	return b, ws, rec, cid
}

// `agent-cli run` thoát (kể cả Ctrl+C rồi retire): lượt nó gắn bị huỷ theo
// RunID; huỷ thừa một lượt đã xong là vô hại.
func TestBoundRun_CancelledWhenClientRetires(t *testing.T) {
	t.Parallel()
	b, ws, rec, cid := boundBackend(t)
	ws.bindRun(cid, boundRun{sessionID: "s1", runID: "r1"})

	require.NoError(t, b.RetireClient(cid))
	require.Equal(t, []string{"s1/r1"}, rec.seen())
}

// `agent-cli run` bị kill -9: SSE đứt, hết thời gian chờ nối lại thì claim
// mất và lượt bị huỷ, thay vì chạy tiếp tới trần mồ côi không ai giám sát.
func TestBoundRun_CancelledWhenClientVanishes(t *testing.T) {
	t.Parallel()
	b, ws, rec, cid := boundBackend(t)
	ws.bindRun(cid, boundRun{sessionID: "s1", runID: "r1"})

	b.DetachClient(ws.ID, cid)
	require.Empty(t, rec.seen(), "a dropped stream gets the reconnect grace first")
	require.Eventually(t, func() bool { return len(rec.seen()) == 1 },
		2*time.Second, 5*time.Millisecond, "after the grace the bound run is cancelled")
	require.Equal(t, []string{"s1/r1"}, rec.seen())
}

// Client nối lại trong thời gian chờ: lượt không bị huỷ.
func TestBoundRun_SurvivesReconnectWithinGrace(t *testing.T) {
	t.Parallel()
	b, ws, rec, cid := boundBackend(t)
	ws.bindRun(cid, boundRun{sessionID: "s1", runID: "r1"})

	b.DetachClient(ws.ID, cid)
	require.NoError(t, b.AttachClient(ws.ID, cid))
	require.Never(t, func() bool { return len(rec.seen()) > 0 }, 150*time.Millisecond, 10*time.Millisecond)
}

// Lượt không gắn (send, TUI) sống tiếp khi client rời đi.
func TestBoundRun_UnboundRunsAreLeftAlone(t *testing.T) {
	t.Parallel()
	b, _, rec, cid := boundBackend(t)
	require.NoError(t, b.RetireClient(cid))
	require.Empty(t, rec.seen())
}

// SendMessage có BindToClient thì gắn lượt vào claim của client đó.
func TestSendMessage_BindsRunToClient(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	rec := &runCancelRecorder{busyCoordinator: &busyCoordinator{blockingCoordinator: newBlockingCoordinator()}}
	t.Cleanup(func() { close(rec.release) })
	ws := insertAgentWorkspace(t, b, rec)
	cid := newClientID(t)
	b.registerClient(ws, cid)
	require.NoError(t, b.AttachClient(ws.ID, cid))

	require.NoError(t, b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "s1", RunID: "r1", Prompt: "p", BindToClient: cid}))
	require.NoError(t, b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "s1", RunID: "r2", Prompt: "p"}))
	require.NoError(t, b.RetireClient(cid))
	require.Equal(t, []string{"s1/r1"}, rec.seen(), "only the bound run is cancelled")
}

// ctxCoordinator ghi lại ctx mà RunAccepted nhận.
type ctxCoordinator struct {
	*blockingCoordinator
	got chan context.Context
}

func (c *ctxCoordinator) RunAccepted(ctx context.Context, _ *agent.AcceptedRun, _, _ string, _ ...message.Attachment) (*fantasy.AgentResult, error) {
	c.got <- ctx
	return nil, nil
}

// SendMessage có NonInteractive thì lượt chạy với ctx đánh dấu phi tương tác.
func TestSendMessage_PassesNonInteractive(t *testing.T) {
	t.Parallel()
	for _, want := range []bool{true, false} {
		b, _ := newTestBackend(t)
		coord := &ctxCoordinator{blockingCoordinator: newBlockingCoordinator(), got: make(chan context.Context, 1)}
		ws := insertAgentWorkspace(t, b, coord)
		require.NoError(t, b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "s1", RunID: "r1", Prompt: "p", NonInteractive: want}))
		select {
		case ctx := <-coord.got:
			require.Equal(t, want, agent.NonInteractiveFromContext(ctx))
		case <-time.After(5 * time.Second):
			t.Fatal("run never dispatched")
		}
	}
}
