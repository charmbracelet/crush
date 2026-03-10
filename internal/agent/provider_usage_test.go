package agent

// Diagnostic tests for real provider API calls to verify usage field reporting.
// These tests make real HTTP calls and are intentionally not parallelized.
//
// Run with:
//
//	go test ./internal/agent/... -run TestProviderUsage -v

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// anthropicProxyProvider returns a configured Anthropic provider pointing at
// the anthropic-proxy entry in crush.json, skipping the test if unavailable.
func anthropicProxyProvider(t *testing.T) (fantasy.LanguageModel, func()) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping real API test in short mode")
	}
	cfg, err := config.Init(t.TempDir(), "", false)
	require.NoError(t, err)

	providerCfg, ok := cfg.Providers.Get("anthropic-proxy")
	if !ok {
		t.Skip("anthropic-proxy provider not configured")
	}
	if providerCfg.APIKey == "" {
		t.Skip("anthropic-proxy has no API key configured")
	}

	provider, err := anthropic.New(
		anthropic.WithBaseURL(providerCfg.BaseURL),
		anthropic.WithAPIKey(providerCfg.APIKey),
	)
	require.NoError(t, err)

	lm, err := provider.LanguageModel(context.Background(), "qwen3.5-plus")
	require.NoError(t, err)
	return lm, func() {}
}

