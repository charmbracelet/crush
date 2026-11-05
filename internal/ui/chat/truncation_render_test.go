package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// TestTruncationFinishRendersBanner asserts that a max-tokens finish
// paints a visible TRUNCATED banner rather than a turn that simply
// looks complete while stopping mid-sentence. As with refusals, the
// agent persists only the reason; the TUI owns this copy.
func TestTruncationFinishRendersBanner(t *testing.T) {
	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:   "truncated-1",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "The answer starts here and"},
			message.Finish{
				Reason: message.FinishReasonMaxTokens,
				Time:   testFinishTime,
			},
		},
	}

	require.True(t, ShouldRenderAssistantMessage(msg),
		"a truncated turn must still render so the banner is visible")

	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	out := item.Render(80)
	require.Contains(t, out, truncatedTagLabel, "banner tag")
	require.Contains(t, out, truncatedTitle, "title")
	require.Contains(t, strings.ToLower(out), "max tokens", "actionable hint")
}

// TestTruncationEmptyBodyStillRenders covers the worst case for silent
// truncation: reasoning consumed the entire token budget, so the turn
// has neither text nor tool calls. The message must still render so
// the banner can explain why nothing came back.
func TestTruncationEmptyBodyStillRenders(t *testing.T) {
	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:   "truncated-2",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "call-1", Name: "bash", Input: `{"command": "ls"}`},
			message.Finish{
				Reason: message.FinishReasonMaxTokens,
				Time:   testFinishTime,
			},
		},
	}

	// Tool calls but no body: normally an unrendered intermediate step.
	require.True(t, ShouldRenderAssistantMessage(msg),
		"a truncated tool-only turn must render its banner")

	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	require.Contains(t, item.Render(80), truncatedTagLabel)
}

// TestTruncationPersistedCopyWins asserts that when a finish part
// carries its own message/details (e.g. a provider that supplies
// them), the persisted copy takes precedence over the TUI defaults.
func TestTruncationPersistedCopyWins(t *testing.T) {
	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:   "truncated-3",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.Finish{
				Reason:  message.FinishReasonMaxTokens,
				Message: "Custom truncation title",
				Details: "Custom truncation details.",
				Time:    testFinishTime,
			},
		},
	}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	out := item.Render(80)
	require.Contains(t, out, truncatedTagLabel)
	require.Contains(t, out, "Custom truncation title")
	require.Contains(t, out, "Custom truncation details.")
}

// TestTruncationInfoItemKeepsFullFooter covers the compact turn
// header: a truncated turn is the last turn of its prompt (the model
// cannot continue past the token limit), so it must render the full
// footer with duration and separator line rather than the compact
// intermediate-step header, or the banner reads as one step of a loop
// that is still running.
func TestTruncationInfoItemKeepsFullFooter(t *testing.T) {
	sty := styles.CharmtonePantera()
	// GetModel dereferences the Providers map, so it must be
	// initialized even though no models are configured: the
	// Prism-routed branch supplies the model name.
	cfg := &config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
	finishTime := time.Unix(testFinishTime, 0)

	build := func(reason message.FinishReason) *message.Message {
		return &message.Message{
			ID:             "info",
			Role:           message.Assistant,
			Provider:       "hyper",
			Model:          "prism",
			PrismModelName: "Prism Router",
			Parts: []message.ContentPart{
				message.Finish{Reason: reason, Time: finishTime.Unix()},
			},
		}
	}
	// Prism-routed turns show the info item even for non-EndTurn
	// finishes; that is exactly the compact-header path.
	require.True(t, ShouldShowAssistantInfo(build(message.FinishReasonMaxTokens)))

	info := NewAssistantInfoItem(&sty, build(message.FinishReasonMaxTokens), cfg, time.Unix(0, 0))
	out := info.Render(80)
	require.Contains(t, out, "in ", "duration footer")
	require.Contains(t, out, "Prism Router", "model line")

	// The same holds without a Prism route: a truncated turn on an
	// ordinary model is still the last turn of its prompt, so it gets
	// the info footer rather than the compact header that reads as a
	// running loop.
	plain := build(message.FinishReasonMaxTokens)
	plain.PrismModelName = ""
	require.True(t, ShouldShowAssistantInfo(plain))
	plainInfo := NewAssistantInfoItem(&sty, plain, cfg, time.Unix(0, 0))
	require.Contains(t, plainInfo.Render(80), "in ", "duration footer without Prism")

	// An intermediate tool-use turn still renders the compact header.
	compact := NewAssistantInfoItem(&sty, build(message.FinishReasonToolUse), cfg, time.Unix(0, 0))
	require.NotContains(t, compact.Render(80), "in ",
		"intermediate turns keep the compact header")
}
