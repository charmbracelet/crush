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