// TestProviderUsage_AnthropicProxy_SingleTurn verifies what usage fields
// the anthropic-proxy provider actually returns for qwen3.5-plus without a
// system prompt.
func TestProviderUsage_AnthropicProxy_SingleTurn(t *testing.T) {
	lm, _ := anthropicProxyProvider(t)

	maxTokens := int64(50)
	resp, err := lm.Generate(context.Background(), fantasy.Call{
		Prompt: fantasy.Prompt{
			fantasy.NewUserMessage("Reply with exactly: hello"),
		},
		MaxOutputTokens: &maxTokens,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	u := resp.Usage
	t.Logf("=== Usage: qwen3.5-plus (no system prompt), single turn ===")
	t.Logf("  InputTokens:         %d", u.InputTokens)
	t.Logf("  OutputTokens:        %d", u.OutputTokens)
	t.Logf("  CacheCreationTokens: %d", u.CacheCreationTokens)
	t.Logf("  CacheReadTokens:     %d", u.CacheReadTokens)
	t.Logf("  promptTokensForUsage: %d", u.InputTokens+u.CacheCreationTokens+u.CacheReadTokens)

	require.Greater(t, u.OutputTokens, int64(0), "OutputTokens must be > 0")
	require.Greater(t, u.InputTokens, int64(0), "InputTokens should be > 0")
}

// TestProviderUsage_AnthropicProxy_WithSystemPrompt verifies whether the proxy
// includes system-prompt tokens in its reported input_tokens, both with and
// without cache_control (which crush always sets for anthropic providers).
func TestProviderUsage_AnthropicProxy_WithSystemPrompt(t *testing.T) {
	lm, _ := anthropicProxyProvider(t)

	// Use the raw coder system prompt template so the size is realistic (~18 KB).
	systemPromptText := string(coderPromptTmpl)
	maxTokens := int64(50)

	// Case A: no system prompt (baseline).
	respNoSys, err := lm.Generate(context.Background(), fantasy.Call{
		Prompt:          fantasy.Prompt{fantasy.NewUserMessage("Reply: hello")},
		MaxOutputTokens: &maxTokens,
	})
	require.NoError(t, err)
	tokensNoSys := respNoSys.Usage.InputTokens + respNoSys.Usage.CacheCreationTokens + respNoSys.Usage.CacheReadTokens

	// Case B: system prompt WITHOUT cache_control (plain).
	respWithSys, err := lm.Generate(context.Background(), fantasy.Call{
		Prompt: fantasy.Prompt{
			fantasy.NewSystemMessage(systemPromptText),
			fantasy.NewUserMessage("Reply: hello"),
		},
		MaxOutputTokens: &maxTokens,
	})
	require.NoError(t, err)
	tokensWithSys := respWithSys.Usage.InputTokens + respWithSys.Usage.CacheCreationTokens + respWithSys.Usage.CacheReadTokens

	// Case C: system prompt WITH cache_control (exactly what crush does for anthropic providers).
	sysMsg := fantasy.NewSystemMessage(systemPromptText)
	sysMsg.ProviderOptions = fantasy.ProviderOptions{
		"anthropic": &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
	}
	respWithCache, err := lm.Generate(context.Background(), fantasy.Call{
		Prompt: fantasy.Prompt{
			sysMsg,
			fantasy.NewUserMessage("Reply: hello"),
		},
		MaxOutputTokens: &maxTokens,
	})
	require.NoError(t, err)
	tokensWithCache := respWithCache.Usage.InputTokens + respWithCache.Usage.CacheCreationTokens + respWithCache.Usage.CacheReadTokens

	t.Logf("=== System-prompt token inclusion test (system prompt: %d bytes) ===", len(systemPromptText))
	t.Logf("A) No system prompt          — Input:%d Create:%d Read:%d  Total:%d",
		respNoSys.Usage.InputTokens, respNoSys.Usage.CacheCreationTokens, respNoSys.Usage.CacheReadTokens, tokensNoSys)
	t.Logf("B) With sys (no cache_ctrl)  — Input:%d Create:%d Read:%d  Total:%d",
		respWithSys.Usage.InputTokens, respWithSys.Usage.CacheCreationTokens, respWithSys.Usage.CacheReadTokens, tokensWithSys)
	t.Logf("C) With sys + cache_control  — Input:%d Create:%d Read:%d  Total:%d",
		respWithCache.Usage.InputTokens, respWithCache.Usage.CacheCreationTokens, respWithCache.Usage.CacheReadTokens, tokensWithCache)

	t.Logf("--- Diagnostics ---")
	if tokensWithCache < tokensNoSys+500 {
		t.Logf("FINDING (C): proxy does NOT count system prompt when cache_control is set!")
		t.Logf("  This is likely why the UI shows ~95 tokens: crush sets cache_control on the system")
		t.Logf("  block, but the proxy only returns user-message tokens in that case.")
	} else {
		t.Logf("FINDING (C): proxy correctly includes system-prompt tokens with cache_control (+%d tokens).", tokensWithCache-tokensNoSys)
	}
	if tokensWithSys < tokensNoSys+500 {
		t.Logf("FINDING (B): proxy does NOT count system prompt even without cache_control!")
	} else {
		t.Logf("FINDING (B): proxy correctly includes system-prompt tokens without cache_control (+%d tokens).", tokensWithSys-tokensNoSys)
	}
}

// TestProviderUsage_AnthropicProxy_WithThinking verifies how token usage is reported
// when the model is configured with thinking mode (think: true).  The user's config
// has think=true for qwen3.5-plus, and the 95-token display suggests thinking mode
// may affect how the proxy reports system-prompt tokens.
func TestProviderUsage_AnthropicProxy_WithThinking(t *testing.T) {
	lm, _ := anthropicProxyProvider(t)

	systemPromptText := string(coderPromptTmpl)
	maxTokens := int64(200)
	thinkingBudget := int64(1000)

	cacheOptions := fantasy.ProviderOptions{
		anthropic.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
	}
	thinkingOptions := fantasy.ProviderOptions{
		anthropic.Name: &anthropic.ProviderOptions{
			Thinking: &anthropic.ThinkingProviderOption{BudgetTokens: thinkingBudget},
		},
	}

	sysMsg := fantasy.NewSystemMessage(systemPromptText)
	sysMsg.ProviderOptions = cacheOptions

	// With thinking enabled (what crush sends when think=true).
	respThinking, err := lm.Generate(context.Background(), fantasy.Call{
		Prompt: fantasy.Prompt{
			sysMsg,
			fantasy.NewUserMessage("你是？"),
		},
		MaxOutputTokens: &maxTokens,
		ProviderOptions: thinkingOptions,
	})
	require.NoError(t, err)
	u := respThinking.Usage
	totalPrompt := u.InputTokens + u.CacheCreationTokens + u.CacheReadTokens

	// Without thinking, same prompt.
	respNoThink, err := lm.Generate(context.Background(), fantasy.Call{
		Prompt: fantasy.Prompt{
			sysMsg,
			fantasy.NewUserMessage("你是？"),
		},
		MaxOutputTokens: &maxTokens,
	})
	require.NoError(t, err)
	u2 := respNoThink.Usage
	totalPrompt2 := u2.InputTokens + u2.CacheCreationTokens + u2.CacheReadTokens

	t.Logf("=== Thinking mode token test (user msg: '你是？', sys prompt: %d bytes) ===", len(systemPromptText))
	t.Logf("With thinking — Input:%d Create:%d Read:%d Reason:%d Total:%d (crush formula: %d)",
		u.InputTokens, u.CacheCreationTokens, u.CacheReadTokens, u.ReasoningTokens, u.TotalTokens, totalPrompt)
	t.Logf("No thinking  — Input:%d Create:%d Read:%d Reason:%d Total:%d (crush formula: %d)",
		u2.InputTokens, u2.CacheCreationTokens, u2.CacheReadTokens, u2.ReasoningTokens, u2.TotalTokens, totalPrompt2)

	if totalPrompt < 500 {
		t.Logf("FINDING: thinking mode causes proxy to report only ~%d tokens (system prompt excluded!).", totalPrompt)
		t.Logf("  This explains the 95-token UI display: system+tools tokens are missing when think=true.")
	} else {
		t.Logf("FINDING: thinking mode does NOT suppress system-prompt tokens (total=%d). Different cause for 95.", totalPrompt)
	}
}
func TestProviderUsage_AnthropicProxy_MultiTurn(t *testing.T) {
	lm, _ := anthropicProxyProvider(t)

	maxTokens := int64(50)

	// Turn 1.
	resp1, err := lm.Generate(context.Background(), fantasy.Call{
		Prompt: fantasy.Prompt{
			fantasy.NewUserMessage("What is 2+2? Reply with just the number."),
		},
		MaxOutputTokens: &maxTokens,
	})
	require.NoError(t, err)
	u1 := resp1.Usage

	// Turn 2: include previous exchange in context.
	resp2, err := lm.Generate(context.Background(), fantasy.Call{
		Prompt: fantasy.Prompt{
			fantasy.NewUserMessage("What is 2+2? Reply with just the number."),
			{
				Role:    fantasy.MessageRoleAssistant,
				Content: []fantasy.MessagePart{fantasy.TextPart{Text: resp1.Content.Text()}},
			},
			fantasy.NewUserMessage("And what is 3+3? Reply with just the number."),
		},
		MaxOutputTokens: &maxTokens,
	})
	require.NoError(t, err)
	u2 := resp2.Usage

	t.Logf("=== Multi-turn usage for qwen3.5-plus (anthropic-proxy) ===")
	t.Logf("Turn 1 — Input: %d, Output: %d, CacheCreate: %d, CacheRead: %d, Total: %d",
		u1.InputTokens, u1.OutputTokens, u1.CacheCreationTokens, u1.CacheReadTokens, u1.TotalTokens)
	t.Logf("Turn 2 — Input: %d, Output: %d, CacheCreate: %d, CacheRead: %d, Total: %d",
		u2.InputTokens, u2.OutputTokens, u2.CacheCreationTokens, u2.CacheReadTokens, u2.TotalTokens)

	turn1Prompt := u1.InputTokens + u1.CacheCreationTokens + u1.CacheReadTokens
	turn2Prompt := u2.InputTokens + u2.CacheCreationTokens + u2.CacheReadTokens
	t.Logf("crush promptTokensForUsage — Turn1: %d, Turn2: %d", turn1Prompt, turn2Prompt)
	t.Logf("(OLD display) cumulative: %d — (NEW display) last step: %d",
		turn1Prompt+turn2Prompt, turn2Prompt)

	// Turn 2 should use more input tokens because it carries the full conversation.
	if u1.InputTokens == 0 && u2.InputTokens == 0 {
		t.Log("FINDING: provider reports 0 input tokens on both turns — crush context display will show near-zero")
	} else {
		require.GreaterOrEqual(t, turn2Prompt, turn1Prompt,
			"Turn 2 should use >= input tokens as Turn 1 (larger context window)")
	}
}

// TestProviderUsage_AnthropicProxy_StreamingStepUsage verifies that the
// OnStepFinish callback receives the same usage values as resp.TotalUsage.
// This tests the exact code path crush uses: agent.Stream with OnStepFinish.
func TestProviderUsage_AnthropicProxy_StreamingStepUsage(t *testing.T) {
	lm, _ := anthropicProxyProvider(t)

	systemPromptText := string(coderPromptTmpl)
	maxTokens := int64(100)

	cacheOptions := fantasy.ProviderOptions{
		anthropic.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
	}
	sysMsg := fantasy.NewSystemMessage(systemPromptText)
	sysMsg.ProviderOptions = cacheOptions

	agent := fantasy.NewAgent(lm,
		fantasy.WithSystemPrompt(systemPromptText),
		fantasy.WithMaxOutputTokens(maxTokens),
	)

	var stepUsages []fantasy.Usage
	resp, err := agent.Stream(context.Background(), fantasy.AgentStreamCall{
		Prompt: "Reply with exactly: hello",
		PrepareStep: func(ctx context.Context, opts fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = opts.Messages
			// Apply cache_control to last system message (same as crush).
			for i, msg := range prepared.Messages {
				if msg.Role == fantasy.MessageRoleSystem {
					prepared.Messages[i].ProviderOptions = cacheOptions
				}
			}
			// Apply cache_control to last 2 messages (same as crush).
			for i := max(0, len(prepared.Messages)-2); i < len(prepared.Messages); i++ {
				prepared.Messages[i].ProviderOptions = cacheOptions
			}
			return ctx, prepared, nil
		},
		OnStepFinish: func(stepResult fantasy.StepResult) error {
			stepUsages = append(stepUsages, stepResult.Usage)
			return nil
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, stepUsages, 1, "Expected exactly 1 step (no tool use)")

	su := stepUsages[0]
	tu := resp.TotalUsage
	stepPrompt := su.InputTokens + su.CacheCreationTokens + su.CacheReadTokens
	totalPrompt := tu.InputTokens + tu.CacheCreationTokens + tu.CacheReadTokens

	t.Logf("=== Streaming step usage vs TotalUsage (sys prompt: %d bytes) ===", len(systemPromptText))
	t.Logf("StepResult.Usage  — Input:%d Output:%d CacheCreate:%d CacheRead:%d Reasoning:%d → promptTokensForUsage=%d",
		su.InputTokens, su.OutputTokens, su.CacheCreationTokens, su.CacheReadTokens, su.ReasoningTokens, stepPrompt)
	t.Logf("resp.TotalUsage   — Input:%d Output:%d CacheCreate:%d CacheRead:%d Reasoning:%d → promptTokensForUsage=%d",
		tu.InputTokens, tu.OutputTokens, tu.CacheCreationTokens, tu.CacheReadTokens, tu.ReasoningTokens, totalPrompt)

	if stepPrompt != totalPrompt {
		t.Errorf("MISMATCH: StepResult prompt tokens (%d) != TotalUsage prompt tokens (%d)", stepPrompt, totalPrompt)
	}

	if stepPrompt < 500 {
		t.Errorf("FINDING: step prompt tokens only %d — system prompt NOT counted in streaming step!", stepPrompt)
	} else {
		t.Logf("OK: step prompt tokens=%d correctly includes system prompt", stepPrompt)
	}
}
