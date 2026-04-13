package mcp

import (
	"bytes"
	"encoding/base64"
	"testing"

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
