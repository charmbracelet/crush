package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

type contextWindowTestModel struct {
	generateCalls atomic.Int64
	streamCalls   atomic.Int64
	streamErr     error
}

type contextWindowTestTool struct {
	info fantasy.ToolInfo
}

func (t *contextWindowTestTool) Info() fantasy.ToolInfo { return t.info }
func (t *contextWindowTestTool) Run(context.Context, fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return fantasy.ToolResponse{}, nil
}
func (t *contextWindowTestTool) ProviderOptions() fantasy.ProviderOptions   { return nil }
func (t *contextWindowTestTool) SetProviderOptions(fantasy.ProviderOptions) {}

func (m *contextWindowTestModel) Provider() string { return "test" }
func (m *contextWindowTestModel) Model() string    { return "test-model" }

func (m *contextWindowTestModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	m.generateCalls.Add(1)
	return &fantasy.Response{}, nil
}

func (m *contextWindowTestModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	m.streamCalls.Add(1)
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	return func(func(fantasy.StreamPart) bool) {}, nil
}

func (m *contextWindowTestModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *contextWindowTestModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func TestContextWindowModelRejectsOversizedCallBeforeDispatch(t *testing.T) {
	t.Parallel()

	provider := &contextWindowTestModel{}
	model := withContextWindowLimit(provider, 204_800)
	// Reproduce the shape of the captured LM Studio incident: one system,
	// 33 user, 571 assistant, and 775 tool messages (1,380 total). A single
	// giant string would miss regressions that omit accumulated tool results.
	prompt := make(fantasy.Prompt, 0, 1_380)
	prompt = append(prompt, fantasy.NewSystemMessage("system prompt"))
	for range 33 {
		prompt = append(prompt, fantasy.NewUserMessage("continue the translation"))
	}
	for range 571 {
		prompt = append(prompt, fantasy.Message{
			Role:    fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{fantasy.TextPart{Text: "working"}},
		})
	}
	toolPayload := strings.Repeat("large tool output ", 400)
	for i := range 775 {
		prompt = append(prompt, fantasy.Message{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: fmt.Sprintf("call-%d", i),
					Output:     fantasy.ToolResultOutputContentText{Text: toolPayload},
				},
			},
		})
	}
	require.Len(t, prompt, 1_380)
	call := fantasy.Call{Prompt: prompt}

	_, err := model.Stream(t.Context(), call)
	require.ErrorContains(t, err, "model request exceeds context window")
	require.Zero(t, provider.streamCalls.Load())
}

func TestContextWindowModelRejectsOversizedGenerateBeforeDispatch(t *testing.T) {
	t.Parallel()

	provider := &contextWindowTestModel{}
	model := withContextWindowLimit(provider, 1_000)
	_, err := model.Generate(t.Context(), fantasy.Call{
		Prompt: fantasy.Prompt{fantasy.NewUserMessage(strings.Repeat("x", 3_000))},
	})

	var exceeded *contextWindowExceededError
	require.ErrorAs(t, err, &exceeded)
	require.Zero(t, provider.generateCalls.Load())
	require.Equal(t, int64(1_000), exceeded.contextWindow)
}

func TestContextWindowModelDispatchesGenerateWithinLimit(t *testing.T) {
	t.Parallel()

	provider := &contextWindowTestModel{}
	model := withContextWindowLimit(provider, 4_096)
	_, err := model.Generate(t.Context(), fantasy.Call{
		Prompt: fantasy.Prompt{fantasy.NewUserMessage("small request")},
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), provider.generateCalls.Load())
}

func TestContextWindowModelBoundaryIsInclusive(t *testing.T) {
	t.Parallel()

	provider := &contextWindowTestModel{}
	model := withContextWindowLimit(provider, 1_000)
	// Message framing plus "user" contributes six estimated tokens and 397
	// short word/space pairs contribute 794, leaving exactly the 200-token
	// output reserve.
	exact := fantasy.Call{Prompt: fantasy.Prompt{
		fantasy.NewUserMessage(strings.Repeat("xxx ", 397)),
	}}
	_, err := model.Stream(t.Context(), exact)
	require.NoError(t, err)
	require.Equal(t, int64(1), provider.streamCalls.Load())

	over := fantasy.Call{Prompt: fantasy.Prompt{
		fantasy.NewUserMessage(strings.Repeat("xxx ", 397) + "!"),
	}}
	_, err = model.Stream(t.Context(), over)
	require.ErrorAs(t, err, new(*contextWindowExceededError))
	require.Equal(t, int64(1), provider.streamCalls.Load(), "one-token-over call must not reach the provider")
}

