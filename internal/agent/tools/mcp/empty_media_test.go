package mcp

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// A blank payload decodes to nothing, so it must reach the agent as empty
// media rather than as whitespace posing as image bytes.
func TestBlankMediaDecodesToEmpty(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		content mcp.Content
		kind    string
	}{
		{"empty image", &mcp.ImageContent{Data: []byte{}, MIMEType: "image/png"}, "image"},
		{"blank image", &mcp.ImageContent{Data: []byte("  "), MIMEType: "image/png"}, "image"},
		{"nil image", &mcp.ImageContent{MIMEType: "image/png"}, "image"},
		{"empty audio", &mcp.AudioContent{Data: []byte{}, MIMEType: "audio/wav"}, "media"},
		{"blank audio", &mcp.AudioContent{Data: []byte("\n"), MIMEType: "audio/wav"}, "media"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := toolResultFromCall(&mcp.CallToolResult{Content: []mcp.Content{tt.content}})
			require.Equal(t, tt.kind, got.Type)
			require.Empty(t, got.Data)
		})
	}
}

func TestEmptyImageKeepsAccompanyingText(t *testing.T) {
	t.Parallel()

	got := toolResultFromCall(&mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "scrolled to the bottom"},
			&mcp.ImageContent{Data: []byte{}, MIMEType: "image/png"},
		},
	})
	require.Empty(t, got.Data)
	require.Equal(t, "scrolled to the bottom", got.Content)
}

func TestLaterNonEmptyImageWins(t *testing.T) {
	t.Parallel()

	real := []byte{0x89, 'P', 'N', 'G'}
	got := toolResultFromCall(&mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ImageContent{Data: []byte{}, MIMEType: "image/gif"},
			&mcp.ImageContent{Data: real, MIMEType: "image/png"},
		},
	})
	require.Equal(t, "image", got.Type)
	require.Equal(t, real, got.Data)
	require.Equal(t, "image/png", got.MediaType)
}

func TestEmptyImageDoesNotHideRealAudio(t *testing.T) {
	t.Parallel()

	wav := []byte{0xff, 0xf3, 0x44, 0xc4}
	got := toolResultFromCall(&mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ImageContent{Data: []byte{}, MIMEType: "image/png"},
			&mcp.AudioContent{Data: wav, MIMEType: "audio/wav"},
		},
	})
	require.Equal(t, "media", got.Type)
	require.Equal(t, wav, got.Data)
}

func TestGoodImageStillPasses(t *testing.T) {
	t.Parallel()

	real := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a}
	got := toolResultFromCall(&mcp.CallToolResult{
		Content: []mcp.Content{&mcp.ImageContent{Data: real, MIMEType: "image/png"}},
	})
	require.Equal(t, "image", got.Type)
	require.Equal(t, real, got.Data)
}
