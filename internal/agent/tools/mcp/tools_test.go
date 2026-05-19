package mcp

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
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
