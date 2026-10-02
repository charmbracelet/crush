package agent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCacheWarmingToolWarmsLongRunningTool(t *testing.T) {
	model := &fakeCacheWarmModel{}
	warmer := &cacheWarmer{
		model:     model,
		messages:  []fantasy.Message{fantasy.NewUserMessage("hello")},
		warmEvery: 10 * time.Millisecond,
	}
	tool := &cacheWarmingTool{
		AgentTool: &sleepTool{delay: 35 * time.Millisecond},
		warmer:    warmer,
	}

	_, err := tool.Run(t.Context(), fantasy.ToolCall{Name: "sleep"})

	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return model.calls.Load() > 0
	}, time.Second, time.Millisecond)
}

func TestCacheWarmingToolSkipsShortRunningTool(t *testing.T) {
	model := &fakeCacheWarmModel{}
	warmer := &cacheWarmer{
		model:     model,
		messages:  []fantasy.Message{fantasy.NewUserMessage("hello")},
		warmEvery: 50 * time.Millisecond,
	}
	tool := &cacheWarmingTool{
		AgentTool: &sleepTool{delay: time.Millisecond},
		warmer:    warmer,
	}

	_, err := tool.Run(t.Context(), fantasy.ToolCall{Name: "sleep"})

	require.NoError(t, err)
	time.Sleep(75 * time.Millisecond)
	require.Zero(t, model.calls.Load())
}

func TestNewCacheWarmerGuardsUnsupportedProviders(t *testing.T) {
	model := Model{
		Model: &fakeCacheWarmModel{},
		ModelCfg: config.SelectedModel{
			Provider: "openai",
		},
		CatwalkCfg: catwalk.Model{},
	}
	messages := []fantasy.Message{fantasy.NewUserMessage("hello")}

	require.Nil(t, newCacheWarmer(model, messages, nil))

	model.ModelCfg.Provider = "anthropic"
	require.NotNil(t, newCacheWarmer(model, messages, nil))
}

type fakeCacheWarmModel struct {
	calls atomic.Int64
}

func (m *fakeCacheWarmModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	m.calls.Add(1)
	return &fantasy.Response{}, nil
}

func (m *fakeCacheWarmModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	return func(func(fantasy.StreamPart) bool) {}, nil
}

func (m *fakeCacheWarmModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return &fantasy.ObjectResponse{}, nil
}

func (m *fakeCacheWarmModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return func(func(fantasy.ObjectStreamPart) bool) {}, nil
}

func (m *fakeCacheWarmModel) Provider() string {
	return "anthropic"
}

func (m *fakeCacheWarmModel) Model() string {
	return "test-model"
}

type sleepTool struct {
	delay time.Duration
}

func (t *sleepTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{Name: "sleep"}
}

func (t *sleepTool) Run(ctx context.Context, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
	select {
	case <-ctx.Done():
		return fantasy.ToolResponse{}, ctx.Err()
	case <-time.After(t.delay):
		return fantasy.NewTextResponse("ok"), nil
	}
}

func (t *sleepTool) ProviderOptions() fantasy.ProviderOptions {
	return nil
}

func (t *sleepTool) SetProviderOptions(fantasy.ProviderOptions) {}
