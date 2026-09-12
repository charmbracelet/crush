package agent

import (
	"github.com/charmbracelet/crush/internal/message"
)

// estimateRawMessageTokens provides a rough token estimate for a slice
// of internal message.Message (~4 chars per token). This is distinct
// from estimateMessageTokens which operates on fantasy.Message.
func estimateRawMessageTokens(msgs []message.Message) int {
	var totalChars int
	for _, m := range msgs {
		for _, part := range m.Parts {
			switch v := part.(type) {
			case message.TextContent:
				totalChars += len(v.Text)
			case message.ToolCall:
				totalChars += len(v.Input)
			case message.ToolResult:
				if v.Superseded != nil && v.Superseded.Applied {
					// The render emits the stub, not the stored
					// original — count what the model actually sees.
					totalChars += len(v.Superseded.StubText(v))
				} else {
					totalChars += len(v.Content)
				}
			case message.ReasoningContent:
				totalChars += len(v.Thinking)
			}
		}
	}
	return totalChars / 4
}
