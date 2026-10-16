package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// scriptModel chạy theo kịch bản cho các request CÔNG VIỆC (request tóm tắt
// không tiêu kịch bản): "cut" = trả chữ rồi bị cắt vì hết output (finish
// length); "tool" = gọi tool với usage lớn (ép tự tóm tắt); "done" = xong.
// Request tóm tắt trả summaryText (rỗng = không có chữ nào). Nếu gate != nil
// thì request công việc đầu tiên chờ gate để test xếp prompt vào hàng đợi.
type scriptModel struct {
	script      []string
	summaryText string
	gate        chan struct{}
	entered     chan struct{}

	mu      sync.Mutex
	work    int
	kinds   []string
	prompts []string
}

func (m *scriptModel) Provider() string { return "fake" }
func (m *scriptModel) Model() string    { return "fake-model" }
func (m *scriptModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{Content: fantasy.ResponseContent{fantasy.TextContent{Text: "title"}}, FinishReason: fantasy.FinishReasonStop}, nil
}

func (m *scriptModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not supported")
}

func (m *scriptModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not supported")
}

func (m *scriptModel) seen() (kinds, prompts []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.kinds...), append([]string(nil), m.prompts...)
}

func textStream(text string, reason fantasy.FinishReason, usage fantasy.Usage) fantasy.StreamResponse {
	return func(yield func(fantasy.StreamPart) bool) {
		if text != "" {
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) ||
				!yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: text}) ||
				!yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
				return
			}
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: reason, Usage: usage})
	}
}

func (m *scriptModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	isSummary := false
	for _, msg := range call.Prompt {
		if msg.Role != fantasy.MessageRoleSystem {
			continue
		}
		for _, part := range msg.Content {
			if tp, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok && strings.Contains(tp.Text, "summar") {
				isSummary = true
			}
		}
	}
	last := lastUserText(call.Prompt)
	m.mu.Lock()
	if isSummary {
		m.kinds = append(m.kinds, "summary")
		m.prompts = append(m.prompts, last)
		m.mu.Unlock()
		return textStream(m.summaryText, fantasy.FinishReasonStop, fantasy.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}), nil
	}
	step := "done"
	if m.work < len(m.script) {
		step = m.script[m.work]
	}
	m.work++
	n := m.work
	first := n == 1
	m.kinds = append(m.kinds, "work")
	m.prompts = append(m.prompts, last)
	m.mu.Unlock()

	if first && m.gate != nil {
		close(m.entered)
		select {
		case <-m.gate:
		case <-ctx.Done():
		}
	}
	small := fantasy.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}
	switch step {
	case "cut":
		return textStream("part ", fantasy.FinishReasonLength, small), nil
	case "tool", "tool_small":
		id := fmt.Sprintf("tc%d", n)
		usage := fantasy.Usage{InputTokens: 900, OutputTokens: 60, TotalTokens: 960}
		if step == "tool_small" {
			usage = small
		}
		return func(yield func(fantasy.StreamPart) bool) {
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputStart, ID: id, ToolCallName: "noop"}) ||
				!yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputEnd, ID: id}) ||
				!yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: id, ToolCallName: "noop", ToolCallInput: "{}"}) {
				return
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls, Usage: usage})
		}, nil
	}
	return textStream("done", fantasy.FinishReasonStop, small), nil
}

type contHarness struct {
	sa    *sessionAgent
	env   fakeEnv
	ch    <-chan pubsub.Event[notify.RunComplete]
	model *scriptModel
	sid   string
}

func newContHarness(t *testing.T, m *scriptModel) *contHarness {
	t.Helper()
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
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
	return &contHarness{sa: sa, env: env, ch: broker.Subscribe(t.Context()), model: m, sid: sess.ID}
}

// Bản tóm tắt rỗng (model suy luận ăn hết ngân sách nên không ra chữ nào) không
// được thành điểm cắt lịch sử: toàn bộ ngữ cảnh sẽ bị thay bằng chuỗi rỗng mà
// không ai hay. Phải coi như tóm tắt hỏng: giữ lịch sử, làm tiếp.
func TestEmptySummaryKeepsHistory(t *testing.T) {
	h := newContHarness(t, &scriptModel{script: []string{"tool", "done"}, summaryText: ""})

	_, err := h.sa.Run(t.Context(), SessionAgentCall{SessionID: h.sid, RunID: "x", Prompt: "refactor the scanner"})
	require.NoError(t, err)
	rc := waitRunComplete(t, h.ch, "x")
	require.Empty(t, rc.Error)
	require.Equal(t, "done", rc.Text)

	sess, err := h.env.sessions.Get(t.Context(), h.sid)
	require.NoError(t, err)
	require.Empty(t, sess.SummaryMessageID, "an empty summary must not become the history cut point")
}

