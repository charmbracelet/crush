package agent

import (
	"context"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func cancelRunAgent(t *testing.T, large *gatedStreamModel) (*sessionAgent, *pubsub.Broker[notify.RunComplete], string, fakeEnv) {
	t.Helper()
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  Model{Model: large, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		SmallModel:  Model{Model: &finishStreamModel{text: "title"}, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
	}).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	return sa, broker, sess.ID, env
}

// waitRunComplete chờ RunComplete mang runID.
func waitRunComplete(t *testing.T, ch <-chan pubsub.Event[notify.RunComplete], runID string) notify.RunComplete {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Payload.RunID == runID {
				return ev.Payload
			}
		case <-deadline:
			t.Fatalf("no RunComplete for %s", runID)
		}
	}
}

// ctxGatedModel như gatedStreamModel nhưng bị huỷ thì trả lỗi của ctx như
// một provider thật (request HTTP bị huỷ), thay vì vẫn trả kết quả.
type ctxGatedModel struct{ gatedStreamModel }

func (m *ctxGatedModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	if m.calls.Add(1) == 1 {
		close(m.entered)
		select {
		case <-m.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	m.calls.Add(-1)
	return m.gatedStreamModel.Stream(ctx, call)
}

// CancelRun với RunID của lượt đang chạy thì huỷ lượt đó; RunID khác thì không.
func TestCancelRun_CancelsOnlyTheMatchingActiveRun(t *testing.T) {
	t.Parallel()
	large := &ctxGatedModel{gatedStreamModel{text: "done", gate: make(chan struct{}), entered: make(chan struct{})}}
	sa, broker, sessionID, _ := cancelRunAgent(t, &large.gatedStreamModel)
	sa.SetModels(Model{Model: large, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}}, sa.smallModel.Get())
	ch := broker.Subscribe(t.Context())

	done := make(chan error, 1)
	go func() {
		_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sessionID, RunID: "run-a", Prompt: "a"})
		done <- err
	}()
	waitEntered(t, large.entered)

	sa.CancelRun(sessionID, "someone-else")
	require.True(t, sa.IsSessionBusy(sessionID), "a different RunID must not cancel the active run")

	sa.CancelRun(sessionID, "run-a")
	rc := waitRunComplete(t, ch, "run-a")
	require.True(t, rc.Cancelled)
	<-done
}

// Prompt mang RunID đang xếp hàng sau lượt của người khác: CancelRun gỡ đúng
// nó (người chờ nhận RunComplete Cancelled), lượt đang chạy không bị đụng.
func TestCancelRun_RemovesOnlyTheQueuedPrompt(t *testing.T) {
	t.Parallel()
	large := &gatedStreamModel{text: "done", gate: make(chan struct{}), entered: make(chan struct{})}
	sa, broker, sessionID, _ := cancelRunAgent(t, large)
	ch := broker.Subscribe(t.Context())

	done := make(chan error, 1)
	go func() {
		_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sessionID, RunID: "tui-turn", Prompt: "tui"})
		done <- err
	}()
	waitEntered(t, large.entered)

	_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sessionID, RunID: "run-queued", Prompt: "queued"})
	require.NoError(t, err)
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sessionID, RunID: "other-queued", Prompt: "other"})
	require.NoError(t, err)
	require.Equal(t, 2, sa.QueuedPrompts(sessionID))

	sa.CancelRun(sessionID, "run-queued")
	rc := waitRunComplete(t, ch, "run-queued")
	require.True(t, rc.Cancelled)
	require.Equal(t, []string{"other"}, sa.QueuedPromptsList(sessionID), "other queued prompts stay")
	require.True(t, sa.IsSessionBusy(sessionID), "the active turn keeps running")

	close(large.gate)
	require.NoError(t, <-done)
	require.False(t, waitRunComplete(t, ch, "tui-turn").Cancelled)
}

// CancelRun tới TRƯỚC prompt (Ctrl+C khi request gửi prompt còn đang bay):
// lượt đó bị huỷ ngay lúc vào, model không được gọi.
func TestCancelRun_BeforeArrivalCancelsOnEntry(t *testing.T) {
	t.Parallel()
	large := &gatedStreamModel{text: "done", gate: make(chan struct{}), entered: make(chan struct{})}
	close(large.gate)
	sa, broker, sessionID, _ := cancelRunAgent(t, large)
	ch := broker.Subscribe(t.Context())

	sa.CancelRun(sessionID, "late-run")
	_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sessionID, RunID: "late-run", Prompt: "late"})
	require.NoError(t, err)
	require.True(t, waitRunComplete(t, ch, "late-run").Cancelled)
	require.Zero(t, large.calls.Load(), "a run cancelled before arrival must not reach the model")

	// Dấu huỷ chỉ dùng một lần: lượt sau cùng RunID (không có thật) hay RunID khác chạy bình thường.
	_, err = sa.Run(context.WithoutCancel(t.Context()), SessionAgentCall{SessionID: sessionID, RunID: "next-run", Prompt: "next"})
	require.NoError(t, err)
	require.False(t, waitRunComplete(t, ch, "next-run").Cancelled)
}
