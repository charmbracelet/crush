package agent

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"charm.land/fantasy"
)

const (
	cacheWarmInterval        = 4 * time.Minute
	cacheWarmMaxOutputTokens = int64(1)
	cacheWarmTemperature     = 0.0
)

type cacheWarmer struct {
	model           fantasy.LanguageModel
	messages        []fantasy.Message
	providerOptions fantasy.ProviderOptions
	userAgent       string
	warmEvery       time.Duration

	mu       sync.Mutex
	lastWarm time.Time
}

func newCacheWarmer(model Model, messages []fantasy.Message, providerOptions fantasy.ProviderOptions) *cacheWarmer {
	if cacheWarmingDisabled() || !cacheWarmingSupportedProvider(model.ModelCfg.Provider) || len(messages) == 0 {
		return nil
	}
	return &cacheWarmer{
		model:           model.Model,
		messages:        cloneFantasyMessages(messages),
		providerOptions: providerOptions,
		userAgent:       userAgent,
		warmEvery:       cacheWarmInterval,
	}
}

func cacheWarmingDisabled() bool {
	disabled, _ := strconv.ParseBool(os.Getenv("CRUSH_DISABLE_CACHE_WARMING"))
	return disabled
}

func cacheWarmingSupportedProvider(provider string) bool {
	switch provider {
	case "anthropic", "bedrock", "vercel":
		return true
	default:
		return false
	}
}

func wrapToolsWithCacheWarmer(tools []fantasy.AgentTool, warmer *cacheWarmer) []fantasy.AgentTool {
	if warmer == nil || len(tools) == 0 {
		return tools
	}
	wrapped := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		wrapped[i] = &cacheWarmingTool{
			AgentTool: tool,
			warmer:    warmer,
		}
	}
	return wrapped
}

type cacheWarmingTool struct {
	fantasy.AgentTool
	warmer *cacheWarmer
}

func (t *cacheWarmingTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	stop := t.warmer.start(ctx)
	defer stop()
	return t.AgentTool.Run(ctx, call)
}

func (w *cacheWarmer) start(ctx context.Context) func() {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		timer := time.NewTimer(w.warmEvery)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				w.warm(ctx)
				timer.Reset(w.warmEvery)
			}
		}
	}()
	return cancel
}

func (w *cacheWarmer) warm(ctx context.Context) {
	if !w.reserveWarm(time.Now()) {
		return
	}
	maxOutputTokens := cacheWarmMaxOutputTokens
	temperature := cacheWarmTemperature
	_, err := w.model.Generate(ctx, fantasy.Call{
		Prompt:          append(fantasy.Prompt{}, w.messages...),
		MaxOutputTokens: &maxOutputTokens,
		Temperature:     &temperature,
		UserAgent:       w.userAgent,
		ProviderOptions: w.providerOptions,
	})
	if err != nil {
		slog.Debug("Failed to warm provider cache", "error", err)
	}
}

func (w *cacheWarmer) reserveWarm(now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.lastWarm.IsZero() && now.Sub(w.lastWarm) < w.warmEvery {
		return false
	}
	w.lastWarm = now
	return true
}
