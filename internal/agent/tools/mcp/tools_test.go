package mcp

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf8"

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

func TestCapToolResult(t *testing.T) {
	t.Parallel()

	t.Run("short content is unchanged", func(t *testing.T) {
		t.Parallel()
		content := "small output"
		require.Equal(t, content, capToolResult(content, 131072))
	})

	t.Run("content at the cap is unchanged", func(t *testing.T) {
		t.Parallel()
		content := strings.Repeat("a", 131072)
		require.Equal(t, content, capToolResult(content, 131072))
	})

	t.Run("oversized content is capped with head, marker, and tail", func(t *testing.T) {
		t.Parallel()
		content := strings.Repeat("a", 131072) + strings.Repeat("b", 131072)
		out := capToolResult(content, 131072)
		require.Contains(t, out, "truncated")
		require.Contains(t, out, "128.0 KB")
		require.True(t, strings.HasPrefix(out, strings.Repeat("a", 131072*3/4)))
		require.True(t, strings.HasSuffix(out, strings.Repeat("b", 131072/4)))
	})

	t.Run("zero cap returns content unchanged", func(t *testing.T) {
		t.Parallel()
		content := "some output"
		require.Equal(t, content, capToolResult(content, 0))
	})

	t.Run("output stays valid UTF-8", func(t *testing.T) {
		t.Parallel()
		content := strings.Repeat("你好世界", 65536)
		out := capToolResult(content, 131072)
		require.True(t, utf8.ValidString(out))
	})
}

func TestFormatBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int
		want  string
	}{
		{name: "bytes", bytes: 512, want: "512 B"},
		{name: "kilobytes", bytes: 96 * 1024, want: "96.0 KB"},
		{name: "megabytes", bytes: 41*1024*1024 + 209715, want: "41.2 MB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, formatBytes(tt.bytes))
		})
	}
}

func TestMaxToolResultBytes(t *testing.T) {
	t.Parallel()

	t.Run("configured server uses its own cap", func(t *testing.T) {
		t.Parallel()
		cfg := config.NewTestStore(&config.Config{MCP: config.MCPs{"myserver": {MaxToolResultBytes: 65536}}})
		require.Equal(t, 65536, maxToolResultBytes(cfg, "myserver"))
	})

	t.Run("unconfigured server uses default", func(t *testing.T) {
		t.Parallel()
		cfg := config.NewTestStore(&config.Config{})
		require.Equal(t, defaultMaxToolResultBytes, maxToolResultBytes(cfg, "myserver"))
	})

	t.Run("server without cap uses default", func(t *testing.T) {
		t.Parallel()
		cfg := config.NewTestStore(&config.Config{MCP: config.MCPs{"myserver": {}}})
		require.Equal(t, defaultMaxToolResultBytes, maxToolResultBytes(cfg, "myserver"))
	})
}
