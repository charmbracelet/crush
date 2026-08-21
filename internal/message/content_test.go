package message

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/google"
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

// TestToAIMessage_GoogleThoughtSignaturesPerToolCall verifies that each tool
// call's Google thought signature is replayed in its own ReasoningPart placed
// immediately before that tool call, never concatenated. Concatenation or
// misplacement is what triggers Gemini's "Corrupted thought signature" error.
func TestToAIMessage_GoogleThoughtSignaturesPerToolCall(t *testing.T) {
	t.Parallel()

	msg := &Message{
		Role: Assistant,
		Parts: []ContentPart{
			ReasoningContent{Thinking: "let me think", FinishedAt: 1},
			ToolCall{ID: "call_1", Name: "view", Input: "{}", Finished: true, ThoughtSignature: "SIG1"},
			ToolCall{ID: "call_2", Name: "ls", Input: "{}", Finished: true, ThoughtSignature: "SIG2"},
		},
	}

	messages := msg.ToAIMessage()
	require.Len(t, messages, 1)
	content := messages[0].Content
	// reasoning(thinking), reasoning(SIG1), toolcall_1, reasoning(SIG2), toolcall_2
	require.Len(t, content, 5)

	// [0] thinking reasoning, no google signature attached.
	r0, ok := content[0].(fantasy.ReasoningPart)
	require.True(t, ok)
	require.Equal(t, "let me think", r0.Text)
	require.Nil(t, r0.ProviderOptions[google.Name])

	assertGoogleSig := func(i int, sig, toolID string) {
		t.Helper()
		rp, ok := content[i].(fantasy.ReasoningPart)
		require.True(t, ok, "part %d must be a ReasoningPart", i)
		meta, ok := rp.ProviderOptions[google.Name].(*google.ReasoningMetadata)
		require.True(t, ok, "part %d must carry google ReasoningMetadata", i)
		require.Equal(t, sig, meta.Signature)
		require.Equal(t, toolID, meta.ToolID)
	}

	assertGoogleSig(1, "SIG1", "call_1")
	tc1, ok := content[2].(fantasy.ToolCallPart)
	require.True(t, ok)
	require.Equal(t, "call_1", tc1.ToolCallID)

	assertGoogleSig(3, "SIG2", "call_2")
	tc2, ok := content[4].(fantasy.ToolCallPart)
	require.True(t, ok)
	require.Equal(t, "call_2", tc2.ToolCallID)
}

// TestToAIMessage_GoogleTextAnswerSignature verifies the final-answer thought
// signature (no tool ID) is replayed on a ReasoningPart immediately before the
// text part.
func TestToAIMessage_GoogleTextAnswerSignature(t *testing.T) {
	t.Parallel()

	msg := &Message{
		Role: Assistant,
		Parts: []ContentPart{
			ReasoningContent{ThoughtSignature: "TEXTSIG", FinishedAt: 1},
			TextContent{Text: "final answer"},
		},
	}

	messages := msg.ToAIMessage()
	require.Len(t, messages, 1)
	content := messages[0].Content
	require.Len(t, content, 2)

	rp, ok := content[0].(fantasy.ReasoningPart)
	require.True(t, ok)
	meta, ok := rp.ProviderOptions[google.Name].(*google.ReasoningMetadata)
	require.True(t, ok)
	require.Equal(t, "TEXTSIG", meta.Signature)
	require.Empty(t, meta.ToolID)

	tp, ok := content[1].(fantasy.TextPart)
	require.True(t, ok)
	require.Equal(t, "final answer", tp.Text)
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

func TestResetStreamedContent(t *testing.T) {
	t.Parallel()

	msg := &Message{}
	msg.AddImageURL("https://example.com/img.png", "high")
	msg.AppendContent("partial answer")
	msg.AppendReasoningContent("thinking...")
	msg.AddToolCall(ToolCall{ID: "1", Name: "bash"})
	msg.AddToolResult(ToolResult{ToolCallID: "1", Content: "output"})
	msg.AddFinish(FinishReasonError, "boom", "stream died")

	msg.ResetStreamedContent()

	// Streamed parts are gone.
	require.Empty(t, msg.Content().Text, "text should be cleared")
	require.Empty(t, msg.ReasoningContent().Thinking, "reasoning should be cleared")
	require.Empty(t, msg.ToolCalls(), "tool calls should be cleared")
	require.Nil(t, msg.FinishPart(), "finish should be cleared")

	// Non-streamed parts survive.
	require.Len(t, msg.ImageURLContent(), 1, "image should survive")
	require.Len(t, msg.ToolResults(), 1, "tool results should survive")
}

func TestResetStreamedContentEmpty(t *testing.T) {
	t.Parallel()

	// Reset on an empty message is a no-op and must not panic.
	msg := &Message{}
	msg.ResetStreamedContent()
	require.Empty(t, msg.Parts)
}
