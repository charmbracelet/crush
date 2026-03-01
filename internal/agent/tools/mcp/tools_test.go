package mcp

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureBase64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []byte
		wantData []byte // expected output
	}{
		{
			name:     "already base64 encoded",
			input:    []byte("SGVsbG8gV29ybGQh"), // "Hello World!" in base64
			wantData: []byte("SGVsbG8gV29ybGQh"),
		},
		{
			name:     "raw binary data (PNG header)",
			input:    []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			wantData: []byte(base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})),
		},
		{
			name:     "raw binary with high bytes",
			input:    []byte{0xFF, 0xD8, 0xFF, 0xE0}, // JPEG header
			wantData: []byte(base64.StdEncoding.EncodeToString([]byte{0xFF, 0xD8, 0xFF, 0xE0})),
		},
		{
			name:     "empty data",
			input:    []byte{},
			wantData: []byte{},
		},
		{
			name:     "base64 with padding",
			input:    []byte("YQ=="), // "a" in base64
			wantData: []byte("YQ=="),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ensureBase64(tt.input)
			require.Equal(t, tt.wantData, result)

			// Verify the result is valid base64 that can be decoded.
			if len(result) > 0 {
				_, err := base64.StdEncoding.DecodeString(string(result))
				require.NoError(t, err, "result should be valid base64")
			}
		})
	}
}

func TestIsValidBase64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
		want  bool
	}{
		{
			name:  "valid base64",
			input: []byte("SGVsbG8gV29ybGQh"),
			want:  true,
		},
		{
			name:  "valid base64 with padding",
			input: []byte("YQ=="),
			want:  true,
		},
		{
			name:  "raw binary with high bytes",
			input: []byte{0xFF, 0xD8, 0xFF},
			want:  false,
		},
		{
			name:  "empty",
			input: []byte{},
			want:  true,
		},
		{
			name:  "invalid base64 characters",
			input: []byte("SGVsbG8!@#$"),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isValidBase64(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}
