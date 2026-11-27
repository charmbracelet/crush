package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/schema"
)

const (
	cacheWarmInterval        = 4 * time.Minute
	cacheWarmMaxOutputTokens = int64(1)

	// cacheWarmMaxPerRun caps how many warm requests a single tool run may
	// send, so a hung tool cannot keep paying for warms indefinitely.
	cacheWarmMaxPerRun = 3

	// cacheWarmThinkingBreakEven is the minimum ratio of estimated cached
	// prefix tokens to warm-call max_tokens before a manual-thinking warm is
	// worth sending. A warm costs up to T output tokens (~5x input price)
	// plus a 0.1x cache read; letting the cache lapse costs a 1.25x rewrite
	// of the P-token prefix, i.e. ~1.15P extra. Break-even is roughly
	// P > 5T/1.05, rounded up to 5x.
	cacheWarmThinkingBreakEven = 5

	// cacheWarmCharsPerToken is the heuristic used to estimate prefix size
	// from the serialized prompt. We don't have a pre-request token count
	// for the step being warmed, so a chars/4 estimate over the exact
	// messages and tools being sent is used instead.
	cacheWarmCharsPerToken = 4
)

type cacheWarmer struct {
	model           fantasy.LanguageModel
	messages        []fantasy.Message
	tools           []fantasy.Tool
	providerOptions fantasy.ProviderOptions
	userAgent       string
	maxOutputTokens int64
	warmEvery       time.Duration
	maxPerRun       int

	mu       sync.Mutex
	lastWarm time.Time
}

func newCacheWarmer(model Model, messages []fantasy.Message, tools []fantasy.AgentTool, providerOptions fantasy.ProviderOptions) *cacheWarmer {
	if cacheWarmingDisabled() || !cacheWarmingSupportedProvider(model.ModelCfg.Provider) || len(messages) == 0 {
		return nil
	}
	warmTools := cacheWarmTools(tools)

	// The thinking config is part of the cached prefix, so the warm call
	// must send the exact same provider options as the real request. In
	// manual thinking mode max_tokens must exceed budget_tokens, so the warm
	// call may produce up to budget+1 output tokens; only warm when the
	// prefix is large enough for that to pay off. Adaptive/effort-only
	// setups have no budget rule and keep the 1-token warm.
	maxOutputTokens := cacheWarmMaxOutputTokens
	if budget := thinkingBudget(providerOptions); budget > 0 {
		maxOutputTokens = budget + 1
		if estimatePromptTokens(messages, warmTools) < cacheWarmThinkingBreakEven*maxOutputTokens {
			return nil
		}
	}
	return &cacheWarmer{
		model:           model.Model,
		messages:        cloneFantasyMessages(messages),
		tools:           warmTools,
		providerOptions: providerOptions,
		userAgent:       userAgent,
		maxOutputTokens: maxOutputTokens,
		warmEvery:       cacheWarmInterval,
		maxPerRun:       cacheWarmMaxPerRun,
	}
}

// estimatePromptTokens approximates the cached prefix size of a request
// using a chars/4 heuristic over the serialized messages and tools.
func estimatePromptTokens(messages []fantasy.Message, tools []fantasy.Tool) int64 {
	var chars int
	if data, err := json.Marshal(messages); err == nil {
		chars += len(data)
	}
	if data, err := json.Marshal(tools); err == nil {
		chars += len(data)
	}
	return int64(chars / cacheWarmCharsPerToken)
}

func cacheWarmingDisabled() bool {
	disabled, _ := strconv.ParseBool(os.Getenv("CRUSH_DISABLE_CACHE_WARMING"))
	return disabled
}

func cacheWarmingSupportedProvider(provider string) bool {
	switch provider {
	case string(catwalk.InferenceProviderAnthropic),
		string(catwalk.InferenceProviderBedrock),
		string(catwalk.InferenceProviderBedrockEurope),
		string(catwalk.InferenceProviderVercel):
		return true
	default:
		return false
	}
}

// thinkingBudget returns the Anthropic manual extended-thinking budget set
// on the call, or 0 when thinking is off or adaptive/effort-based.
func thinkingBudget(opts fantasy.ProviderOptions) int64 {
	o, ok := opts[anthropic.Name].(*anthropic.ProviderOptions)
	if !ok || o == nil || o.Thinking == nil {
		return 0
	}
	return o.Thinking.BudgetTokens
}

// cacheWarmTools mirrors how fantasy converts AgentTools into request
// tools. The provider cache prefix is hashed over tools, then system, then
// messages, so the warm request must carry an identical tool list or it
// writes a separate cache entry instead of refreshing the live one.
func cacheWarmTools(tools []fantasy.AgentTool) []fantasy.Tool {
	if len(tools) == 0 {
		return nil
	}
	prepared := make([]fantasy.Tool, 0, len(tools))
	for _, tool := range tools {
		info := tool.Info()
		inputSchema := map[string]any{
			"type":       "object",
			"properties": info.Parameters,
			"required":   info.Required,
		}
		schema.Normalize(inputSchema)
		prepared = append(prepared, fantasy.FunctionTool{
			Name:            info.Name,
			Description:     info.Description,
			InputSchema:     inputSchema,
			ProviderOptions: tool.ProviderOptions(),
		})
	}
	return prepared
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
		for warms := 0; w.maxPerRun <= 0 || warms < w.maxPerRun; {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				if w.warm(ctx) {
					warms++
				}
				timer.Reset(w.warmEvery)
			}
		}
	}()
	return cancel
}

// warm sends one warm request and reports whether it was sent. The
// prompt is sent unchanged (no trailing "Reply OK." turn) so the cached
// prefix and its breakpoints stay byte-identical to the real request.
func (w *cacheWarmer) warm(ctx context.Context) bool {
	if !w.reserveWarm(time.Now()) {
		return false
	}
	maxOutputTokens := w.maxOutputTokens
	if maxOutputTokens <= 0 {
		maxOutputTokens = cacheWarmMaxOutputTokens
	}
	// Temperature is intentionally omitted: a fixed value is rejected by
	// Bedrock/Anthropic when thinking or adaptive reasoning is enabled.
	_, err := w.model.Generate(ctx, fantasy.Call{
		Prompt:          append(fantasy.Prompt{}, w.messages...),
		Tools:           w.tools,
		MaxOutputTokens: &maxOutputTokens,
		UserAgent:       w.userAgent,
		ProviderOptions: w.providerOptions,
	})
	if err != nil {
		slog.Debug("Failed to warm provider cache", "error", err)
	}
	return true
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
