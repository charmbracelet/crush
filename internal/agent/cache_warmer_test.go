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

	require.Nil(t, newCacheWarmer(model, messages, nil, nil))

	model.ModelCfg.Provider = "anthropic"
	require.NotNil(t, newCacheWarmer(model, messages, nil, nil))
}

func TestCacheWarmerSendsSameToolsAsStep(t *testing.T) {
	model := &fakeCacheWarmModel{}
	cacheOpts := fantasy.ProviderOptions{"anthropic": nil}
	tool := &sleepTool{delay: time.Millisecond}
	tool.SetProviderOptions(cacheOpts)
	warmer := &cacheWarmer{
		model:     model,
		messages:  []fantasy.Message{fantasy.NewUserMessage("hello")},
		tools:     cacheWarmTools([]fantasy.AgentTool{tool}),
		warmEvery: time.Millisecond,
	}

	warmer.warm(t.Context())

	call := model.lastCall.Load()
	require.NotNil(t, call)
	require.Len(t, call.Tools, 1)
	fn, ok := call.Tools[0].(fantasy.FunctionTool)
	require.True(t, ok)
	require.Equal(t, "sleep", fn.Name)
	require.Equal(t, cacheOpts, fn.ProviderOptions)
}

type fakeCacheWarmModel struct {
	calls    atomic.Int64
	lastCall atomic.Pointer[fantasy.Call]
}

func (m *fakeCacheWarmModel) Generate(_ context.Context, call fantasy.Call) (*fantasy.Response, error) {
	m.calls.Add(1)
	m.lastCall.Store(&call)
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
	opts  fantasy.ProviderOptions
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
	return t.opts
}

func (t *sleepTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.opts = opts
}
