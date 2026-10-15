package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// recordingModel ghi lại mỗi lần Stream: số lần gọi và tên các tool gửi kèm.
// Nếu toolFirst thì lần Stream đầu trả một lời gọi tool "noop" (để lượt có
// bước thứ hai), các lần sau trả chữ rồi kết thúc. Lần Stream đầu chờ gate
// (nếu có) để test đổi model/tool đúng lúc lượt đang chạy.
type recordingModel struct {
	name      string
	toolFirst bool
	gate      chan struct{}
	entered   chan struct{}

	mu    sync.Mutex
	calls int
	tools [][]string
}

func (m *recordingModel) Provider() string { return "fake" }
func (m *recordingModel) Model() string    { return m.name }

func (m *recordingModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: "title"}},
		FinishReason: fantasy.FinishReasonStop,
	}, nil
}

func (m *recordingModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	m.calls++
	n := m.calls
	var names []string
	for _, t := range call.Tools {
		names = append(names, t.GetName())
	}
	m.tools = append(m.tools, names)
	m.mu.Unlock()

	if n == 1 && m.gate != nil {
		close(m.entered)
		select {
		case <-m.gate:
		case <-ctx.Done():
		}
	}
	if n == 1 && m.toolFirst {
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
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
		}, nil
	}
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: m.name}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (m *recordingModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *recordingModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *recordingModel) snapshot() (int, [][]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls, append([][]string(nil), m.tools...)
}

type noopInput struct{}

func namedTool(name string) fantasy.AgentTool {
	return fantasy.NewAgentTool(name, "test tool", func(context.Context, noopInput, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("ok"), nil
	})
}

func testModel(m fantasy.LanguageModel, provider, id string) Model {
	return Model{
		Model:      m,
		CatwalkCfg: catwalk.Model{ID: id, ContextWindow: 200000, DefaultMaxTokens: 10000},
		ModelCfg:   config.SelectedModel{Provider: provider, Model: id},
	}
}

func waitEntered(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("run never entered Stream")
	}
}

// Đổi model chung khi một lượt đang chạy (session khác chọn model khác, hay
// `agent-cli run --model`) không được chuyển các bước còn lại của lượt đó
// sang provider mới.
func TestRun_ModelSwitchMidRunKeepsRunModel(t *testing.T) {
	t.Parallel()
	env := testEnv(t)

	x := &recordingModel{name: "model-x", toolFirst: true, gate: make(chan struct{}), entered: make(chan struct{})}
	y := &recordingModel{name: "model-y"}
	small := &finishStreamModel{text: "title"}

	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel: testModel(x, "prov-x", "model-x"),
		SmallModel: testModel(small, "prov-x", "small"),
		IsYolo:     true,
		Sessions:   env.sessions,
		Messages:   env.messages,
		Tools:      []fantasy.AgentTool{namedTool("noop")},
	}).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, runErr := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "do it"})
		done <- runErr
	}()
	waitEntered(t, x.entered)

	// Như updateAgentModels khi một prompt được gửi ở session khác: dựng lại
	// model VÀ bộ tool (tool `agent` mang agent con của model mới).
	sa.SetModels(testModel(y, "prov-y", "model-y"), testModel(small, "prov-y", "small"))
	sa.SetTools([]fantasy.AgentTool{namedTool("noop"), namedTool("built-for-model-y")})
	close(x.gate)
	require.NoError(t, <-done)

	xCalls, xTools := x.snapshot()
	yCalls, _ := y.snapshot()
	require.Equal(t, 2, xCalls, "both steps of the run must use the model it started with")
	require.Zero(t, yCalls, "the run must not switch provider mid-run")
	require.Equal(t, []string{"noop"}, xTools[1], "later steps must keep the run's tools, not tools built for another model")
}

// Bản dựng lại của CHÍNH model đang chạy (sau khi làm mới token) thì được
// dùng cho bước sau: ghim model không được chặn đường làm mới credential.
func TestRun_RebuiltSameModelIsUsedForLaterSteps(t *testing.T) {
	t.Parallel()
	env := testEnv(t)

	x := &recordingModel{name: "model-x", toolFirst: true, gate: make(chan struct{}), entered: make(chan struct{})}
	xRefreshed := &recordingModel{name: "model-x"}
	small := &finishStreamModel{text: "title"}

	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel: testModel(x, "prov-x", "model-x"),
		SmallModel: testModel(small, "prov-x", "small"),
		IsYolo:     true,
		Sessions:   env.sessions,
		Messages:   env.messages,
		Tools:      []fantasy.AgentTool{namedTool("noop")},
	}).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, runErr := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "do it"})
		done <- runErr
	}()
	waitEntered(t, x.entered)

	sa.SetModels(testModel(xRefreshed, "prov-x", "model-x"), testModel(small, "prov-x", "small"))
	close(x.gate)
	require.NoError(t, <-done)

	xCalls, _ := x.snapshot()
	refreshedCalls, _ := xRefreshed.snapshot()
	require.Equal(t, 1, xCalls)
	require.Equal(t, 1, refreshedCalls, "a rebuilt instance of the same provider/model must serve the next step")
}

