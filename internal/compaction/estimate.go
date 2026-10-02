package compaction

import (
	"strings"

	"github.com/charmbracelet/crush/internal/message"
)

const (
	// charsPerToken is Crush's standing approximation for a token; the
	// provider is authoritative and this only has to be close enough to
	// place a cut point.
	charsPerToken = 4
	// framingTokens approximates the JSON structure a provider re-serializes
	// around a tool call or tool result (ids, names, role markers).
	framingTokens = 16
	// imageTokens approximates one image part. Providers bill images by
	// tile rather than by character; one constant is close enough for
	// sizing a cut and beats counting base64 bytes.
	imageTokens = 1_600
)

// Estimate returns the approximate number of context tokens a message
// occupies when it is sent to a model.
func Estimate(msg message.Message) int64 {
	var total int64
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case message.TextContent:
			total += textTokens(p.Text)
		case message.ReasoningContent:
			total += textTokens(p.Thinking)
		case message.ToolCall:
			total += framingTokens + textTokens(p.Name) + textTokens(p.Input)
		case message.ToolResult:
			// Metadata is not counted on purpose: it is UI bookkeeping, where
			// tools park exit status and spill paths for the TUI to render,
			// and it never reaches a provider. Including it made one real
			// session estimate at 4x what the provider billed for it, which
			// in turn made compaction fire at a quarter of the window.
			total += framingTokens + textTokens(p.Content)
			switch {
			case p.Data == "":
			case strings.HasPrefix(p.MIMEType, "image/"):
				// Images are billed by tile rather than by the length of
				// their base64 encoding, which is three quarters bigger than
				// the bytes themselves.
				total += imageTokens
			default:
				total += textTokens(p.Data)
			}
		case message.ShellCommand:
			total += framingTokens + textTokens(p.Command) + textTokens(p.Output)
		case message.ImageURLContent:
			total += imageTokens
		case message.BinaryContent:
			total += binaryTokens(p)
		}
	}
	return total
}

// EstimateAll sums Estimate over msgs.
func EstimateAll(msgs []message.Message) int64 {
	var total int64
	for _, msg := range msgs {
		total += Estimate(msg)
	}
	return total
}

func textTokens(s string) int64 {
	if s == "" {
		return 0
	}
	return int64((len(s) + charsPerToken - 1) / charsPerToken)
}

func binaryTokens(p message.BinaryContent) int64 {
	if strings.HasPrefix(p.MIMEType, "text/") {
		return textTokens(string(p.Data))
	}
	return imageTokens
}

// Tokens returns the approximate context cost of a plain string, using the
// same measure as Estimate so checkpoint bookkeeping adds up.
func Tokens(s string) int64 {
	return textTokens(s)
}
