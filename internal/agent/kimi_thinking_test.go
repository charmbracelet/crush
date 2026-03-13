package agent

// Diagnostic tests for Kimi K2.5 multi-turn thinking via anthropic-proxy.
//
// Run with:
//
//	go test ./internal/agent/... -run TestKimiThinking -v

import (
	"context"
	"fmt"
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// kimiProvider returns a LanguageModel for kimi-k2.5 via the anthropic-proxy
// provider configured in crush.json, skipping if not available.
func kimiProvider(t *testing.T) fantasy.LanguageModel {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping real API test in short mode")
	}
	cfg, err := config.Init(t.TempDir(), "", false)
	require.NoError(t, err)

	providerCfg, ok := cfg.Config().Providers.Get("anthropic-proxy")
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

	lm, err := provider.LanguageModel(context.Background(), "kimi-k2.5")
	require.NoError(t, err)
	return lm
}

func kimiThinkingOptions() fantasy.ProviderOptions {
	budget := int64(5000)
	return fantasy.ProviderOptions{
		anthropic.Name: &anthropic.ProviderOptions{
			Thinking: &anthropic.ThinkingProviderOption{BudgetTokens: budget},
		},
	}
}

// TestKimiThinking_StreamEvents prints every raw stream event from a single
// thinking turn, so we can see exactly what events Kimi returns and whether
// signature_delta appears.
func TestKimiThinking_StreamEvents(t *testing.T) {
	lm := kimiProvider(t)

	maxTokens := int64(1000)
	stream, err := lm.Stream(context.Background(), fantasy.Call{
		Prompt: fantasy.Prompt{
			fantasy.NewUserMessage("用一句话介绍一下你自己"),
		},
		MaxOutputTokens: &maxTokens,
		ProviderOptions: kimiThinkingOptions(),
	})
	require.NoError(t, err)

	t.Log("=== Stream events from single thinking turn ===")
	eventCount := 0
	for part := range stream {
		eventCount++
		switch part.Type {
		case fantasy.StreamPartTypeReasoningStart:
			t.Logf("[%d] ReasoningStart id=%s providerMeta=%+v", eventCount, part.ID, part.ProviderMetadata)
		case fantasy.StreamPartTypeReasoningDelta:
			if part.Delta != "" {
				t.Logf("[%d] ReasoningDelta id=%s delta_len=%d providerMeta=%+v", eventCount, part.ID, len(part.Delta), part.ProviderMetadata)
			} else {
				// signature_delta arrives as ReasoningDelta with empty Delta but ProviderMetadata set
				t.Logf("[%d] ReasoningDelta(signature?) id=%s delta=%q providerMeta=%+v", eventCount, part.ID, part.Delta, part.ProviderMetadata)
				if part.ProviderMetadata != nil {
					if meta, ok := part.ProviderMetadata[anthropic.Name]; ok {
						t.Logf("         anthropic meta: %+v", meta)
						if rm, ok := meta.(*anthropic.ReasoningOptionMetadata); ok {
							t.Logf("         Signature=%q RedactedData=%q", rm.Signature, rm.RedactedData)
						}
					}
				}
			}
		case fantasy.StreamPartTypeReasoningEnd:
			t.Logf("[%d] ReasoningEnd id=%s providerMeta=%+v", eventCount, part.ID, part.ProviderMetadata)
			if part.ProviderMetadata != nil {
				if meta, ok := part.ProviderMetadata[anthropic.Name]; ok {
					if rm, ok := meta.(*anthropic.ReasoningOptionMetadata); ok {
						t.Logf("         Signature=%q RedactedData=%q", rm.Signature, rm.RedactedData)
					}
				}
			}
		case fantasy.StreamPartTypeFinish:
			t.Logf("[%d] Finish reason=%s", eventCount, part.FinishReason)
		case fantasy.StreamPartTypeError:
			t.Logf("[%d] Error: %v", eventCount, part.Error)
		default:
			t.Logf("[%d] type=%v id=%s", eventCount, part.Type, part.ID)
		}
	}
}

