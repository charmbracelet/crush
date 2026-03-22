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
		wantData []byte
	}{
		{
			name:     "already base64 encoded",
			input:    []byte("SGVsbG8gV29ybGQh"),
			wantData: []byte("SGVsbG8gV29ybGQh"),
		},
		{
			name:     "raw binary data",
			input:    []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			wantData: []byte(base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})),
		},
		{
			name:     "raw binary with high bytes",
			input:    []byte{0xFF, 0xD8, 0xFF, 0xE0},
			wantData: []byte(base64.StdEncoding.EncodeToString([]byte{0xFF, 0xD8, 0xFF, 0xE0})),
		},
		{
			name:     "empty data",
			input:    []byte{},
			wantData: []byte{},
		},
		{
			name:     "base64 with padding",
			input:    []byte("YQ=="),
			wantData: []byte("YQ=="),
		},
		{
			name:     "base64 without padding (short, treated as raw)",
			input:    []byte("YQ"),
			wantData: []byte(base64.StdEncoding.EncodeToString([]byte("YQ"))), // "YQ" is too short, encoded as raw
		},
		{
			name:     "base64 with whitespace",
			input:    []byte("U0dWc2JHOGdWMjl5YkdRaA==\n"),
			wantData: []byte("U0dWc2JHOGdWMjl5YkdRaA=="),
		},
		{
			// RawStdEncoding fallback requires len >= 8 and len % 4 == 0
			name:     "base64 without padding (8 chars, valid alignment)",
			input:    []byte("SGVsbG8h"), // "Hello!" in base64 without padding
			wantData: []byte("SGVsbG8h"),
		},
		{
			// "ABCD" is valid StdEncoding base64 (4 chars = 3 bytes decoded)
			name:     "4-char valid base64 (StdEncoding)",
			input:    []byte("ABCD"),
			wantData: []byte("ABCD"), // Already valid base64, returned as-is after normalization
		},
		{
			// 6 chars but not aligned to 4, RawStdEncoding fallback won't trigger
			name:     "6-char ASCII treated as raw (not 4-aligned for raw fallback)",
			input:    []byte("ABCDEF"), // 6 chars, not multiple of 4 for raw fallback
			wantData: []byte(base64.StdEncoding.EncodeToString([]byte("ABCDEF"))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ensureBase64(tt.input)
			require.Equal(t, tt.wantData, result)

			if len(result) > 0 {
				_, err := base64.StdEncoding.DecodeString(string(result))
				if err != nil {
					_, err = base64.RawStdEncoding.DecodeString(string(result))
				}
				require.NoError(t, err)
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
			name:  "valid raw base64 without padding",
			input: []byte("YQ"),
			want:  true,
		},
		{
			name:  "valid base64 with whitespace",
			input: normalizeBase64Input([]byte("U0dWc2JHOGdWMjl5YkdRaA==\n")),
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
			require.Equal(t, tt.want, isValidBase64(tt.input))
		})
	}
}
