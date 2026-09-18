package mcp

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// A browser tool answered a scroll with an image block carrying no bytes,
// Anthropic refused the whole request for it, and every later turn replaying
// the transcript failed the same way. Emptiness is judged after decoding,
// because a blank string is just as unusable as an absent one.
func TestUnusableMediaIsNotSentAsMedia(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		content mcp.Content
	}{
		{"empty image", &mcp.ImageContent{Data: []byte{}, MIMEType: "image/png"}},
		{"blank image", &mcp.ImageContent{Data: []byte("  "), MIMEType: "image/png"}},
		{"empty audio", &mcp.AudioContent{Data: []byte{}, MIMEType: "audio/wav"}},
		{"blank audio", &mcp.AudioContent{Data: []byte("\n"), MIMEType: "audio/wav"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := toolResultFromCall(&mcp.CallToolResult{Content: []mcp.Content{tt.content}})
			require.Equal(t, "text", got.Type, "an unusable payload is not media")
			require.Empty(t, got.Data)
			require.Contains(t, got.Content, EmptyMediaNote, "the model has to be told the media was unusable")
		})
	}
}

// Dropping the picture must not also drop what the server said about it.
func TestEmptyImageKeepsAccompanyingText(t *testing.T) {
	t.Parallel()

	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "scrolled to the bottom"},
			&mcp.ImageContent{Data: []byte{}, MIMEType: "image/png"},
		},
	}

	got := toolResultFromCall(result)
	require.Equal(t, "text", got.Type)
	require.Contains(t, got.Content, "scrolled to the bottom")
	require.Contains(t, got.Content, EmptyMediaNote)
}

// A usable picture must not be lost behind an empty block that arrived first.
func TestLaterNonEmptyImageWins(t *testing.T) {
	t.Parallel()

	real := []byte{0x89, 'P', 'N', 'G'}
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ImageContent{Data: []byte{}, MIMEType: "image/gif"},
			&mcp.ImageContent{Data: real, MIMEType: "image/png"},
		},
	}

	got := toolResultFromCall(result)
	require.Equal(t, "image", got.Type)
	require.Equal(t, real, got.Data)
	require.Equal(t, "image/png", got.MediaType, "the media type follows the image that was kept")
	require.NotContains(t, got.Content, EmptyMediaNote, "a picture did get through")
}

// Guards against closing the empty case by breaking the ordinary one.
func TestGoodImageStillPasses(t *testing.T) {
	t.Parallel()

	real := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a}
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ImageContent{Data: real, MIMEType: "image/png"},
		},
	}

	got := toolResultFromCall(result)
	require.Equal(t, "image", got.Type)
	require.Equal(t, real, got.Data)
}
