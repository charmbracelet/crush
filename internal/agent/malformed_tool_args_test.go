package agent

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// badArgsModel: request đầu trả một lời gọi tool "write" với arguments args
// (có thể là JSON hỏng), các request sau trả "done". Ghi lại arguments của
// mọi tool call nằm trong lịch sử gửi lên ở từng request.
type badArgsModel struct {
	args string

	mu       sync.Mutex
	requests int
	sentArgs []string
}

func (m *badArgsModel) Provider() string { return "fake" }
func (m *badArgsModel) Model() string    { return "fake-model" }
func (m *badArgsModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{Content: fantasy.ResponseContent{fantasy.TextContent{Text: "title"}}, FinishReason: fantasy.FinishReasonStop}, nil
}

func (m *badArgsModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not supported")
}

func (m *badArgsModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not supported")
}

func (m *badArgsModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	m.requests++
	n := m.requests
	for _, msg := range call.Prompt {
		for _, part := range msg.Content {
			if tc, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](part); ok {
				m.sentArgs = append(m.sentArgs, tc.Input)
			}
		}
	}
	m.mu.Unlock()
	if n == 1 {
		return func(yield func(fantasy.StreamPart) bool) {
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputStart, ID: "tc1", ToolCallName: "write"}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputEnd, ID: "tc1"}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: "tc1", ToolCallName: "write", ToolCallInput: m.args}) {
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

type writeArgs struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// runBadArgs chạy một lượt mà model gọi "write" với args; trả model và các
// nội dung tool "write" đã thực sự ghi.
func runBadArgs(t *testing.T, args string) (*badArgsModel, []string) {
	t.Helper()
	m := &badArgsModel{args: args}
	var mu sync.Mutex
	var written []string
	write := fantasy.NewAgentTool("write", "write a file", func(_ context.Context, in writeArgs, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
		mu.Lock()
		written = append(written, in.Content)
		mu.Unlock()
		return fantasy.NewTextResponse("ok"), nil
	})

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
		Tools:       []fantasy.AgentTool{write},
	}).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	ch := broker.Subscribe(t.Context())

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "x", Prompt: "write the file"})
	require.NoError(t, err)
	rc := waitRunComplete(t, ch, "x")
	require.Empty(t, rc.Error)

	mu.Lock()
	defer mu.Unlock()
	return m, append([]string(nil), written...)
}

// Arguments hỏng (không sửa được thành lời gọi hợp lệ) không được quay lại
// provider ở bước kế tiếp: vLLM parse lại arguments trong lịch sử và trả 400
// "Unterminated string", làm chết cả phiên.
func TestMalformedToolArgsNeverSentBackToProvider(t *testing.T) {
	m, written := runBadArgs(t, `{"content": "unfinished`)

	require.Empty(t, written, "a malformed call must not run")
	require.Equal(t, 2, m.requests, "the model gets the error and answers")
	require.NotEmpty(t, m.sentArgs)
	for _, a := range m.sentArgs {
		require.True(t, json.Valid([]byte(a)), "tool call arguments sent to the provider must be valid JSON, got %q", a)
	}
}

// Arguments bị cắt giữa chừng không được "sửa" bằng cách đoán phần thiếu rồi
// chạy: đóng chuỗi cho JSON hợp lệ sẽ ghi một tệp cụt mà không ai biết.
func TestTruncatedToolArgsAreNotRepairedAndRun(t *testing.T) {
	m, written := runBadArgs(t, `{"file_path": "main.go", "content": "package main\n\nfunc main() {`)

	require.Empty(t, written, "a truncated call must not run with guessed arguments")
	require.Equal(t, 2, m.requests, "the model gets the error and can retry")
	for _, a := range m.sentArgs {
		require.True(t, json.Valid([]byte(a)), "got %q", a)
	}
}
