package mcp

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// TestEmptyImageIsNotSentAsMedia covers the report that took a whole session
// down: a browser tool answered a scroll with an image block carrying no
// bytes. An empty payload decodes to a slice that is present but zero
// length, so it passed the nil check and went out as a media block with
// nothing in it. Anthropic refuses the entire request for that — "image
// cannot be empty" — so every later turn replaying the transcript failed the
// same way and the session could not be used again.
func TestEmptyImageIsNotSentAsMedia(t *testing.T) {
	t.Parallel()

	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ImageContent{Data: []byte{}, MIMEType: "image/png"},
		},
	}

	got := toolResultFromCall(result)
	require.Equal(t, "text", got.Type, "an empty payload is not an image")
	require.Empty(t, got.Data)
	require.NotEmpty(t, got.Content, "the model has to be told the media was unusable")
}

// TestEmptyAudioIsNotSentAsMedia is the same hole on the audio branch.
func TestEmptyAudioIsNotSentAsMedia(t *testing.T) {
	t.Parallel()

	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.AudioContent{Data: []byte{}, MIMEType: "audio/wav"},
		},
	}

	got := toolResultFromCall(result)
	require.Equal(t, "text", got.Type)
	require.Empty(t, got.Data)
}

// TestEmptyImageKeepsAccompanyingText makes sure dropping the picture does
// not also drop what the server said about it.
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
}

// TestLaterNonEmptyImageWins keeps a usable picture from being lost behind
// an empty block that arrived first.
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
}

// TestGoodImageStillPasses guards against fixing the empty case by breaking
// the ordinary one.
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