// Prompt xếp hàng sau một lượt đang chạy giữ model và bộ tool của lúc gửi,
// kể cả khi model/tool chung đổi trước khi nó tới lượt.
func TestRun_QueuedPromptKeepsModelAndToolsAtSendTime(t *testing.T) {
	t.Parallel()
	env := testEnv(t)

	x := &recordingModel{name: "model-x", gate: make(chan struct{}), entered: make(chan struct{})}
	y := &recordingModel{name: "model-y"}
	small := &finishStreamModel{text: "title"}

	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel: testModel(x, "prov-x", "model-x"),
		SmallModel: testModel(small, "prov-x", "small"),
		IsYolo:     true,
		Sessions:   env.sessions,
		Messages:   env.messages,
		Tools:      []fantasy.AgentTool{namedTool("tool-a")},
	}).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, runErr := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "first"})
		done <- runErr
	}()
	waitEntered(t, x.entered)

	// Gửi prompt thứ hai khi model còn là X: nó phải chạy bằng X và tool-a.
	res, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "second"})
	require.NoError(t, err)
	require.Nil(t, res)
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID))

	sa.SetModels(testModel(y, "prov-y", "model-y"), testModel(small, "prov-y", "small"))
	sa.SetTools([]fantasy.AgentTool{namedTool("tool-b")})
	close(x.gate)
	require.NoError(t, <-done)

	require.Eventually(t, func() bool { return !sa.IsSessionBusy(sess.ID) && sa.QueuedPrompts(sess.ID) == 0 },
		5*time.Second, 10*time.Millisecond)

	xCalls, xTools := x.snapshot()
	yCalls, _ := y.snapshot()
	require.Equal(t, 2, xCalls, "the queued prompt must run on the model selected when it was sent")
	require.Zero(t, yCalls)
	require.Equal(t, []string{"tool-a"}, xTools[1], "the queued prompt must keep the tools of its send time")
}

// Prompt gửi SAU khi model đã đổi không gộp vào lượt đang chạy bằng model
// cũ: nó chạy thành lượt riêng bằng model mới.
func TestDrainQueueForStep_KeepsPromptsPinnedToAnotherModel(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{Sessions: env.sessions, Messages: env.messages}).(*sessionAgent)

	const sessionID = "pinned-drain"
	same := Model{ModelCfg: config.SelectedModel{Provider: "prov-x", Model: "model-x"}}
	other := Model{ModelCfg: config.SelectedModel{Provider: "prov-y", Model: "model-y"}}
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, Prompt: "same", pinnedModel: &same},
		{SessionID: sessionID, Prompt: "other", pinnedModel: &other},
		{SessionID: sessionID, Prompt: "unpinned"},
	})

	fold, _ := a.drainQueueForStep(sessionID, same.ModelCfg)
	var folded []string
	for _, c := range fold {
		folded = append(folded, c.Prompt)
	}
	require.Equal(t, []string{"same", "unpinned"}, folded)
	kept, _ := a.messageQueue.Get(sessionID)
	require.Len(t, kept, 1)
	require.Equal(t, "other", kept[0].Prompt)
}

// Lượt phi tương tác (agent-cli run) không có tool question: không ai trả
// lời được, model phải làm tiếp thay vì dừng chờ.
func TestRun_NonInteractiveDropsQuestionTool(t *testing.T) {
	t.Parallel()
	for _, nonInteractive := range []bool{true, false} {
		env := testEnv(t)
		m := &recordingModel{name: "model-x"}
		sa := NewSessionAgent(SessionAgentOptions{
			LargeModel: testModel(m, "prov-x", "model-x"),
			SmallModel: testModel(&finishStreamModel{text: "title"}, "prov-x", "small"),
			IsYolo:     true,
			Sessions:   env.sessions,
			Messages:   env.messages,
			Tools:      []fantasy.AgentTool{namedTool("question"), namedTool("noop")},
		}).(*sessionAgent)
		sess, err := env.sessions.Create(t.Context(), "session")
		require.NoError(t, err)

		_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "do it", NonInteractive: nonInteractive})
		require.NoError(t, err)
		_, tools := m.snapshot()
		require.NotEmpty(t, tools)
		if nonInteractive {
			require.Equal(t, []string{"noop"}, tools[0])
		} else {
			require.Equal(t, []string{"question", "noop"}, tools[0])
		}
	}
}
