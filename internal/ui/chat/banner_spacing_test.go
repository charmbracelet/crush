package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func assistantWith(parts ...message.ContentPart) *message.Message {
	return &message.Message{Role: message.Assistant, Parts: parts}
}

// A refusal or cancellation banner butted straight against the model's last
// line, so it read as the end of the sentence rather than a separate notice.
func TestFinishBannerIsSetOffFromContent(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	for _, tc := range []struct {
		name string
		msg  *message.Message
	}{
		{"refusal", assistantWith(
			message.TextContent{Text: "Now the real test, replay:"},
			message.Finish{Reason: message.FinishReasonContentFilter, Time: 1},
		)},
		{"cancelled", assistantWith(
			message.TextContent{Text: "Now the real test, replay:"},
			message.Finish{Reason: message.FinishReasonCanceled, Time: 1},
		)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			item, ok := NewAssistantMessageItem(&sty, tc.msg).(*AssistantMessageItem)
			require.True(t, ok)

			out, _ := item.renderMessageContent(80)
			lines := strings.Split(stripStyle(out), "\n")

			bannerAt := -1
			for i, l := range lines {
				if i > 0 && strings.TrimSpace(l) != "" && !strings.Contains(l, "replay") {
					bannerAt = i
					break
				}
			}
			require.Greater(t, bannerAt, 0, "expected a banner after the content")
			require.Empty(t, strings.TrimSpace(lines[bannerAt-1]),
				"the banner needs a blank line above it")
		})
	}
}

func stripStyle(s string) string {
	var b strings.Builder
	var esc bool
	for _, r := range s {
		switch {
		case r == 0x1b:
			esc = true
		case esc && r == 'm':
			esc = false
		case !esc:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// A cancelled turn that made tool calls renders those calls below this item,
// each with its own interrupted result. Repeating "Canceled" here put it
// above the tools it described.
func TestCanceledBannerYieldsToToolCalls(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	withTools := assistantWith(
		message.TextContent{Text: "Now the discriminating test:"},
		message.ToolCall{ID: "c1", Name: "mcp__prickly__find", Finished: true},
		message.Finish{Reason: message.FinishReasonCanceled, Time: 1},
	)
	item, ok := NewAssistantMessageItem(&sty, withTools).(*AssistantMessageItem)
	require.True(t, ok)
	out, _ := item.renderMessageContent(80)
	require.NotContains(t, stripStyle(out), "Canceled",
		"the tool rows carry the outcome; this would sit above them")

	// With no tool calls there is nothing else to say it, so it stays.
	bare := assistantWith(
		message.TextContent{Text: "Now the discriminating test:"},
		message.Finish{Reason: message.FinishReasonCanceled, Time: 1},
	)
	item, ok = NewAssistantMessageItem(&sty, bare).(*AssistantMessageItem)
	require.True(t, ok)
	out, _ = item.renderMessageContent(80)
	require.Contains(t, stripStyle(out), "Canceled")
}
