package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// scriptModel answers the session agent's two kinds of model calls: a title
// request (no tools) and the turn itself. The first turn streams several
// parallel tool calls; later turns stop.
type scriptModel struct {
	turn atomic.Int32
}

func (m *scriptModel) Provider() string { return "test" }
func (m *scriptModel) Model() string    { return "test" }

func (m *scriptModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *scriptModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *scriptModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *scriptModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	if len(call.Tools) == 0 {
		return textStream("Title"), nil
	}
	if m.turn.Add(1) == 1 {
		return parallelToolCallStream(parallelToolCount), nil
	}
	return textStream("done"), nil
}

const parallelToolCount = 5

func textStream(text string) fantasy.StreamResponse {
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text-1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text-1", Delta: text}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "text-1"}) {
			return
		}
		yield(fantasy.StreamPart{
			Type:         fantasy.StreamPartTypeFinish,
			Usage:        fantasy.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
			FinishReason: fantasy.FinishReasonStop,
		})
	}
}

func parallelToolCallStream(n int) fantasy.StreamResponse {
	return func(yield func(fantasy.StreamPart) bool) {
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("call-%d", i)
			name := fmt.Sprintf("poke_%d", i)
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputStart, ID: id, ToolCallName: name}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputDelta, ID: id, Delta: `{}`}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputEnd, ID: id}) {
				return
			}
			if !yield(fantasy.StreamPart{
				Type:          fantasy.StreamPartTypeToolCall,
				ID:            id,
				ToolCallName:  name,
				ToolCallInput: `{}`,
			}) {
				return
			}
		}
		yield(fantasy.StreamPart{
			Type:         fantasy.StreamPartTypeFinish,
			Usage:        fantasy.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
			FinishReason: fantasy.FinishReasonToolCalls,
		})
	}
}

// TestParallelToolResultsDoNotRace completes several parallel tools at the
// same moment. fantasy runs those tools on separate goroutines and calls
// OnToolResult from each one. Recording the completed names must not race.
func TestParallelToolResultsDoNotRace(t *testing.T) {
	env := testEnv(t)
	var ready sync.WaitGroup
	ready.Add(parallelToolCount)
	tools := make([]fantasy.AgentTool, 0, parallelToolCount)
	for i := 0; i < parallelToolCount; i++ {
		name := fmt.Sprintf("poke_%d", i)
		tools = append(tools, fantasy.NewParallelAgentTool(
			name,
			"test tool",
			func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
				ready.Done()
				ready.Wait()
				return fantasy.NewTextResponse("ok"), nil
			},
		))
	}

	agent := testSessionAgent(env, &scriptModel{}, &scriptModel{}, "test", tools...)
	sess, err := env.sessions.Create(t.Context(), "parallel tools")
	require.NoError(t, err)

	_, err = agent.Run(t.Context(), SessionAgentCall{
		Prompt:          "run the tools",
		SessionID:       sess.ID,
		MaxOutputTokens: 1000,
	})
	require.NoError(t, err)
}
