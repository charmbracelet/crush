package mcp

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stretchr/testify/require"
)

func TestEnsureRawBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []byte
		wantData []byte
	}{
		{
			name:     "already base64 encoded",
			input:    []byte("SGVsbG8gV29ybGQh"), // "Hello World!" in base64
			wantData: []byte("Hello World!"),
		},
		{
			name:     "raw binary data (PNG header)",
			input:    []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			wantData: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
		},
		{
			name:     "raw binary with high bytes",
			input:    []byte{0xFF, 0xD8, 0xFF, 0xE0}, // JPEG header
			wantData: []byte{0xFF, 0xD8, 0xFF, 0xE0},
		},
		{
			name:     "empty data",
			input:    []byte{},
			wantData: []byte{},
		},
		{
			name:     "base64 with padding",
			input:    []byte("YQ=="), // "a" in base64
			wantData: []byte("a"),
		},
		{
			name:     "base64 without padding",
			input:    []byte("YQ"),
			wantData: []byte("a"),
		},
		{
			name:     "base64 with whitespace",
			input:    []byte("U0dWc2JHOGdWMjl5YkdRaA==\n"),
			wantData: []byte("SGVsbG8gV29ybGQh"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ensureRawBytes(tt.input)
			require.Equal(t, tt.wantData, result)

			if len(result) > 0 && !bytes.Equal(result, tt.input) {
				reEncoded := base64.StdEncoding.EncodeToString(result)
				_, err := base64.StdEncoding.DecodeString(reEncoded)
				require.NoError(t, err, "re-encoded result should be valid base64")
			}
		})
	}
}

func TestFilterTools(t *testing.T) {
	t.Parallel()

	tools := []*Tool{
		{Name: "tool_a"},
		{Name: "tool_b"},
		{Name: "tool_c"},
	}

	t.Run("no filters returns all tools", func(t *testing.T) {
		t.Parallel()
		result := filterTools(config.MCPConfig{}, tools)
		require.Len(t, result, 3)
	})

	t.Run("disabled tools filters deny list", func(t *testing.T) {
		t.Parallel()
		result := filterTools(config.MCPConfig{DisabledTools: []string{"tool_a"}}, tools)
		require.Len(t, result, 2)
		require.Equal(t, "tool_b", result[0].Name)
		require.Equal(t, "tool_c", result[1].Name)
	})

	t.Run("enabled tools acts as allow list", func(t *testing.T) {
		t.Parallel()
		result := filterTools(config.MCPConfig{EnabledTools: []string{"tool_b"}}, tools)
		require.Len(t, result, 1)
		require.Equal(t, "tool_b", result[0].Name)
	})

	t.Run("enabled and disabled both apply", func(t *testing.T) {
		t.Parallel()
		result := filterTools(config.MCPConfig{
			EnabledTools:  []string{"tool_a", "tool_b"},
			DisabledTools: []string{"tool_b"},
		}, tools)
		require.Len(t, result, 1)
		require.Equal(t, "tool_a", result[0].Name)
	})

	t.Run("enabled with non-existent tool returns empty", func(t *testing.T) {
		t.Parallel()
		result := filterTools(config.MCPConfig{EnabledTools: []string{"non_existent"}}, tools)
		require.Len(t, result, 0)
	})
}

func TestContentToResult(t *testing.T) {
	t.Parallel()

	t.Run("text and image blocks keep their existing shape", func(t *testing.T) {
		t.Parallel()

		result := contentToResult([]mcp.Content{
			&mcp.TextContent{Text: "first"},
			&mcp.TextContent{Text: "second"},
			&mcp.ImageContent{Data: []byte{0x89, 0x50, 0x4E, 0x47}, MIMEType: "image/png"},
		})
		require.Equal(t, "image", result.Type)
		require.Equal(t, "first\nsecond", result.Content)
		require.Equal(t, "image/png", result.MediaType)
		require.Equal(t, []byte{0x89, 0x50, 0x4E, 0x47}, result.Data)
	})

	// Regression test for #3846: an embedded resource fell through to the
	// `%v` default and reached the model as "&{0x1399715f00a0 map[] <nil>}".
	t.Run("embedded text resource contributes its text", func(t *testing.T) {
		t.Parallel()

		result := contentToResult([]mcp.Content{
			&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{
				URI:  "qmd://notes/brief.md",
				Text: "# Brief\nthe body",
			}},
		})
		require.Equal(t, "text", result.Type)
		require.Equal(t, "# Brief\nthe body", result.Content)
	})

	t.Run("embedded text resource joins surrounding text", func(t *testing.T) {
		t.Parallel()

		result := contentToResult([]mcp.Content{
			&mcp.TextContent{Text: "before"},
			&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{URI: "file:///a", Text: "body"}},
			&mcp.TextContent{Text: "after"},
		})
		require.Equal(t, "before\nbody\nafter", result.Content)
	})

	t.Run("image resource is returned as image data", func(t *testing.T) {
		t.Parallel()

		result := contentToResult([]mcp.Content{
			&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{
				URI:      "file:///shot.png",
				MIMEType: "image/png",
				Blob:     []byte{0x89, 0x50, 0x4E, 0x47},
			}},
		})
		require.Equal(t, "image", result.Type)
		require.Equal(t, "image/png", result.MediaType)
		require.Equal(t, []byte{0x89, 0x50, 0x4E, 0x47}, result.Data)
	})

	t.Run("audio resource is returned as media data", func(t *testing.T) {
		t.Parallel()

		result := contentToResult([]mcp.Content{
			&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{
				URI:      "file:///clip.mp3",
				MIMEType: "audio/mpeg",
				Blob:     []byte{0xFF, 0xD8, 0xFF, 0xE0},
			}},
		})
		require.Equal(t, "media", result.Type)
		require.Equal(t, "audio/mpeg", result.MediaType)
		require.Equal(t, []byte{0xFF, 0xD8, 0xFF, 0xE0}, result.Data)
	})

	t.Run("other blob resources describe what was left out", func(t *testing.T) {
		t.Parallel()

		result := contentToResult([]mcp.Content{
			&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{
				URI:      "file:///doc.pdf",
				MIMEType: "application/pdf",
				Blob:     []byte("not really a pdf"),
			}},
		})
		require.Equal(t, "text", result.Type)
		require.Equal(t, "[resource: file:///doc.pdf (application/pdf, 16 bytes)]", result.Content)
	})

	t.Run("empty resource is described, not stringified", func(t *testing.T) {
		t.Parallel()

		result := contentToResult([]mcp.Content{&mcp.EmbeddedResource{}})
		require.Equal(t, "text", result.Type)
		require.Equal(t, "[empty resource]", result.Content)

		result = contentToResult([]mcp.Content{
			&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{URI: "file:///empty.md"}},
		})
		require.Equal(t, "[resource: file:///empty.md, 0 bytes]", result.Content)
	})

	t.Run("resource link names its target", func(t *testing.T) {
		t.Parallel()

		result := contentToResult([]mcp.Content{
			&mcp.ResourceLink{URI: "file:///data.csv", Name: "data"},
		})
		require.Equal(t, "text", result.Type)
		require.Equal(t, "[resource link: data (file:///data.csv)]", result.Content)

		result = contentToResult([]mcp.Content{
			&mcp.ResourceLink{URI: "file:///data.csv"},
		})
		require.Equal(t, "[resource link: file:///data.csv]", result.Content)
	})
}
