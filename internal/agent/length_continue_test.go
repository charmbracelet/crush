package agent

import (
	"context"
	"errors"
	"sync"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// lengthModel trả lời bị cắt (finish = length) cutoffs lần đầu, sau đó trả
// "done"; ghi lại prompt người dùng cuối của từng request.
type lengthModel struct {
	cutoffs int
	mu      sync.Mutex
	prompts []string
}

func (m *lengthModel) Provider() string { return "fake" }
func (m *lengthModel) Model() string    { return "fake-model" }
func (m *lengthModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{Content: fantasy.ResponseContent{fantasy.TextContent{Text: "title"}}, FinishReason: fantasy.FinishReasonStop}, nil
}

func (m *lengthModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not supported")
}

func (m *lengthModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not supported")
}

func (m *lengthModel) seen() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.prompts...)
}

func (m *lengthModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	m.prompts = append(m.prompts, lastUserText(call.Prompt))
	n := len(m.prompts)
	m.mu.Unlock()
	text, reason := "part ", fantasy.FinishReasonLength
	if n > m.cutoffs {
		text, reason = "done", fantasy.FinishReasonStop
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
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: reason})
	}, nil
}

func lengthAgent(t *testing.T, m fantasy.LanguageModel) (*sessionAgent, <-chan pubsub.Event[notify.RunComplete], string) {
	t.Helper()
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	cw := catwalk.Model{ContextWindow: 100000, DefaultMaxTokens: 100}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  modelWith(m, "prov", "model", cw),
		SmallModel:  modelWith(&finishStreamModel{text: "title"}, "prov", "small", cw),
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
	}).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	return sa, broker.Subscribe(t.Context()), sess.ID
}

// Trả lời bị cắt vì hết ngân sách output: lượt tự làm tiếp (cùng RunID) tới
// khi xong, thay vì dừng chờ người dùng gõ "tiếp tục".
func TestReplyCutOffAtOutputLimitContinues(t *testing.T) {
	m := &lengthModel{cutoffs: 2}
	sa, ch, sid := lengthAgent(t, m)

	_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sid, RunID: "x", Prompt: "write the big file"})
	require.NoError(t, err)
	rc := waitRunComplete(t, ch, "x")
	require.Empty(t, rc.Error)
	require.Equal(t, "done", rc.Text, "the RunID completes with the final continuation")

	got := m.seen()
	require.Len(t, got, 3)
	require.Equal(t, "write the big file", got[0])
	require.Equal(t, lengthContinuePrompt, got[1])
	require.Equal(t, lengthContinuePrompt, got[2])
	require.Zero(t, sa.QueuedPrompts(sid))
}

// Model cứ viết quá dài: dừng sau maxLengthContinues lần làm tiếp, không lặp
// mãi; prompt mới của người dùng có lại đủ số lần.
func TestOutputLimitContinuationIsBounded(t *testing.T) {
	m := &lengthModel{cutoffs: 1000}
	sa, ch, sid := lengthAgent(t, m)

	_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sid, RunID: "x", Prompt: "endless"})
	require.NoError(t, err)
	waitRunComplete(t, ch, "x")
	require.Len(t, m.seen(), 1+maxLengthContinues)

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sid, RunID: "y", Prompt: "again"})
	require.NoError(t, err)
	waitRunComplete(t, ch, "y")
	require.Len(t, m.seen(), 2*(1+maxLengthContinues))
}
