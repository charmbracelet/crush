package message

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/openaicodex"
	"github.com/stretchr/testify/require"
)

func makeTestAttachments(n int, contentSize int) []Attachment {
	attachments := make([]Attachment, n)
	content := []byte(strings.Repeat("x", contentSize))
	for i := range n {
		attachments[i] = Attachment{
			FilePath: fmt.Sprintf("/path/to/file%d.txt", i),
			MimeType: "text/plain",
			Content:  content,
		}
	}
	return attachments
}

func BenchmarkPromptWithTextAttachments(b *testing.B) {
	cases := []struct {
		name        string
		numFiles    int
		contentSize int
	}{
		{"1file_100bytes", 1, 100},
		{"5files_1KB", 5, 1024},
		{"10files_10KB", 10, 10 * 1024},
		{"20files_50KB", 20, 50 * 1024},
	}

	for _, tc := range cases {
		attachments := makeTestAttachments(tc.numFiles, tc.contentSize)
		prompt := "Process these files"

		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = PromptWithTextAttachments(prompt, attachments)
			}
		})
	}
}

func TestToAIMessageAssistantProviderPolicySkipsReasoningText(t *testing.T) {
	msg := Message{
		Role:     Assistant,
		Provider: openaicodex.ProviderID,
		Parts: []ContentPart{
			ReasoningContent{Thinking: "internal reasoning that should not be replayed"},
			TextContent{Text: "final answer"},
		},
	}

	out := msg.ToAIMessage()
	require.Len(t, out, 1)
	require.Equal(t, fantasy.MessageRoleAssistant, out[0].Role)
	require.Len(t, out[0].Content, 1)

	textPart, ok := out[0].Content[0].(fantasy.TextPart)
	require.True(t, ok)
	require.Equal(t, "final answer", textPart.Text)
}

func TestToAIMessageAssistantNonCodexKeepsReasoningText(t *testing.T) {
	msg := Message{
		Role:     Assistant,
		Provider: "openai",
		Parts: []ContentPart{
			ReasoningContent{Thinking: "reasoning should be preserved"},
			TextContent{Text: "final answer"},
		},
	}

	out := msg.ToAIMessage()
	require.Len(t, out, 1)
	require.Equal(t, fantasy.MessageRoleAssistant, out[0].Role)
	require.Len(t, out[0].Content, 2)

	textPart, ok := out[0].Content[0].(fantasy.TextPart)
	require.True(t, ok)
	require.Equal(t, "final answer", textPart.Text)

	reasoningPart, ok := out[0].Content[1].(fantasy.ReasoningPart)
	require.True(t, ok)
	require.Equal(t, "reasoning should be preserved", reasoningPart.Text)
}
