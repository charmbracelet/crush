package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

type foldedPromptProbeModel struct {
	mu            sync.Mutex
	calls         []fantasy.Call
	secondEntered chan struct{}
	releaseSecond chan struct{}
}

func (m *foldedPromptProbeModel) Provider() string { return "fake" }
func (m *foldedPromptProbeModel) Model() string    { return "folded-prompt-probe" }

func (m *foldedPromptProbeModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *foldedPromptProbeModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	callNumber := len(m.calls)
	call.Prompt = cloneFantasyMessages([]fantasy.Message(call.Prompt))
	m.calls = append(m.calls, call)
	m.mu.Unlock()

	if callNumber == 1 {
		close(m.secondEntered)
		<-m.releaseSecond
	}

	return func(yield func(fantasy.StreamPart) bool) {
		if callNumber < 3 {
			yield(fantasy.StreamPart{
				Type:          fantasy.StreamPartTypeToolCall,
				ID:            fmt.Sprintf("folded-call-%d", callNumber),
				ToolCallName:  "echo",
				ToolCallInput: fmt.Sprintf(`{"message":"folded step %d"}`, callNumber),
			})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
			return
		}

		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "folded-final"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "folded-final", Delta: "Finished."})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "folded-final"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (m *foldedPromptProbeModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *foldedPromptProbeModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *foldedPromptProbeModel) recordedCalls() []fantasy.Call {
	m.mu.Lock()
	defer m.mu.Unlock()

	calls := make([]fantasy.Call, len(m.calls))
	for i, call := range m.calls {
		calls[i] = call
		calls[i].Prompt = cloneFantasyMessages(call.Prompt)
	}
	return calls
}

func TestRun_PreservesFoldedQueuedPromptAcrossAutonomousSteps(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	model := &foldedPromptProbeModel{
		secondEntered: make(chan struct{}),
		releaseSecond: make(chan struct{}),
	}
	small := &finishStreamModel{text: "title"}
	echo := fantasy.NewAgentTool(
		"echo",
		"Echo the input.",
		func(_ context.Context, input struct {
			Message string `json:"message"`
		}, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("Echo: " + input.Message), nil
		},
	)
	agent := testSessionAgent(env, model, small, "system", echo)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	firstDone := make(chan error, 1)
	go func() {
		_, runErr := agent.Run(t.Context(), SessionAgentCall{
			SessionID: sess.ID,
			Prompt:    "initial task",
		})
		firstDone <- runErr
	}()

	select {
	case <-model.secondEntered:
	case <-t.Context().Done():
		t.Fatal("second autonomous step did not start")
	}

	const queuedPrompt = "Please pause and report progress."
	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    queuedPrompt,
	})
	require.NoError(t, err)
	close(model.releaseSecond)
	require.NoError(t, <-firstDone)

	calls := model.recordedCalls()
	require.Len(t, calls, 4)

	foldedWirePrompt := openAICompatMessages(t, calls[2].Prompt)
	nextWirePrompt := openAICompatMessages(t, calls[3].Prompt)
	require.Greater(t, len(nextWirePrompt), len(foldedWirePrompt))
	require.Equal(t, foldedWirePrompt, nextWirePrompt[:len(foldedWirePrompt)])

	queuedMessage := foldedWirePrompt[len(foldedWirePrompt)-1]
	var decoded struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	require.NoError(t, json.Unmarshal(queuedMessage, &decoded))
	require.Equal(t, "user", decoded.Role)
	require.Equal(t, queuedPrompt, decoded.Content)
}
