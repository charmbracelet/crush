package chat

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// largeMarkdown returns roughly size bytes of paragraphed markdown prose.
func largeMarkdown(size int) string {
	var sb strings.Builder
	for i := 0; sb.Len() < size; i++ {
		fmt.Fprintf(&sb, "Paragraph %d explains another step of the plan in some detail, with enough words to wrap across a couple of lines in the chat view.\n\n", i)
	}
	return sb.String()
}

// BenchmarkColdRenderLargeMessage measures the first render of a finished
// assistant message carrying a large body or a large reasoning trace, as
// happens when a session loads or the cache is dropped.
func BenchmarkColdRenderLargeMessage(b *testing.B) {
	sty := styles.CharmtonePantera()
	cases := []struct {
		name      string
		content   int
		reasoning int
	}{
		{"content=144KB", 144 << 10, 0},
		{"reasoning=100KB", 0, 100 << 10},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			msg := &message.Message{ID: "a", Role: message.Assistant}
			if tc.reasoning > 0 {
				msg.AppendReasoningContent(largeMarkdown(tc.reasoning))
				msg.FinishThinking()
			}
			if tc.content > 0 {
				msg.AppendContent(largeMarkdown(tc.content))
			}
			msg.AddFinish(message.FinishReasonEndTurn, "", "")
			b.ReportAllocs()
			for b.Loop() {
				item := NewAssistantMessageItem(&sty, msg)
				_ = item.Render(120)
			}
		})
	}
}
