package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/openaicompat"
	"github.com/stretchr/testify/require"
)

type replayProbeModel struct {
	mu            sync.Mutex
	calls         []fantasy.Call
	secondEntered chan struct{}
	releaseSecond chan struct{}
}

func (m *replayProbeModel) Provider() string { return "fake" }
func (m *replayProbeModel) Model() string    { return "replay-probe" }

func (m *replayProbeModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *replayProbeModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	callNumber := len(m.calls)
	call.Prompt = cloneFantasyMessages([]fantasy.Message(call.Prompt))
	m.calls = append(m.calls, call)
	m.mu.Unlock()
	if callNumber == 1 && m.secondEntered != nil {
		close(m.secondEntered)
		<-m.releaseSecond
	}

	return func(yield func(fantasy.StreamPart) bool) {
		switch callNumber {
		case 0:
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeReasoningStart, ID: "reasoning-1", Delta: "Thinking.\n"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeReasoningEnd, ID: "reasoning-1"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text-1"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text-1", Delta: "Working on it.\n\n"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "text-1"})
			yield(fantasy.StreamPart{
				Type:          fantasy.StreamPartTypeToolCall,
				ID:            "call-1",
				ToolCallName:  "echo",
				ToolCallInput: `{"message":"hello"}`,
			})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
		case 1:
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text-2"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text-2", Delta: "\nFinal answer.\n\n"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "text-2"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
		default:
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text-3"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text-3", Delta: "continued"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "text-3"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
		}
	}, nil
}

func (m *replayProbeModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *replayProbeModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *replayProbeModel) recordedCalls() []fantasy.Call {
	m.mu.Lock()
	defer m.mu.Unlock()

	calls := make([]fantasy.Call, len(m.calls))
	for i, call := range m.calls {
		calls[i] = call
		calls[i].Prompt = cloneFantasyMessages(call.Prompt)
	}
	return calls
}

type longReplayProbeModel struct {
	mu        sync.Mutex
	calls     []fantasy.Call
	toolSteps int
}

func (m *longReplayProbeModel) Provider() string { return "fake" }
func (m *longReplayProbeModel) Model() string    { return "long-replay-probe" }

func (m *longReplayProbeModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *longReplayProbeModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	callNumber := len(m.calls)
	call.Prompt = cloneFantasyMessages([]fantasy.Message(call.Prompt))
	m.calls = append(m.calls, call)
	m.mu.Unlock()

	return func(yield func(fantasy.StreamPart) bool) {
		if callNumber < m.toolSteps {
			padding := strings.Repeat(fmt.Sprintf("step-%02d payload ", callNumber), 160)
			yield(fantasy.StreamPart{
				Type:  fantasy.StreamPartTypeReasoningStart,
				ID:    fmt.Sprintf("reasoning-%02d", callNumber),
				Delta: fmt.Sprintf("Reasoning step %02d.\n%s\n", callNumber, padding),
			})
			yield(fantasy.StreamPart{
				Type: fantasy.StreamPartTypeReasoningEnd,
				ID:   fmt.Sprintf("reasoning-%02d", callNumber),
			})
			if callNumber%3 != 2 {
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: fmt.Sprintf("text-%02d", callNumber)})
				yield(fantasy.StreamPart{
					Type:  fantasy.StreamPartTypeTextDelta,
					ID:    fmt.Sprintf("text-%02d", callNumber),
					Delta: fmt.Sprintf("\nProgress step %02d.\n%s\n\n", callNumber, padding),
				})
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: fmt.Sprintf("text-%02d", callNumber)})
			}
			for toolIndex := range 2 {
				yield(fantasy.StreamPart{
					Type:          fantasy.StreamPartTypeToolCall,
					ID:            fmt.Sprintf("call-%02d-%d", callNumber, toolIndex),
					ToolCallName:  "echo",
					ToolCallInput: fmt.Sprintf(`{"message":"tool step %02d result %d"}`, callNumber, toolIndex),
				})
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
			return
		}

		if callNumber == m.toolSteps {
			yield(fantasy.StreamPart{
				Type:  fantasy.StreamPartTypeReasoningStart,
				ID:    "terminal-reasoning",
				Delta: "The long tool loop is complete.\n",
			})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeReasoningEnd, ID: "terminal-reasoning"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "terminal-text"})
			yield(fantasy.StreamPart{
				Type:  fantasy.StreamPartTypeTextDelta,
				ID:    "terminal-text",
				Delta: "\nStopped with a progress report.\n\n",
			})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "terminal-text"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
			return
		}

		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "continued-text"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "continued-text", Delta: "Continuing."})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "continued-text"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (m *longReplayProbeModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *longReplayProbeModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *longReplayProbeModel) recordedCalls() []fantasy.Call {
	m.mu.Lock()
	defer m.mu.Unlock()

	calls := make([]fantasy.Call, len(m.calls))
	for i, call := range m.calls {
		calls[i] = call
		calls[i].Prompt = cloneFantasyMessages(call.Prompt)
	}
	return calls
}