// TestKimiThinking_OnReasoningEnd checks what OnReasoningEnd receives for
// Kimi: specifically whether ProviderMetadata contains the signature and
// what Signature/RedactedData values look like.
func TestKimiThinking_OnReasoningEnd(t *testing.T) {
	lm := kimiProvider(t)

	maxTokens := int64(1000)
	agent := fantasy.NewAgent(lm, fantasy.WithMaxOutputTokens(maxTokens))

	type reasoningCapture struct {
		id   string
		text string
		meta *anthropic.ReasoningOptionMetadata
	}
	var captures []reasoningCapture

	_, err := agent.Stream(context.Background(), fantasy.AgentStreamCall{
		Prompt: "用一句话介绍一下你自己",
		PrepareStep: func(ctx context.Context, opts fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = opts.Messages
			return ctx, prepared, nil
		},
		OnReasoningDelta: func(id string, delta string) error {
			return nil
		},
		OnReasoningEnd: func(id string, reasoning fantasy.ReasoningContent) error {
			cap := reasoningCapture{id: id, text: reasoning.Text}
			if reasoning.ProviderMetadata != nil {
				if meta, ok := reasoning.ProviderMetadata[anthropic.Name]; ok {
					if rm, ok := meta.(*anthropic.ReasoningOptionMetadata); ok {
						cap.meta = rm
					}
				}
			}
			captures = append(captures, cap)
			return nil
		},
		ProviderOptions: kimiThinkingOptions(),
	})
	require.NoError(t, err)

	t.Logf("=== OnReasoningEnd captures ===")
	for i, c := range captures {
		t.Logf("[%d] id=%s text_len=%d meta=%+v", i, c.id, len(c.text), c.meta)
		if c.meta != nil {
			t.Logf("     Signature=%q (len=%d)", c.meta.Signature, len(c.meta.Signature))
			t.Logf("     RedactedData=%q", c.meta.RedactedData)
		} else {
			t.Log("     meta=nil (NO anthropic ProviderMetadata!)")
		}
	}

	require.NotEmpty(t, captures, "expected at least one OnReasoningEnd call")
}

// TestKimiThinking_MultiTurn is the key test: simulates exactly what crush
// does in a multi-turn thinking conversation, using the ACTUAL values from
// OnReasoningEnd to build the second turn's assistant message.
// This test will fail with the current bug and pass after the fix.
func TestKimiThinking_MultiTurn(t *testing.T) {
	lm := kimiProvider(t)

	maxTokens := int64(2000)
	thinkingOpts := kimiThinkingOptions()

	// ---- Turn 1: simple question with thinking ----
	t.Log("=== Turn 1 ===")
	var (
		turn1ReasoningText string
		turn1ReasoningMeta *anthropic.ReasoningOptionMetadata
		turn1Text          string
	)

	agent1 := fantasy.NewAgent(lm, fantasy.WithMaxOutputTokens(maxTokens))
	_, err := agent1.Stream(context.Background(), fantasy.AgentStreamCall{
		Prompt: "用一句话介绍一下你自己",
		PrepareStep: func(ctx context.Context, opts fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = opts.Messages
			return ctx, prepared, nil
		},
		OnReasoningEnd: func(id string, reasoning fantasy.ReasoningContent) error {
			turn1ReasoningText = reasoning.Text
			if reasoning.ProviderMetadata != nil {
				if meta, ok := reasoning.ProviderMetadata[anthropic.Name]; ok {
					if rm, ok := meta.(*anthropic.ReasoningOptionMetadata); ok {
						turn1ReasoningMeta = rm
					}
				}
			}
			return nil
		},
		OnTextDelta: func(id string, text string) error {
			turn1Text += text
			return nil
		},
		ProviderOptions: thinkingOpts,
	})
	require.NoError(t, err)

	t.Logf("Turn1 reasoning text_len=%d", len(turn1ReasoningText))
	t.Logf("Turn1 reasoning meta=%+v", turn1ReasoningMeta)
	if turn1ReasoningMeta != nil {
		t.Logf("  Signature=%q (len=%d)", turn1ReasoningMeta.Signature, len(turn1ReasoningMeta.Signature))
		t.Logf("  RedactedData=%q", turn1ReasoningMeta.RedactedData)
	}
	t.Logf("Turn1 text=%q", turn1Text)

	// ---- Build the turn1 assistant message exactly as crush does ----
	// This mirrors ToAIMessage() logic in internal/message/content.go
	var assistantParts []fantasy.MessagePart

	// reasoning first (our fix ensures this ordering)
	if turn1ReasoningText != "" {
		reasoningPart := fantasy.ReasoningPart{
			Text:            turn1ReasoningText,
			ProviderOptions: fantasy.ProviderOptions{},
		}
		if turn1ReasoningMeta != nil && turn1ReasoningMeta.Signature != "" {
			reasoningPart.ProviderOptions[anthropic.Name] = &anthropic.ReasoningOptionMetadata{
				Signature: turn1ReasoningMeta.Signature,
			}
		} else {
			// fallback for proxy without signature
			reasoningPart.ProviderOptions[anthropic.Name] = &anthropic.ReasoningOptionMetadata{
				RedactedData: "thinking_redacted",
			}
		}
		assistantParts = append(assistantParts, reasoningPart)
	}
	if turn1Text != "" {
		assistantParts = append(assistantParts, fantasy.TextPart{Text: turn1Text})
	}

	assistantMsg := fantasy.Message{
		Role:    fantasy.MessageRoleAssistant,
		Content: assistantParts,
	}
	t.Logf("Turn1 assistant msg content types: %v", func() []string {
		var types []string
		for _, p := range assistantMsg.Content {
			types = append(types, fmt.Sprintf("%T", p))
		}
		return types
	}())

	// ---- Turn 2: send conversation history + new question ----
	t.Log("=== Turn 2 ===")
	var turn2Text string

	resp2, err := lm.Generate(context.Background(), fantasy.Call{
		Prompt: fantasy.Prompt{
			fantasy.NewUserMessage("用一句话介绍一下你自己"),
			assistantMsg,
			fantasy.NewUserMessage("你能做什么？用一句话回答。"),
		},
		MaxOutputTokens: &maxTokens,
		ProviderOptions: thinkingOpts,
	})
	if err != nil {
		t.Logf("Turn 2 ERROR: %v", err)
		t.FailNow()
	}
	turn2Text = resp2.Content.Text()
	t.Logf("Turn 2 response: %q", turn2Text)
	require.NotEmpty(t, turn2Text, "Turn 2 should return a response")
}

