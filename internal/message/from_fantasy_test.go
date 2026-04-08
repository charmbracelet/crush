package message

import (
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"github.com/stretchr/testify/require"
)

func TestFromFantasyMessages_SystemMessage(t *testing.T) {
	t.Parallel()

	msgs := []fantasy.Message{
		fantasy.NewSystemMessage("system prompt"),
	}

	result := FromFantasyMessages(msgs)
	require.Len(t, result, 1)
	require.Equal(t, System, result[0].Role)
	require.Equal(t, "system prompt", result[0].Content().Text)
}

func TestFromFantasyMessages_UserMessage(t *testing.T) {
	t.Parallel()

	msgs := []fantasy.Message{
		{
			Role: fantasy.MessageRoleUser,
			Content: []fantasy.MessagePart{
				fantasy.TextPart{Text: "hello"},
				fantasy.FilePart{
					Filename:  "test.txt",
					MediaType: "text/plain",
					Data:      []byte("content"),
				},
			},
		},
	}

	result := FromFantasyMessages(msgs)
	require.Len(t, result, 1)
	require.Equal(t, User, result[0].Role)
	require.Equal(t, "hello", result[0].Content().Text)

	binContent := result[0].BinaryContent()
	require.Len(t, binContent, 1)
	require.Equal(t, "test.txt", binContent[0].Path)
	require.Equal(t, "text/plain", binContent[0].MIMEType)
	require.Equal(t, []byte("content"), binContent[0].Data)
}

func TestFromFantasyMessages_AssistantMessage(t *testing.T) {
	t.Parallel()

	msgs := []fantasy.Message{
		{
			Role: fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{
				fantasy.TextPart{Text: "response"},
				fantasy.ToolCallPart{
					ToolCallID: "call-1",
					ToolName:   "bash",
					Input:      `{"command": "ls"}`,
				},
			},
		},
	}

	result := FromFantasyMessages(msgs)
	require.Len(t, result, 1)
	require.Equal(t, Assistant, result[0].Role)
	require.Equal(t, "response", result[0].Content().Text)

	toolCalls := result[0].ToolCalls()
	require.Len(t, toolCalls, 1)
	require.Equal(t, "call-1", toolCalls[0].ID)
	require.Equal(t, "bash", toolCalls[0].Name)
}

func TestFromFantasyMessages_AssistantWithReasoning(t *testing.T) {
	t.Parallel()

	msgs := []fantasy.Message{
		{
			Role: fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{
				fantasy.ReasoningPart{
					Text: "thinking...",
					ProviderOptions: fantasy.ProviderOptions{
						anthropic.Name: &anthropic.ReasoningOptionMetadata{
							Signature: "sig-123",
						},
					},
				},
				fantasy.TextPart{Text: "answer"},
			},
		},
	}

	result := FromFantasyMessages(msgs)
	require.Len(t, result, 1)
	require.Equal(t, Assistant, result[0].Role)

	reasoning := result[0].ReasoningContent()
	require.Equal(t, "thinking...", reasoning.Thinking)
	require.Equal(t, "sig-123", reasoning.Signature)
}

func TestFromFantasyMessages_ToolMessage(t *testing.T) {
	t.Parallel()

	msgs := []fantasy.Message{
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: "call-1",
					Output:     fantasy.ToolResultOutputContentText{Text: "output"},
				},
			},
		},
	}

	result := FromFantasyMessages(msgs)
	require.Len(t, result, 1)
	require.Equal(t, Tool, result[0].Role)

	toolResults := result[0].ToolResults()
	require.Len(t, toolResults, 1)
	require.Equal(t, "call-1", toolResults[0].ToolCallID)
	require.Equal(t, "output", toolResults[0].Content)
}

func TestFromFantasyMessages_ToolResultError(t *testing.T) {
	t.Parallel()

	msgs := []fantasy.Message{
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: "call-1",
					Output:     fantasy.ToolResultOutputContentError{Error: assertAnError("tool failed")},
				},
			},
		},
	}

	result := FromFantasyMessages(msgs)
	require.Len(t, result, 1)

	toolResults := result[0].ToolResults()
	require.Len(t, toolResults, 1)
	require.True(t, toolResults[0].IsError)
	require.Equal(t, "tool failed", toolResults[0].Content)
}

func TestFromFantasyMessages_EmptyMessages(t *testing.T) {
	t.Parallel()

	result := FromFantasyMessages(nil)
	require.Empty(t, result)

	result = FromFantasyMessages([]fantasy.Message{})
	require.Empty(t, result)
}

func TestFromFantasyMessages_RoundTrip(t *testing.T) {
	t.Parallel()

	original := []Message{
		{
			Role: User,
			Parts: []ContentPart{
				TextContent{Text: "hello"},
			},
		},
		{
			Role: Assistant,
			Parts: []ContentPart{
				TextContent{Text: "hi"},
				ToolCall{
					ID:    "call-1",
					Name:  "bash",
					Input: `{"command": "ls"}`,
				},
			},
		},
		{
			Role: Tool,
			Parts: []ContentPart{
				ToolResult{
					ToolCallID: "call-1",
					Name:       "bash",
					Content:    "file1.txt\nfile2.txt",
				},
			},
		},
	}

	// Convert to fantasy and back
	fantasyMsgs := make([]fantasy.Message, 0, len(original))
	for _, m := range original {
		fantasyMsgs = append(fantasyMsgs, m.ToAIMessage()...)
	}

	result := FromFantasyMessages(fantasyMsgs)

	require.Len(t, result, 3)
	require.Equal(t, User, result[0].Role)
	require.Equal(t, "hello", result[0].Content().Text)

	require.Equal(t, Assistant, result[1].Role)
	require.Equal(t, "hi", result[1].Content().Text)
	require.Len(t, result[1].ToolCalls(), 1)

	require.Equal(t, Tool, result[2].Role)
	require.Len(t, result[2].ToolResults(), 1)
}

func assertAnError(msg string) error {
	return &testError{msg: msg}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
