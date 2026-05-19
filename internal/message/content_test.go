package message

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"charm.land/fantasy"
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

func TestToAIMessage_CorruptedMediaData(t *testing.T) {
	t.Parallel()

	msg := &Message{
		Role: Tool,
		Parts: []ContentPart{
			ToolResult{
				ToolCallID: "call_123",
				Name:       "screenshot",
				Content:    "Loaded image/png content",
				Data:       "abc\x80def",
				MIMEType:   "image/png",
			},
		},
	}

	messages := msg.ToAIMessage()
	require.Len(t, messages, 1)
	require.Len(t, messages[0].Content, 1)

	part, ok := messages[0].Content[0].(fantasy.ToolResultPart)
	require.True(t, ok)

	require.Equal(t, "call_123", part.ToolCallID)

	textContent, ok := part.Output.(fantasy.ToolResultOutputContentText)
	require.True(t, ok, "corrupted media should be downgraded to text")
	require.Equal(t, mediaLoadFailedPlaceholder, textContent.Text)
}

func TestToAIMessage_ValidMediaData(t *testing.T) {
	t.Parallel()

	validBase64 := base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4E, 0x47})

	msg := &Message{
		Role: Tool,
		Parts: []ContentPart{
			ToolResult{
				ToolCallID: "call_456",
				Name:       "screenshot",
				Content:    "Loaded image/png content",
				Data:       validBase64,
				MIMEType:   "image/png",
			},
		},
	}

	messages := msg.ToAIMessage()
	require.Len(t, messages, 1)
	require.Len(t, messages[0].Content, 1)

	part, ok := messages[0].Content[0].(fantasy.ToolResultPart)
	require.True(t, ok)

	require.Equal(t, "call_456", part.ToolCallID)

	mediaContent, ok := part.Output.(fantasy.ToolResultOutputContentMedia)
	require.True(t, ok, "valid media should remain as media")
	require.Equal(t, validBase64, mediaContent.Data)
	require.Equal(t, "image/png", mediaContent.MediaType)
}

func TestToAIMessage_ASCIIButInvalidBase64(t *testing.T) {
	t.Parallel()

	msg := &Message{
		Role: Tool,
		Parts: []ContentPart{
			ToolResult{
				ToolCallID: "call_789",
				Name:       "screenshot",
				Content:    "Loaded image/png content",
				Data:       "not-valid-base64!!!",
				MIMEType:   "image/png",
			},
		},
	}

	messages := msg.ToAIMessage()
	require.Len(t, messages, 1)
	require.Len(t, messages[0].Content, 1)

	part, ok := messages[0].Content[0].(fantasy.ToolResultPart)
	require.True(t, ok)

	require.Equal(t, "call_789", part.ToolCallID)

	textContent, ok := part.Output.(fantasy.ToolResultOutputContentText)
	require.True(t, ok, "ASCII but invalid base64 should be downgraded to text")
	require.Equal(t, mediaLoadFailedPlaceholder, textContent.Text)
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

// TestAppendContent_OnlyAppendsToFirstMatch pins the defensive
// single-match behavior of AppendContent: a delta must extend exactly
// one TextContent part, even when Parts contains more than one. Without
// the `return` after a match, the loop continues and appends `delta`
// to every TextContent in Parts — quietly multiplying the visible
// response. The sibling helpers in this file (AppendThoughtSignature,
// SetReasoningResponsesData, FinishThinking) all single-match; this
// test guards that AppendContent and AppendReasoningContent stay in
// the same family.
func TestAppendContent_OnlyAppendsToFirstMatch(t *testing.T) {
	t.Parallel()

	m := &Message{
		Parts: []ContentPart{
			TextContent{Text: "first"},
			TextContent{Text: "second"},
		},
	}

	m.AppendContent(" delta")

	require.Len(t, m.Parts, 2)
	require.Equal(t, "first delta", m.Parts[0].(TextContent).Text)
	require.Equal(t, "second", m.Parts[1].(TextContent).Text,
		"second TextContent part must be untouched; AppendContent multi-matched")
}

func TestAppendContent_CreatesPartWhenAbsent(t *testing.T) {
	t.Parallel()

	m := &Message{Parts: []ContentPart{ToolCall{ID: "tc1"}}}
	m.AppendContent("hello")

	require.Len(t, m.Parts, 2)
	tc, ok := m.Parts[1].(TextContent)
	require.True(t, ok, "expected TextContent appended after ToolCall")
	require.Equal(t, "hello", tc.Text)
}

func TestAppendReasoningContent_OnlyAppendsToFirstMatch(t *testing.T) {
	t.Parallel()

	m := &Message{
		Parts: []ContentPart{
			ReasoningContent{Thinking: "a", StartedAt: 1},
			ReasoningContent{Thinking: "b", StartedAt: 2},
		},
	}

	m.AppendReasoningContent(" delta")

	require.Len(t, m.Parts, 2)
	require.Equal(t, "a delta", m.Parts[0].(ReasoningContent).Thinking)
	require.Equal(t, "b", m.Parts[1].(ReasoningContent).Thinking,
		"second ReasoningContent part must be untouched")
}
