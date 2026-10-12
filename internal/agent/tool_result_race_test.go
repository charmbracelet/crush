package agent

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

type raceToolParams struct {
	Value string `json:"value"`
}

type parallelToolStreamModel struct {
	toolNames []string
	steps     atomic.Int32
}

func (m *parallelToolStreamModel) Provider() string { return "fake" }
func (m *parallelToolStreamModel) Model() string    { return "fake-model" }

func (m *parallelToolStreamModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: "done"}},
		FinishReason: fantasy.FinishReasonStop,
	}, nil
}

func (m *parallelToolStreamModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	step := m.steps.Add(1)

	return func(yield func(fantasy.StreamPart) bool) {
		if step > 1 {
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "t"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "t", Delta: "done"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "t"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
			return
		}

		for i, name := range m.toolNames {
			yield(fantasy.StreamPart{
				Type:          fantasy.StreamPartTypeToolCall,
				ID:            fmt.Sprintf("call-%d", i),
				ToolCallName:  name,
				ToolCallInput: `{"value":"x"}`,
			})
		}

		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
	}, nil
}

func (m *parallelToolStreamModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *parallelToolStreamModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func TestRun_ConcurrentToolResultsDoNotRace(t *testing.T) {
	t.Parallel()

	env := testEnv(t)

	const toolCount = 8

	names := make([]string, 0, toolCount)
	executions := make([]atomic.Int32, toolCount)
	toolsList := make([]fantasy.AgentTool, 0, toolCount)

	for i := range toolCount {
		name := fmt.Sprintf("race_tool_%d", i)
		names = append(names, name)
		toolsList = append(toolsList, fantasy.NewParallelAgentTool(
			name,
			"test tool",
			func(context.Context, raceToolParams, fantasy.ToolCall) (fantasy.ToolResponse, error) {
				executions[i].Add(1)
				return fantasy.NewTextResponse("ok"), nil
			},
		))
	}

	model := &parallelToolStreamModel{toolNames: names}
	sa := testSessionAgent(env, model, fastModel{}, "system", toolsList...).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "go"})
	require.NoError(t, err)

	for i := range toolCount {
		require.Equal(t, int32(1), executions[i].Load(), "tool %d must execute exactly once", i)
	}
}