// TestKimiThinking_RedactedDataFormats tries different values for RedactedData
// to find what Kimi actually accepts as valid, when signature is absent.
func TestKimiThinking_RedactedDataFormats(t *testing.T) {
	lm := kimiProvider(t)

	maxTokens := int64(500)

	// First get a real turn1 with thinking.
	var turn1Text string
	var turn1ReasoningText string

	agent1 := fantasy.NewAgent(lm, fantasy.WithMaxOutputTokens(maxTokens))
	_, err := agent1.Stream(context.Background(), fantasy.AgentStreamCall{
		Prompt: "用一句话回答：1+1等于几？",
		PrepareStep: func(ctx context.Context, opts fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = opts.Messages
			return ctx, prepared, nil
		},
		OnReasoningEnd: func(id string, reasoning fantasy.ReasoningContent) error {
			turn1ReasoningText = reasoning.Text
			return nil
		},
		OnTextDelta: func(id string, text string) error {
			turn1Text += text
			return nil
		},
		ProviderOptions: kimiThinkingOptions(),
	})
	require.NoError(t, err)
	t.Logf("Turn1 text=%q reasoning_len=%d", turn1Text, len(turn1ReasoningText))

	// Now try different RedactedData values for turn2.
	candidates := []struct {
		name string
		data string
	}{
		{"empty_string", ""},
		{"placeholder", "thinking_redacted"},
		{"base64_a", "YQ=="},                 // "a" in base64 (1 byte)
		{"base64_ab", "YWI="},                // "ab" in base64 (2 bytes)
		{"base64_abc", "YWJj"},               // "abc" in base64 (3 bytes)
		{"base64_hello", "aGVsbG8="},         // "hello" in base64 (5 bytes)
		{"base64_null1", "AA=="},             // single null byte
		{"base64_null4", "AAAAAA=="},         // 4 null bytes
		{"base64_null8", "AAAAAAAAAA=="},     // 8 null bytes
		{"base64_long", "AAAAAAAAAAAAAAAA="}, // padding bytes (12 bytes)
	}

	for _, c := range candidates {
		t.Run(c.name, func(t *testing.T) {
			var assistantParts []fantasy.MessagePart
			if turn1ReasoningText != "" && c.data != "" {
				rp := fantasy.ReasoningPart{
					Text:            turn1ReasoningText,
					ProviderOptions: fantasy.ProviderOptions{},
				}
				rp.ProviderOptions[anthropic.Name] = &anthropic.ReasoningOptionMetadata{
					RedactedData: c.data,
				}
				assistantParts = append(assistantParts, rp)
			}
			if turn1Text != "" {
				assistantParts = append(assistantParts, fantasy.TextPart{Text: turn1Text})
			}

			_, err := lm.Generate(context.Background(), fantasy.Call{
				Prompt: fantasy.Prompt{
					fantasy.NewUserMessage("用一句话回答：1+1等于几？"),
					{Role: fantasy.MessageRoleAssistant, Content: assistantParts},
					fantasy.NewUserMessage("2+2呢？"),
				},
				MaxOutputTokens: &maxTokens,
				ProviderOptions: kimiThinkingOptions(),
			})
			if err != nil {
				t.Logf("FAIL with RedactedData=%q: %v", c.data, err)
			} else {
				t.Logf("OK   with RedactedData=%q", c.data)
			}
		})
	}
}

