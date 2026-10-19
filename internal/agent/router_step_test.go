package agent

import (
	"context"
	"errors"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// toolStepModel streams one tool call per request and records the max
// output tokens it was called with.
type toolStepModel struct {
	finishStreamModel
	mu        sync.Mutex
	maxTokens []int64
}

func (m *toolStepModel) record(call fantasy.Call) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var v int64
	if call.MaxOutputTokens != nil {
		v = *call.MaxOutputTokens
	}
	m.maxTokens = append(m.maxTokens, v)
}

func (m *toolStepModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.record(call)
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputStart, ID: "t1", ToolCallName: "ping"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: "t1", ToolCallName: "ping", ToolCallInput: `{"host":"a"}`}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
	}, nil
}

// textStepModel finishes the turn with text and records max tokens.
type textStepModel struct {
	toolStepModel
}

func (m *textStepModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.record(call)
	return (&finishStreamModel{text: "implemented"}).Stream(ctx, call)
}

func TestRunRoutesEachStep(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	ping := fantasy.NewAgentTool("ping", "ping a host", func(context.Context, struct {
		Host string `json:"host"`
	}, fantasy.ToolCall,
	) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("pong"), nil
	})
	base := &finishStreamModel{text: "large"}
	sa := testSessionAgent(env, base, base, "system", ping).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "s")
	require.NoError(t, err)

	explorer := &toolStepModel{}
	implementer := &textStepModel{}
	var inputs []RouteInput
	router := &fakeRouter{route: func(_ context.Context, in RouteInput) (RouteResult, error) {
		inputs = append(inputs, in)
		if in.Activity == "" {
			return RouteResult{
				Router: "auto", Route: "explore",
				Model:   Model{Model: explorer, ModelCfg: config.SelectedModel{Provider: "p", Model: "haiku"}},
				Options: TurnOptions{MaxOutputTokens: 111},
			}, nil
		}
		return RouteResult{
			Router: "auto", Route: "implement",
			Model:   Model{Model: implementer, ModelCfg: config.SelectedModel{Provider: "p", Model: "opus"}},
			Options: TurnOptions{MaxOutputTokens: 222},
		}, nil
	}}

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "fix it", MaxOutputTokens: 999, Router: router})
	require.NoError(t, err)

	require.Len(t, inputs, 2, "routed once per step")
	require.Contains(t, inputs[1].Activity, `called ping {"host":"a"}`)
	require.Contains(t, inputs[1].Activity, "ping returned: pong")
	require.Equal(t, "fix it", inputs[1].Prompt)

	// Each step used its own model's call options, not the stream's.
	require.Equal(t, []int64{111}, explorer.maxTokens)
	require.Equal(t, []int64{222}, implementer.maxTokens)

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var got []string
	for _, m := range msgs {
		if m.Role == message.Assistant {
			got = append(got, m.Model+"/"+m.RouteName)
		}
	}
	require.Equal(t, []string{"haiku/explore", "opus/implement"}, got)
}

func TestRunKeepsModelWhenStepRoutingFails(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	ping := fantasy.NewAgentTool("ping", "ping", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("pong"), nil
	})
	base := &finishStreamModel{text: "large"}
	sa := testSessionAgent(env, base, base, "system", ping).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "s")
	require.NoError(t, err)

	first := &toolStepModel{}
	calls := 0
	router := &fakeRouter{route: func(context.Context, RouteInput) (RouteResult, error) {
		calls++
		if calls == 1 {
			return RouteResult{Router: "auto", Route: "a", Model: Model{Model: first, ModelCfg: config.SelectedModel{Provider: "p", Model: "a"}}}, nil
		}
		return RouteResult{}, errors.New("classifier down")
	}}
	// The kept model calls the same tool every step; loop detection ends
	// the turn.
	_, _ = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "go", Router: router})

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	models := []string{}
	for _, m := range msgs {
		if m.Role == message.Assistant {
			models = append(models, m.Model)
		}
	}
	require.GreaterOrEqual(t, len(models), 2)
	for _, m := range models {
		require.Equal(t, "a", m, "a failed re-route keeps the previous step's model")
	}
}
