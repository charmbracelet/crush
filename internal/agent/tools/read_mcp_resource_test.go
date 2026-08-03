package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestA2UIWidthHint(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		uri   string
		width int
		want  string
	}{
		{
			name:  "a2ui run template gets the width",
			uri:   "mcp://cairn/run/vitkuUpp/a2ui",
			width: 114,
			want:  "mcp://cairn/run/vitkuUpp/a2ui?w=114",
		},
		{
			name:  "cairn-scheme alias gets the width",
			uri:   "cairn://run/abc123/a2ui",
			width: 90,
			want:  "cairn://run/abc123/a2ui?w=90",
		},
		{
			name:  "artifact and bundle templates get the width",
			uri:   "mcp://cairn/artifact/PIMfMan7/a2ui",
			width: 80,
			want:  "mcp://cairn/artifact/PIMfMan7/a2ui?w=80",
		},
		{
			name:  "an existing width hint is left alone",
			uri:   "mcp://cairn/run/vitkuUpp/a2ui?w=60",
			width: 114,
			want:  "mcp://cairn/run/vitkuUpp/a2ui?w=60",
		},
		{
			name:  "a non-width query still gets the width appended",
			uri:   "mcp://cairn/run/vitkuUpp/a2ui?theme=dark",
			width: 114,
			want:  "mcp://cairn/run/vitkuUpp/a2ui?theme=dark&w=114",
		},
		{
			name:  "non-a2ui resources are untouched",
			uri:   "mcp://cairn/run/vitkuUpp",
			width: 114,
			want:  "mcp://cairn/run/vitkuUpp",
		},
		{
			name:  "a2ui-looking substring outside the suffix is untouched",
			uri:   "mcp://cairn/artifact/a2ui-notes",
			width: 114,
			want:  "mcp://cairn/artifact/a2ui-notes",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, a2uiWidthHint(tc.uri, tc.width))
		})
	}
}

func TestStripA2UIWidthParam(t *testing.T) {
	t.Parallel()

	require.Equal(t, "cairn://run/x/a2ui", stripA2UIWidthParam("cairn://run/x/a2ui?w=114"))
	require.Equal(t, "cairn://run/x/a2ui?theme=dark", stripA2UIWidthParam("cairn://run/x/a2ui?theme=dark&w=114"))
	require.Equal(t, "cairn://run/x/a2ui", stripA2UIWidthParam("cairn://run/x/a2ui"))
	require.Equal(t, "mcp://cairn/artifact/y", stripA2UIWidthParam("mcp://cairn/artifact/y"))
}

func TestGetContentWidthFromContext(t *testing.T) {
	t.Parallel()

	require.Zero(t, GetContentWidthFromContext(context.Background()))
	ctx := context.WithValue(context.Background(), ContentWidthContextKey, 114)
	require.Equal(t, 114, GetContentWidthFromContext(ctx))
}