// TestKimiThinking_WithToolCall verifies whether Kimi emits signature_delta
// when a thinking block is followed by a tool_use block.
// This is the exact scenario that causes the "reasoning_content missing" error.
func TestKimiThinking_WithToolCall(t *testing.T) {
	lm := kimiProvider(t)

	maxTokens := int64(3000)

	// Register a simple web_search tool that forces Kimi to use tool_use.
	type searchInput struct {
		Query string `json:"query" jsonschema:"description=search query"`
	}
	searchTool := fantasy.NewAgentTool("web_search", "Search the web for information",
		func(ctx context.Context, input searchInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse(`{"results": ["Xiaomi 15 Ultra released in 2025"]}`), nil
		},
	)

	agent := fantasy.NewAgent(lm,
		fantasy.WithMaxOutputTokens(maxTokens),
		fantasy.WithTools(searchTool),
	)

	type roundCapture struct {
		hasSignature bool
		signatureLen int
		hasMeta      bool
	}
	var rounds []roundCapture

	stream, err := agent.Stream(context.Background(), fantasy.AgentStreamCall{
		Prompt: "用web_search搜索一下最新的小米手机型号，然后告诉我结果。必须使用工具。",
		PrepareStep: func(ctx context.Context, opts fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = opts.Messages
			return ctx, prepared, nil
		},
		OnReasoningEnd: func(id string, reasoning fantasy.ReasoningContent) error {
			cap := roundCapture{}
			if reasoning.ProviderMetadata != nil {
				if meta, ok := reasoning.ProviderMetadata[anthropic.Name]; ok {
					cap.hasMeta = true
					if rm, ok := meta.(*anthropic.ReasoningOptionMetadata); ok {
						cap.signatureLen = len(rm.Signature)
						cap.hasSignature = rm.Signature != ""
						t.Logf("OnReasoningEnd: id=%s hasMeta=true Signature_len=%d RedactedData=%q", id, len(rm.Signature), rm.RedactedData)
					}
				}
			} else {
				t.Logf("OnReasoningEnd: id=%s ProviderMetadata=nil (NO SIGNATURE!)", id)
			}
			rounds = append(rounds, cap)
			return nil
		},
		ProviderOptions: kimiThinkingOptions(),
	})
	if err != nil {
		t.Logf("Stream error: %v", err)
		t.FailNow()
	}

	t.Logf("=== Tool-call thinking test: %d OnReasoningEnd calls ===", len(rounds))
	for i, r := range rounds {
		t.Logf("Round[%d]: hasMeta=%v hasSignature=%v signatureLen=%d", i, r.hasMeta, r.hasSignature, r.signatureLen)
	}

	if stream != nil {
		t.Logf("Final steps: %d", len(stream.Steps))
	}

	for i, r := range rounds {
		if !r.hasSignature {
			t.Errorf("Round[%d]: thinking block has NO signature — this is why multi-turn fails!", i)
		}
	}
}
