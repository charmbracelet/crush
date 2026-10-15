package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// rawToolCallModel: bước 1 phát một tool call có arguments JSON HỎNG (chuỗi
// chưa đóng) tới một tool KHÔNG tồn tại, nên fantasy đánh Invalid và giữ
// nguyên Input thô trong step result. Bước 2 ghi lại messages nhận được rồi
// kết thúc. Mô phỏng sự cố 15/10 (write arguments bị cắt).
type rawToolCallModel struct {
	mu    sync.Mutex
	steps [][]fantasy.Message
}

const malformedArgs = `{"content": "unfinished`

func (m *rawToolCallModel) Provider() string { return "fake" }
func (m *rawToolCallModel) Model() string    { return "fake-model" }
func (m *rawToolCallModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{Content: fantasy.ResponseContent{fantasy.TextContent{Text: "t"}}, FinishReason: fantasy.FinishReasonStop}, nil
}
func (m *rawToolCallModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("na")
}
func (m *rawToolCallModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("na")
}

func (m *rawToolCallModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	m.steps = append(m.steps, call.Prompt)
	step := len(m.steps)
	m.mu.Unlock()
	if step == 1 {
		return func(yield func(fantasy.StreamPart) bool) {
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputStart, ID: "call_raw", ToolCallName: "ghost"}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputEnd, ID: "call_raw"}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: "call_raw", ToolCallName: "ghost", ToolCallInput: malformedArgs}) {
				return
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
		}, nil
	}
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
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

// secondStepMessages trả messages model nhận ở bước 2 (nil nếu chưa có).
func (m *rawToolCallModel) secondStepMessages() []fantasy.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.steps) < 2 {
		return nil
	}
	return m.steps[1]
}

// Arguments JSON hỏng của bước 1 KHÔNG được gửi lại ở request bước 2. Trên
// code chưa có cổng ở PrepareStep, bước 2 mang nguyên chuỗi thô → đây là cái
// làm provider trả HTTP 400. Bỏ 3 dòng sanitizeOutgoingToolCalls thì test này
// đỏ lại (thử đột biến).
func TestMalformedToolCallArgsNotResent(t *testing.T) {
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	m := &rawToolCallModel{}
	cw := catwalk.Model{ContextWindow: 100000, DefaultMaxTokens: 1000}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  modelWith(m, "prov", "model", cw),
		SmallModel:  modelWith(&finishStreamModel{text: "title"}, "prov", "small", cw),
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
	}).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "s")
	require.NoError(t, err)
	ch := broker.Subscribe(t.Context())

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "x", Prompt: "write the file"})
	require.NoError(t, err)
	waitRunComplete(t, ch, "x")

	second := m.secondStepMessages()
	require.NotEmpty(t, second, "the turn must reach a second step after the bad tool call")

	var raw, fixed int
	for _, msg := range second {
		for _, part := range msg.Content {
			tc, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](part)
			if !ok || tc.ToolCallID != "call_raw" {
				continue
			}
			if strings.Contains(tc.Input, "unfinished") {
				raw++
			}
			if tc.Input == "{}" {
				fixed++
			}
		}
	}
	require.Zero(t, raw, "the malformed arguments must not be sent to the provider again")
	require.Positive(t, fixed, "the bad tool call is sent as {}")
	// Và không message nào mang chuỗi JSON chưa đóng dưới bất kỳ dạng nào.
	for _, msg := range second {
		for _, part := range msg.Content {
			if tc, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](part); ok {
				require.NotContains(t, tc.Input, malformedArgs)
			}
		}
	}
}
