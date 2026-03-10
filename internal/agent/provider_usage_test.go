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

// TestProviderUsage_AnthropicProxy_SingleTurn verifies what usage fields
// the anthropic-proxy provider actually returns for qwen3.5-plus.
//
// Expected finding: InputTokens > 0 (provider correctly reports usage).
// The context display bug was cumulative accumulation, not missing input tokens.
func TestProviderUsage_AnthropicProxy_SingleTurn(t *testing.T) {
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
	t.Logf("=== Usage: qwen3.5-plus (anthropic-proxy), single turn ===")
	t.Logf("  InputTokens:         %d", u.InputTokens)
	t.Logf("  OutputTokens:        %d", u.OutputTokens)
	t.Logf("  CacheCreationTokens: %d", u.CacheCreationTokens)
	t.Logf("  CacheReadTokens:     %d", u.CacheReadTokens)
	t.Logf("  TotalTokens:         %d", u.TotalTokens)
	t.Logf("  promptTokensForUsage (crush formula): %d",
		u.InputTokens+u.CacheCreationTokens+u.CacheReadTokens)
	t.Logf("  Response text: %q", resp.Content.Text())

	require.Greater(t, u.OutputTokens, int64(0), "OutputTokens must be > 0")

	// Verified: anthropic-proxy/qwen3.5-plus correctly reports input tokens.
	// The context display bug was the cumulative PromptTokens accumulation,
	// not a missing input token issue from the provider.
	require.Greater(t, u.InputTokens, int64(0),
		"InputTokens should be > 0; if this fails, the provider is not reporting input token count")
}

// TestProviderUsage_AnthropicProxy_MultiTurn verifies that in a two-turn
// conversation the second call's InputTokens grows (reflects accumulated context),
// which is the foundation for the per-step LastPromptTokens fix.
func TestProviderUsage_AnthropicProxy_MultiTurn(t *testing.T) {
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
