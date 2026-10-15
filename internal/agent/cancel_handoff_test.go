package agent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// waitRunCompleteFor như waitRunComplete nhưng trả false khi hết giờ thay vì
// làm test thất bại, để thông báo lỗi nói rõ prompt nào bị kẹt.
func waitRunCompleteFor(ch <-chan pubsub.Event[notify.RunComplete], runID string, d time.Duration) (notify.RunComplete, bool) {
	deadline := time.After(d)
	for {
		select {
		case ev := <-ch:
			if ev.Payload.RunID == runID {
				return ev.Payload, true
			}
		case <-deadline:
			return notify.RunComplete{}, false
		}
	}
}

// CancelRun huỷ đúng lượt đang chạy: prompt xếp hàng sau nó vẫn chạy và có
// RunComplete, không kẹt tới lượt sau (người soát độc lập, P1-2).
func TestCancelRunActiveHandsOffQueuedPrompt(t *testing.T) {
	large := &ctxGatedModel{gatedStreamModel{text: "done", gate: make(chan struct{}), entered: make(chan struct{})}}
	sa, broker, sessionID, _ := cancelRunAgent(t, &large.gatedStreamModel)
	sa.SetModels(Model{Model: large, CatwalkCfg: sa.largeModel.Get().CatwalkCfg}, sa.smallModel.Get())
	ch := broker.Subscribe(t.Context())

	done := make(chan error, 1)
	go func() {
		_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sessionID, RunID: "a", Prompt: "a"})
		done <- err
	}()
	waitEntered(t, large.entered)
	_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sessionID, RunID: "y", Prompt: "y"})
	require.NoError(t, err)
	require.Equal(t, 1, sa.QueuedPrompts(sessionID))

	sa.CancelRun(sessionID, "a")
	require.True(t, waitRunComplete(t, ch, "a").Cancelled)
	y, ok := waitRunCompleteFor(ch, "y", 5*time.Second)
	require.True(t, ok, "the queued prompt was stranded (queued=%d)", sa.QueuedPrompts(sessionID))
	require.False(t, y.Cancelled)
	require.Empty(t, y.Error)
	<-done
	require.Zero(t, sa.QueuedPrompts(sessionID))
	require.False(t, sa.IsSessionBusy(sessionID))
}

// Lượt lấy ra từ hàng đợi rồi bị huỷ ngay khi vào Run (cancel-on-entry) vẫn
// chuyển tiếp cho prompt kế (người soát độc lập, P2-5).
func TestCancelOnEntryHandsOffQueue(t *testing.T) {
	large := &gatedStreamModel{text: "done", gate: make(chan struct{}), entered: make(chan struct{})}
	close(large.gate)
	sa, broker, sessionID, _ := cancelRunAgent(t, large)
	ch := broker.Subscribe(t.Context())

	// Hàng đợi còn y trong khi session rảnh: đúng trạng thái giữa lúc lấy x
	// ra và lúc x vào Run.
	sa.enqueueCall(SessionAgentCall{SessionID: sessionID, RunID: "y", Prompt: "y"})
	sa.CancelRun(sessionID, "x")
	_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sessionID, RunID: "x", Prompt: "x"})
	require.NoError(t, err)

	require.True(t, waitRunComplete(t, ch, "x").Cancelled)
	y, ok := waitRunCompleteFor(ch, "y", 5*time.Second)
	require.True(t, ok, "the rest of the queue was stranded (queued=%d)", sa.QueuedPrompts(sessionID))
	require.False(t, y.Cancelled)
	require.Zero(t, sa.QueuedPrompts(sessionID))
}

// summarizeGateModel: lần 1 trả tool call với usage lớn (ép tự tóm tắt), lần
// 2 (bản tóm tắt) chờ tới khi ctx huỷ hoặc gate mở, từ lần 3 trả "done".
type summarizeGateModel struct {
	n       atomic.Int64
	entered chan struct{}
	gate    chan struct{}
}

func (m *summarizeGateModel) Provider() string { return "fake" }
func (m *summarizeGateModel) Model() string    { return "fake-model" }
func (m *summarizeGateModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{Content: fantasy.ResponseContent{fantasy.TextContent{Text: "title"}}, FinishReason: fantasy.FinishReasonStop}, nil
}

func (m *summarizeGateModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not supported")
}

func (m *summarizeGateModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not supported")
}

func (m *summarizeGateModel) Stream(ctx context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
	n := m.n.Add(1)
	if n == 1 {
		return func(yield func(fantasy.StreamPart) bool) {
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputStart, ID: "tc1", ToolCallName: "noop"}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputEnd, ID: "tc1"}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: "tc1", ToolCallName: "noop", ToolCallInput: "{}"}) {
				return
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls,
				Usage: fantasy.Usage{InputTokens: 900, OutputTokens: 60, TotalTokens: 960}})
		}, nil
	}
	text := "done"
	if n == 2 {
		close(m.entered)
		select {
		case <-m.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		text = "summary"
	}
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: text}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func modelWith(m fantasy.LanguageModel, provider, id string, cw catwalk.Model) Model {
	cw.ID = id
	return Model{Model: m, CatwalkCfg: cw, ModelCfg: config.SelectedModel{Provider: provider, Model: id}}
}

// Huỷ lượt lúc nó đang tự tóm tắt thì lượt dừng thật: trước đây summarize
// nuốt lỗi huỷ, Run xếp lại lượt và chạy tiếp như chưa có gì (P2-6).
func TestCancelRunDuringAutoSummarizeStopsRun(t *testing.T) {
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	m := &summarizeGateModel{entered: make(chan struct{}), gate: make(chan struct{})}
	cw := catwalk.Model{ContextWindow: 1000, DefaultMaxTokens: 100}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  modelWith(m, "prov", "model", cw),
		SmallModel:  modelWith(&finishStreamModel{text: "title"}, "prov", "small", cw),
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
		Tools:       []fantasy.AgentTool{namedTool("noop")},
	}).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	ch := broker.Subscribe(t.Context())

	done := make(chan error, 1)
	go func() {
		_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "x", Prompt: "work"})
		done <- err
	}()
	waitEntered(t, m.entered)
	sa.CancelRun(sess.ID, "x")

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}
	rc := waitRunComplete(t, ch, "x")
	require.True(t, rc.Cancelled, "the run carried on after its summary was cancelled (model calls=%d)", m.n.Load())
	require.EqualValues(t, 2, m.n.Load(), "no model call after the cancel")
}
