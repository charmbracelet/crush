package message

import (
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestAutoModePromptRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []AutoModePromptType{
		AutoModePromptTypeFull,
		AutoModePromptTypeSparse,
		AutoModePromptTypeExit,
	}

	for _, promptType := range tests {
		t.Run(string(promptType), func(t *testing.T) {
			params := NewAutoModePromptMessage(promptType)
			require.Equal(t, System, params.Role)
			require.Len(t, params.Parts, 1)

			msg := Message{Role: params.Role, Parts: params.Parts}
			parsed, ok := ParseAutoModePrompt(msg)
			require.True(t, ok)
			require.Equal(t, promptType, parsed)
		})
	}
}

func TestToAIMessage_SystemRole(t *testing.T) {
	t.Parallel()

	msg := Message{
		Role: System,
		Parts: []ContentPart{
			TextContent{Text: "system guardrail text"},
		},
	}

	aiMsgs := msg.ToAIMessage()
	require.Len(t, aiMsgs, 1)
	require.Equal(t, fantasy.MessageRoleSystem, aiMsgs[0].Role)
	require.Len(t, aiMsgs[0].Content, 1)

	text, ok := fantasy.AsContentType[fantasy.TextPart](aiMsgs[0].Content[0])
	require.True(t, ok)
	require.Equal(t, "system guardrail text", text.Text)
}

func TestToAIMessage_SystemRoleEmptyTextSkipped(t *testing.T) {
	t.Parallel()

	msg := Message{
		Role: System,
		Parts: []ContentPart{
			TextContent{Text: "   "},
		},
	}

	aiMsgs := msg.ToAIMessage()
	require.Len(t, aiMsgs, 0)
}

func TestToAIMessage_AutoModePromptMarkerSkipped(t *testing.T) {
	t.Parallel()

	msg := Message{
		Role: System,
		Parts: []ContentPart{
			TextContent{Text: AutoModePromptContent(AutoModePromptTypeFull)},
		},
	}

	aiMsgs := msg.ToAIMessage()
	require.Len(t, aiMsgs, 0)
}

func TestToolResultAutoReviewRoundTripUsesStructuredField(t *testing.T) {
	t.Parallel()

	review := ToolResultAutoReview{
		Suspicious: true,
		Reason:     "local heuristic matched",
		Confidence: "low",
		Sanitized:  true,
	}

	result := ToolResult{
		Name: "view",
	}.WithAutoReview(review)

	parsed, ok := result.AutoReview()
	require.True(t, ok)
	require.Equal(t, review, parsed)
	require.NotEmpty(t, result.Metadata)
}

func TestParseToolResultAutoReviewIgnoresNonReviewMetadata(t *testing.T) {
	t.Parallel()

	review, ok := ParseToolResultAutoReview(`{"old_content":"a","new_content":"b","additions":1}`)
	require.False(t, ok)
	require.Equal(t, ToolResultAutoReview{}, review)
}

func TestWithAutoReviewPreservesExistingMetadata(t *testing.T) {
	t.Parallel()

	result := ToolResult{
		Metadata: `{"old_content":"a","new_content":"b"}`,
	}.WithAutoReview(ToolResultAutoReview{Sanitized: true, Reason: "x"})

	require.Contains(t, result.Metadata, `"old_content":"a"`)
	require.Contains(t, result.Metadata, `"new_content":"b"`)
	require.Contains(t, result.Metadata, `"sanitized":true`)
	require.Contains(t, result.Metadata, `"reason":"x"`)

	parsed, ok := result.AutoReview()
	require.True(t, ok)
	require.True(t, parsed.Sanitized)
	require.Equal(t, "x", parsed.Reason)
}

func TestWithAutoReview_NullMetadataNoPanic(t *testing.T) {
	t.Parallel()

	result := ToolResult{Metadata: `null`}.WithAutoReview(ToolResultAutoReview{
		Suspicious: true,
		Reason:     "x",
		Sanitized:  true,
	})

	require.NotEmpty(t, result.Metadata)
	require.Contains(t, result.Metadata, `"suspicious":true`)
	require.Contains(t, result.Metadata, `"reason":"x"`)
	require.Contains(t, result.Metadata, `"sanitized":true`)
}

func TestWithAutoReview_ClearsStaleReviewFields(t *testing.T) {
	t.Parallel()

	result := ToolResult{Metadata: `{"old_content":"a","sanitized":true,"reason":"old","confidence":"high"}`}.WithAutoReview(ToolResultAutoReview{
		Suspicious: true,
	})

	require.Contains(t, result.Metadata, `"old_content":"a"`)
	require.Contains(t, result.Metadata, `"suspicious":true`)
	require.NotContains(t, result.Metadata, `"sanitized":true`)
	require.NotContains(t, result.Metadata, `"reason":"old"`)
	require.NotContains(t, result.Metadata, `"confidence":"high"`)

	parsed, ok := result.AutoReview()
	require.True(t, ok)
	require.True(t, parsed.Suspicious)
	require.False(t, parsed.Sanitized)
	require.Empty(t, parsed.Reason)
	require.Empty(t, parsed.Confidence)
}

func TestWithAutoReview_InvalidMetadataFallsBackToReviewPayload(t *testing.T) {
	t.Parallel()

	result := ToolResult{Metadata: `not-json`}.WithAutoReview(ToolResultAutoReview{
		Sanitized: true,
		Reason:    "x",
	})

	require.Contains(t, result.Metadata, `"sanitized":true`)
	require.Contains(t, result.Metadata, `"reason":"x"`)

	parsed, ok := result.AutoReview()
	require.True(t, ok)
	require.True(t, parsed.Sanitized)
	require.Equal(t, "x", parsed.Reason)
}

func TestToolResultSubtaskResultRoundTripUsesStructuredMetadata(t *testing.T) {
	t.Parallel()

	result := ToolResult{
		Metadata: `{"existing":"value"}`,
	}.WithSubtaskResult(ToolResultSubtaskResult{
		ChildSessionID:   "child-1",
		ParentToolCallID: "call-1",
		Status:           ToolResultSubtaskStatusCompleted,
	})

	require.Contains(t, result.Metadata, `"existing":"value"`)
	require.Contains(t, result.Metadata, `"subtask_result":`)

	parsed, ok := result.SubtaskResult()
	require.True(t, ok)
	require.Equal(t, ToolResultSubtaskResult{
		ChildSessionID:   "child-1",
		ParentToolCallID: "call-1",
		Status:           ToolResultSubtaskStatusCompleted,
	}, parsed)
}