func openAICompatMessages(t *testing.T, prompt fantasy.Prompt) []json.RawMessage {
	t.Helper()

	messages, warnings := openaicompat.ToPromptFunc(prompt, "", "")
	require.Empty(t, warnings)
	result := make([]json.RawMessage, len(messages))
	for i, msg := range messages {
		data, err := json.Marshal(msg)
		require.NoError(t, err)
		result[i] = data
	}
	return result
}

func withoutProviderOptions(messages []fantasy.Message) []fantasy.Message {
	messages = cloneFantasyMessages(messages)
	for i := range messages {
		messages[i].ProviderOptions = nil
		for j, part := range messages[i].Content {
			switch part := part.(type) {
			case fantasy.TextPart:
				part.ProviderOptions = nil
				messages[i].Content[j] = part
			case fantasy.ReasoningPart:
				part.ProviderOptions = nil
				messages[i].Content[j] = part
			case fantasy.ToolCallPart:
				part.ProviderOptions = nil
				messages[i].Content[j] = part
			case fantasy.ToolResultPart:
				part.ProviderOptions = nil
				messages[i].Content[j] = part
			}
		}
	}
	return messages
}

func TestRun_ReplaysPersistedHistoryWithoutChangingPromptPrefix(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	model := &replayProbeModel{
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
			RunID:     "first-run",
			Prompt:    "  first turn\n\n",
		})
		firstDone <- runErr
	}()

	select {
	case <-model.secondEntered:
	case <-t.Context().Done():
		t.Fatal("second autonomous step did not start")
	}

	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "second-run",
		Prompt:    "second turn",
	})
	require.NoError(t, err)
	close(model.releaseSecond)
	require.NoError(t, <-firstDone)

	calls := model.recordedCalls()
	require.Len(t, calls, 3)

	replayedPrompt := withoutProviderOptions(calls[2].Prompt)

	inMemoryWirePrefix := openAICompatMessages(t, calls[1].Prompt)
	replayedWirePrompt := openAICompatMessages(t, calls[2].Prompt)
	require.Greater(t, len(replayedWirePrompt), len(inMemoryWirePrefix))
	require.Equal(t, inMemoryWirePrefix, replayedWirePrompt[:len(inMemoryWirePrefix)])
	require.Equal(t, calls[1].Tools, calls[2].Tools)

	var foundFinalAnswer bool
	for _, msg := range replayedPrompt {
		if msg.Role != fantasy.MessageRoleAssistant {
			continue
		}
		for _, part := range msg.Content {
			text, ok := part.(fantasy.TextPart)
			if ok && text.Text == "\nFinal answer.\n\n" {
				foundFinalAnswer = true
			}
		}
	}
	require.True(t, foundFinalAnswer)
}

func TestRun_ReplaysLongOpenAIHistoryWithoutChangingWirePrefix(t *testing.T) {
	t.Parallel()

	const toolSteps = 12

	env := testEnv(t)
	model := &longReplayProbeModel{toolSteps: toolSteps}
	small := &finishStreamModel{text: "title"}
	echo := fantasy.NewAgentTool(
		"echo",
		"Echo the input.",
		func(_ context.Context, input struct {
			Message string `json:"message"`
		}, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			response := fantasy.NewTextResponse("Echo: " + input.Message)
			return fantasy.WithResponseMetadata(response, map[string]string{"source": input.Message}), nil
		},
	)
	agent := testSessionAgent(env, model, small, "system", echo)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "long-run",
		Prompt:    "work through the long task",
	})
	require.NoError(t, err)

	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "continue-run",
		Prompt:    "please continue",
	})
	require.NoError(t, err)

	calls := model.recordedCalls()
	require.Len(t, calls, toolSteps+2)

	inMemoryPrompt := withoutProviderOptions(calls[toolSteps].Prompt)
	replayedPrompt := withoutProviderOptions(calls[toolSteps+1].Prompt)
	require.Greater(t, len(replayedPrompt), len(inMemoryPrompt))

	inMemoryWirePrefix := openAICompatMessages(t, calls[toolSteps].Prompt)
	replayedWirePrompt := openAICompatMessages(t, calls[toolSteps+1].Prompt)
	require.Greater(t, len(replayedWirePrompt), len(inMemoryWirePrefix))
	require.Greater(t, len(inMemoryWirePrefix), toolSteps*3)
	require.Equal(t, inMemoryWirePrefix, replayedWirePrompt[:len(inMemoryWirePrefix)])
	require.Equal(t, calls[toolSteps].Tools, calls[toolSteps+1].Tools)

	var wireBytes int
	for _, msg := range inMemoryWirePrefix {
		wireBytes += len(msg)
	}
	require.Greater(t, wireBytes, 50_000)
}