func TestContextWindowModelDispatchesCallWithinLimit(t *testing.T) {
	t.Parallel()

	provider := &contextWindowTestModel{}
	model := withContextWindowLimit(provider, 204_800)
	maxOutputTokens := int64(8_192)
	call := fantasy.Call{
		Prompt: fantasy.Prompt{
			fantasy.NewUserMessage(strings.Repeat("word ", 30_000)),
		},
		MaxOutputTokens: &maxOutputTokens,
	}

	_, err := model.Stream(t.Context(), call)
	require.NoError(t, err)
	require.Equal(t, int64(1), provider.streamCalls.Load())
}

func TestContextWindowModelIncludesToolsInBudget(t *testing.T) {
	t.Parallel()

	provider := &contextWindowTestModel{}
	model := withContextWindowLimit(provider, 204_800)
	call := fantasy.Call{
		Prompt: fantasy.Prompt{
			fantasy.NewUserMessage(strings.Repeat("x", 180_000*4)),
		},
		Tools: []fantasy.Tool{
			fantasy.FunctionTool{
				Name:        "large-tool",
				Description: strings.Repeat("x", 20_000*4),
			},
		},
	}

	_, err := model.Stream(t.Context(), call)
	require.ErrorContains(t, err, "model request exceeds context window")
	require.Zero(t, provider.streamCalls.Load())
}

func TestContextWindowModelReservesConfiguredOutputTokens(t *testing.T) {
	t.Parallel()

	provider := &contextWindowTestModel{}
	model := withContextWindowLimit(provider, 1_000)
	maxOutputTokens := int64(300)
	call := fantasy.Call{
		Prompt: fantasy.Prompt{
			fantasy.NewUserMessage(strings.Repeat("x", 750*4)),
		},
		MaxOutputTokens: &maxOutputTokens,
	}

	_, err := model.Stream(t.Context(), call)
	require.ErrorContains(t, err, "plus 300 reserved output tokens")
	require.Zero(t, provider.streamCalls.Load())
}

func TestContextWindowModelSkipsUnknownLimit(t *testing.T) {
	t.Parallel()

	provider := &contextWindowTestModel{}
	model := withContextWindowLimit(provider, 0)

	require.Same(t, provider, model)
}

func TestResolveMaxOutputTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		contextWindow int64
		modelDefault  int64
		requested     int64
		expected      int64
		expectedIsNil bool
	}{
		{name: "request wins", contextWindow: 204_800, modelDefault: 8_192, requested: 4_096, expected: 4_096},
		{name: "model default", contextWindow: 204_800, modelDefault: 8_192, expected: 8_192},
		{name: "known context reserve", contextWindow: 204_800, expected: 20_000},
		{name: "small context reserve", contextWindow: 4_096, expected: 819},
		{name: "unknown remains unbounded", expectedIsNil: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveMaxOutputTokens(tt.contextWindow, tt.modelDefault, tt.requested)
			if tt.expectedIsNil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, tt.expected, *got)
		})
	}
}

func TestEstimateNextStepIncludesToolResultsAndDefinitions(t *testing.T) {
	t.Parallel()

	currentMessages := []fantasy.Message{
		fantasy.NewUserMessage(strings.Repeat("x", 100*4)),
	}
	step := fantasy.StepResult{
		Messages: []fantasy.Message{
			{
				Role: fantasy.MessageRoleTool,
				Content: []fantasy.MessagePart{
					fantasy.ToolResultPart{
						ToolCallID: "call-1",
						Output: fantasy.ToolResultOutputContentText{
							Text: strings.Repeat("x", 200*4),
						},
					},
				},
			},
		},
	}
	tools := []fantasy.AgentTool{
		&contextWindowTestTool{info: fantasy.ToolInfo{
			Name:        "large-tool",
			Description: strings.Repeat("x", 300*4),
		}},
	}

	estimated := estimateNextStepInputTokens(currentMessages, step, tools)
	require.GreaterOrEqual(t, estimated, int64(600))
}
