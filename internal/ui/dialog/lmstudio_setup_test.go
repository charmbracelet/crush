package dialog

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeLMStudioBaseURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "host only",
			in:   "127.0.0.1:1234",
			want: "http://127.0.0.1:1234/v1",
		},
		{
			name: "root url",
			in:   "http://127.0.0.1:1234",
			want: "http://127.0.0.1:1234/v1",
		},
		{
			name: "already v1",
			in:   "https://example.ts.net/v1",
			want: "https://example.ts.net/v1",
		},
		{
			name: "trailing slash",
			in:   "https://example.ts.net/",
			want: "https://example.ts.net/v1",
		},
		{
			name: "custom path",
			in:   "https://example.ts.net/proxy",
			want: "https://example.ts.net/proxy/v1",
		},
		{
			name: "drops query",
			in:   "https://example.ts.net/v1?x=1",
			want: "https://example.ts.net/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, normalizeLMStudioBaseURL(tt.in))
		})
	}
}
