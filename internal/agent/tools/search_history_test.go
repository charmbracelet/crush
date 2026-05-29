package tools

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnippetAround(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text     string
		idx      int
		query    int
		want     string
		wantHead string // optional substring at start
		wantTail string // optional substring at end
	}{
		{
			name: "match in middle gets ellipses both sides",
			text: strings.Repeat("a", 200) + "MATCH" + strings.Repeat("b", 200),
			idx:  200,
			query: 5,
			wantHead: "…",
			wantTail: "…",
		},
		{
			name: "match at start has no leading ellipsis",
			text: "MATCH" + strings.Repeat("b", 200),
			idx:  0,
			query: 5,
			wantHead: "MATCH",
			wantTail: "…",
		},
		{
			name: "match at end has no trailing ellipsis",
			text: strings.Repeat("a", 200) + "MATCH",
			idx:  200,
			query: 5,
			wantHead: "…",
			wantTail: "MATCH",
		},
		{
			name:  "newlines collapsed to spaces",
			text:  "line1\nline2 MATCH line3\nline4",
			idx:   12,
			query: 5,
			want:  "line1 line2 MATCH line3 line4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := snippetAround(tt.text, tt.idx, tt.query)
			if tt.want != "" {
				require.Equal(t, tt.want, got)
			}
			if tt.wantHead != "" {
				require.True(t, strings.HasPrefix(got, tt.wantHead), "want prefix %q in %q", tt.wantHead, got)
			}
			if tt.wantTail != "" {
				require.True(t, strings.HasSuffix(got, tt.wantTail), "want suffix %q in %q", tt.wantTail, got)
			}
		})
	}
}

func TestShortID(t *testing.T) {
	t.Parallel()
	require.Equal(t, "01234567", shortID("0123456789abcdef"))
	require.Equal(t, "short", shortID("short"))
	require.Equal(t, "12345678", shortID("12345678"))
}

func TestFormatHistoryHits(t *testing.T) {
	t.Parallel()
	hits := []SearchHistoryHit{
		{
			SessionID: "abc12345-rest", SessionTitle: "deploy chat",
			MessageID: "m1", Role: "user", CreatedAt: "2026-01-02T03:04:05Z",
			Snippet: "needle",
		},
		{
			SessionID: "xyz98765-rest", SessionTitle: "",
			MessageID: "m2", Role: "assistant", CreatedAt: "2026-01-03T03:04:05Z",
			Snippet: "needle",
		},
	}
	out := formatHistoryHits("needle", hits)
	require.Contains(t, out, "Found 2 match(es) for \"needle\":")
	require.Contains(t, out, "deploy chat")
	require.Contains(t, out, "(untitled)")
	require.Contains(t, out, "session abc12345")
	require.Contains(t, out, "session xyz98765")
}
