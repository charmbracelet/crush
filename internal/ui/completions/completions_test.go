package completions

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
)

func newTestCompletions() *Completions {
	style := lipgloss.NewStyle()
	return New(style, style, style)
}

func TestFilterPrefersExactBasenameStem(t *testing.T) {
	t.Parallel()

	c := newTestCompletions()
	c.SetItems([]FileCompletionValue{
		{Path: "internal/ui/chat/search.go"},
		{Path: "internal/ui/chat/user.go"},
	}, nil)

	c.Filter("user")

	require.NotEmpty(t, c.filtered)
	first := c.filtered[0]
	require.Equal(t, "internal/ui/chat/user.go", first.Text())
}

func TestFilterPrefersBasenamePrefix(t *testing.T) {
	t.Parallel()

	c := newTestCompletions()
	c.SetItems([]FileCompletionValue{
		{Path: "internal/ui/chat/helper.go"},
		{Path: "internal/ui/model/chat.go"},
	}, nil)

	c.Filter("chat.g")

	require.NotEmpty(t, c.filtered)
	first := c.filtered[0]
	require.Equal(t, "internal/ui/model/chat.go", first.Text())
}

func TestFilterPrefersPathSegmentExact(t *testing.T) {
	t.Parallel()

	c := newTestCompletions()
	c.SetItems([]FileCompletionValue{
		{Path: "internal/ui/model/abc.go"},
		{Path: "internal/ui/chat/mcp.go"},
	}, nil)

	c.Filter("chat")

	require.NotEmpty(t, c.filtered)
	first := c.filtered[0]
	require.Equal(t, "internal/ui/chat/mcp.go", first.Text())
}

func TestNamePriorityTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		query    string
		wantTier int
	}{
		{"exact stem", "internal/ui/chat/user.go", "user", tierExactName},
		{"basename prefix", "internal/ui/model/chat.go", "chat.g", tierPrefixName},
		{"path segment", "internal/ui/chat/mcp.go", "chat", tierPathSegment},
		{"fallback", "internal/ui/chat/other.go", "foo", tierFallback},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.wantTier, namePriorityTier(tt.path, tt.query))
		})
	}
}