// Hai lần tóm tắt liên tiếp trong cùng một việc không được bọc "yêu cầu gốc"
// thành chuỗi lồng nhau: prompt làm tiếp phải mang đúng yêu cầu thật của
// người dùng, mỗi lớp chỉ một lần.
func TestRepeatedSummaryDoesNotNestThePrompt(t *testing.T) {
	h := newContHarness(t, &scriptModel{script: []string{"tool", "tool", "done"}, summaryText: "SUMMARY"})

	_, err := h.sa.Run(t.Context(), SessionAgentCall{SessionID: h.sid, RunID: "x", Prompt: "refactor the scanner"})
	require.NoError(t, err)
	waitRunComplete(t, h.ch, "x")

	kinds, prompts := h.model.seen()
	require.Equal(t, []string{"work", "summary", "work", "summary", "work"}, kinds)
	last := prompts[len(prompts)-1]
	require.Equal(t, 1, strings.Count(last, "interrupted because it got too long"), "one wrapper only, got: %q", last)
	require.Equal(t, 1, strings.Count(last, "refactor the scanner"), "the real request exactly once, got: %q", last)
}

// Trả lời bị cắt rồi tới lượt tóm tắt: "yêu cầu gốc" trong prompt làm tiếp vẫn
// là yêu cầu của người dùng, không phải lời nhắc "reply was cut off".
func TestSummaryAfterLengthContinueKeepsTheRealRequest(t *testing.T) {
	h := newContHarness(t, &scriptModel{script: []string{"cut", "tool", "done"}, summaryText: "SUMMARY"})

	_, err := h.sa.Run(t.Context(), SessionAgentCall{SessionID: h.sid, RunID: "x", Prompt: "refactor the scanner"})
	require.NoError(t, err)
	waitRunComplete(t, h.ch, "x")

	kinds, prompts := h.model.seen()
	require.Equal(t, []string{"work", "work", "summary", "work"}, kinds)
	last := prompts[len(prompts)-1]
	require.Contains(t, last, "refactor the scanner")
	require.NotContains(t, last, "cut off", "the carried request must not be the continuation nudge")
}

// Lượt tự làm tiếp không được gửi/lưu lại attachment của prompt gốc: lịch sử đã
// có chúng, mỗi lần làm tiếp thêm một bản nữa làm phình context.
func TestContinuationDoesNotResendAttachments(t *testing.T) {
	h := newContHarness(t, &scriptModel{script: []string{"cut", "cut", "done"}})
	att := message.Attachment{FilePath: "/tmp/a.png", FileName: "a.png", MimeType: "image/png", Content: []byte{1, 2, 3}}

	_, err := h.sa.Run(t.Context(), SessionAgentCall{SessionID: h.sid, RunID: "x", Prompt: "look at this", Attachments: []message.Attachment{att}})
	require.NoError(t, err)
	waitRunComplete(t, h.ch, "x")

	msgs, err := h.env.messages.List(t.Context(), h.sid)
	require.NoError(t, err)
	withBinary := 0
	for _, msg := range msgs {
		if msg.Role == message.User && len(msg.BinaryContent()) > 0 {
			withBinary++
		}
	}
	require.Equal(t, 1, withBinary, "the attachment belongs to the user's own message only")
}

// Prompt người dùng gõ thêm trong lúc lượt A chạy xếp hàng sau A; lượt tự làm
// tiếp CỦA A phải chạy trước nó, không bị đẩy xuống sau prompt mới.
func TestContinuationRunsBeforeQueuedUserPrompt(t *testing.T) {
	m := &scriptModel{script: []string{"cut", "done", "done"}, gate: make(chan struct{}), entered: make(chan struct{})}
	h := newContHarness(t, m)

	done := make(chan error, 1)
	go func() {
		_, err := h.sa.Run(t.Context(), SessionAgentCall{SessionID: h.sid, RunID: "a", Prompt: "task A"})
		done <- err
	}()
	waitEntered(t, m.entered)
	_, err := h.sa.Run(t.Context(), SessionAgentCall{SessionID: h.sid, RunID: "b", Prompt: "task B"})
	require.NoError(t, err)
	close(m.gate)
	require.NoError(t, <-done)
	waitRunComplete(t, h.ch, "b")

	_, prompts := m.seen()
	require.Equal(t, []string{"task A", lengthContinuePrompt, "task B"}, prompts)
}

// Trần làm tiếp chỉ để chặn model cứ viết quá dài mà KHÔNG tiến triển. Mỗi
// lượt bị cắt nhưng đã chạy xong ít nhất một tool call là có tiến triển: đếm lại
// từ đầu, nếu không một việc dài trên model local bị cắt rải rác sẽ dừng sớm
// sau 3 lần dù lần nào cũng làm được việc.
func TestLengthContinueCapResetsWhenTheRunMakesProgress(t *testing.T) {
	var script []string
	for i := 0; i <= maxLengthContinues; i++ { // maxLengthContinues+1 lần bị cắt
		script = append(script, "tool_small", "cut")
	}
	script = append(script, "done")
	h := newContHarness(t, &scriptModel{script: script})

	_, err := h.sa.Run(t.Context(), SessionAgentCall{SessionID: h.sid, RunID: "x", Prompt: "long job"})
	require.NoError(t, err)
	rc := waitRunComplete(t, h.ch, "x")
	require.Equal(t, "done", rc.Text, "progress between cuts must not exhaust the continuation cap")
}
