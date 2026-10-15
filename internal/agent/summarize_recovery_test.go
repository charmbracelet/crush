package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// summaryFailModel: lần 1 trả tool call với usage lớn (ép tự tóm tắt); request
// tóm tắt (system prompt của summarize) hỏng như provider im lặng quá hạn; các
// request khác trả "done". Ghi lại thứ tự để biết có thử tóm tắt lại không.
type summaryFailModel struct {
	n        atomic.Int64
	mu       sync.Mutex
	kinds    []string
	prompts  []string
	failWith error
}

func (m *summaryFailModel) Provider() string { return "fake" }
func (m *summaryFailModel) Model() string    { return "fake-model" }
func (m *summaryFailModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{Content: fantasy.ResponseContent{fantasy.TextContent{Text: "title"}}, FinishReason: fantasy.FinishReasonStop}, nil
}

func (m *summaryFailModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not supported")
}

func (m *summaryFailModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not supported")
}

func (m *summaryFailModel) record(kind, prompt string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.kinds = append(m.kinds, kind)
	m.prompts = append(m.prompts, prompt)
}

func (m *summaryFailModel) seen() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.kinds...)
}

func (m *summaryFailModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	n := m.n.Add(1)
	isSummary := false
	for _, msg := range call.Prompt {
		if msg.Role == fantasy.MessageRoleSystem {
			for _, part := range msg.Content {
				if tp, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok && strings.Contains(tp.Text, "summar") {
					isSummary = true
				}
			}
		}
	}
	last := lastUserText(call.Prompt)
	if isSummary {
		m.record("summary", last)
		return nil, m.failWith
	}
	if n == 1 {
		m.record("work", last)
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
	m.record("work", last)
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: "done"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop,
			Usage: fantasy.Usage{InputTokens: 950, OutputTokens: 10, TotalTokens: 960}})
	}, nil
}

// Tự tóm tắt hỏng (provider im lặng quá hạn) không còn làm dừng việc: lượt
// làm tiếp chạy ngay với lịch sử đầy đủ, không thử tóm tắt lại trong lúc đó,
// và RunComplete báo xong bình thường. Trước đây lỗi tóm tắt trả thẳng ra
// ngoài và công việc dừng giữa chừng.
func TestAutoSummarizeFailureContinuesTheWork(t *testing.T) {
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	m := &summaryFailModel{failWith: &requestTimeoutError{timeout: 6 * time.Minute, idle: true, cause: context.DeadlineExceeded}}
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

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "x", Prompt: "refactor the scanner"})
	require.NoError(t, err, "a failed summary must not end the work")

	rc := waitRunComplete(t, ch, "x")
	require.False(t, rc.Cancelled)
	require.Empty(t, rc.Error)
	require.Equal(t, "done", rc.Text)
	require.Equal(t, []string{"work", "summary", "work"}, m.seen(), "one summary attempt, then the work continues without retrying it")
	require.Contains(t, m.prompts[2], "automatic context summary failed")
	require.Contains(t, m.prompts[2], "refactor the scanner")

	// Bản tóm tắt hỏng còn trong transcript (người dùng thấy lỗi) nhưng không
	// thành điểm cắt lịch sử.
	stored, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Empty(t, stored.SummaryMessageID)
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var failedSummary bool
	for _, msg := range msgs {
		if msg.IsSummaryMessage {
			if f := msg.FinishPart(); f != nil && f.Reason == message.FinishReasonError {
				failedSummary = true
			}
		}
	}
	require.True(t, failedSummary)

	// Prompt mới của người dùng: tự tóm tắt được thử lại.
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "y", Prompt: "next"})
	require.NoError(t, err)
	waitRunComplete(t, ch, "y")
	require.Contains(t, m.seen()[3:], "summary", "a new user prompt tries summarizing again")
}

// Huỷ lúc đang tóm tắt vẫn dừng lượt (không bị coi là tóm tắt hỏng để làm tiếp).
func TestAutoSummarizeCancelStillStops(t *testing.T) {
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	m := &summaryFailModel{failWith: context.Canceled}
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

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "x", Prompt: "work"})
	require.ErrorIs(t, err, context.Canceled)
	require.True(t, waitRunComplete(t, ch, "x").Cancelled)
	require.Equal(t, []string{"work", "summary"}, m.seen())
}

// Request tóm tắt có hạn chờ gấp summaryRequestTimeoutScale lần hạn thường.
func TestSummaryRequestTimeoutIsScaled(t *testing.T) {
	t.Parallel()
	m := requestTimeoutModel{timeout: 2 * time.Minute}
	require.Equal(t, 2*time.Minute, m.budget(t.Context()))
	require.Equal(t, 6*time.Minute, m.budget(withRequestTimeoutScale(t.Context(), summaryRequestTimeoutScale)))
}
