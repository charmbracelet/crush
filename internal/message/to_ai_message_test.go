package message

import (
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"github.com/stretchr/testify/require"
)

// TestToAIMessage_ReasoningWithSignature verifies that when a reasoning block
// has a real signature (as returned by native Anthropic), ToAIMessage forwards
// it as a thinking block with that signature intact.
func TestToAIMessage_ReasoningWithSignature(t *testing.T) {
	t.Parallel()

	msg := Message{
		Role: Assistant,
		Parts: []ContentPart{
			TextContent{Text: "answer"},
			ReasoningContent{
				Thinking:  "my thoughts",
				Signature: "sig-abc",
			},
		},
	}

	aiMsgs := msg.ToAIMessage()
	require.Len(t, aiMsgs, 1)

	var found bool
	for _, part := range aiMsgs[0].Content {
		rp, ok := fantasy.AsContentType[fantasy.ReasoningPart](part)
		if !ok {
			continue
		}
		found = true
		opts := rp.Options()
		meta, ok := opts[anthropic.Name].(*anthropic.ReasoningOptionMetadata)
		require.True(t, ok, "expected anthropic.ReasoningOptionMetadata in provider options")
		require.Equal(t, "sig-abc", meta.Signature)
		require.Empty(t, meta.RedactedData)
	}
	require.True(t, found, "expected a ReasoningPart in the message")
}

// TestToAIMessage_ReasoningWithoutSignature validates that when an
// anthropic-compatible proxy returns thinking content but no signature and no
// other provider metadata, ToAIMessage omits the reasoning part entirely.
// Sending a redacted_thinking block is not viable because Kimi rejects it;
// and without a signature, native Anthropic would also reject the request.
// Since Kimi always returns a real signature in practice, this case should be
// rare, and the safest behaviour is to omit the reasoning part rather than
// send an invalid placeholder.
func TestToAIMessage_ReasoningWithoutSignature(t *testing.T) {
	t.Parallel()

	msg := Message{
		Role: Assistant,
		Parts: []ContentPart{
			TextContent{Text: "answer"},
			ReasoningContent{
				Thinking:  "my thoughts",
				Signature: "", // no signature, no other provider metadata
			},
		},
	}

	aiMsgs := msg.ToAIMessage()
	require.Len(t, aiMsgs, 1)

	for _, part := range aiMsgs[0].Content {
		_, isReasoning := fantasy.AsContentType[fantasy.ReasoningPart](part)
		require.False(t, isReasoning, "reasoning part without any provider metadata must be omitted")
	}
}

// TestToAIMessage_ReasoningBeforeText verifies that the reasoning (thinking)
// block appears before the text block in the assistant message parts.  Kimi
// (and Anthropic) require this ordering; sending text first causes a 400 error.
func TestToAIMessage_ReasoningBeforeText(t *testing.T) {
	t.Parallel()

	msg := Message{
		Role: Assistant,
		Parts: []ContentPart{
			TextContent{Text: "answer"},
			ReasoningContent{
				Thinking:  "my thoughts",
				Signature: "sig-abc", // must have a signature to be included
			},
		},
	}

	aiMsgs := msg.ToAIMessage()
	require.Len(t, aiMsgs, 1)

	parts := aiMsgs[0].Content
	require.True(t, len(parts) >= 2, "expected at least reasoning + text parts")

	_, isReasoning := fantasy.AsContentType[fantasy.ReasoningPart](parts[0])
	require.True(t, isReasoning, "first part must be ReasoningPart, got %T", parts[0])

	_, isText := fantasy.AsContentType[fantasy.TextPart](parts[1])
	require.True(t, isText, "second part must be TextPart, got %T", parts[1])
}

// TestToAIMessage_ReasoningOnlyAffectsAnthropicWhenNoSignature ensures that
// when reasoning has a ThoughtSignature (Google) or ResponsesData (OpenAI),
// those provider options remain present regardless of the Anthropic signature.
func TestToAIMessage_ReasoningOtherProviderOptionsPreserved(t *testing.T) {
	t.Parallel()

	msg := Message{
		Role: Assistant,
		Parts: []ContentPart{
			TextContent{Text: "answer"},
			ReasoningContent{
				Thinking:  "my thoughts",
				Signature: "sig-real",
			},
		},
	}

	aiMsgs := msg.ToAIMessage()
	require.Len(t, aiMsgs, 1)

	for _, part := range aiMsgs[0].Content {
		rp, ok := fantasy.AsContentType[fantasy.ReasoningPart](part)
		if !ok {
			continue
		}
		meta, ok := rp.Options()[anthropic.Name].(*anthropic.ReasoningOptionMetadata)
		require.True(t, ok)
		require.Equal(t, "sig-real", meta.Signature)
		require.Empty(t, meta.RedactedData)
	}
}
