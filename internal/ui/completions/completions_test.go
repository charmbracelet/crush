package completions

import (
	"io"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/sahilm/fuzzy"
	"github.com/stretchr/testify/require"
)

func TestFilterPrefersExactBasenameStem(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetItems([]FileCompletionValue{
		{Path: "internal/ui/chat/search.go"},
		{Path: "internal/ui/chat/user.go"},
	}, nil)

	c.Filter("user")

	filtered := c.filtered
	require.NotEmpty(t, filtered)
	first, ok := filtered[0].(*CompletionItem)
	require.True(t, ok)
	require.Equal(t, "internal/ui/chat/user.go", first.Text())
	require.NotEmpty(t, first.match.MatchedIndexes)
}

func TestFilterPrefersBasenamePrefix(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetItems([]FileCompletionValue{
		{Path: "internal/ui/chat/mcp.go"},
		{Path: "internal/ui/model/chat.go"},
	}, nil)

	c.Filter("chat.g")

	filtered := c.filtered
	require.NotEmpty(t, filtered)
	first, ok := filtered[0].(*CompletionItem)
	require.True(t, ok)
	require.Equal(t, "internal/ui/model/chat.go", first.Text())
	require.NotEmpty(t, first.match.MatchedIndexes)
}

func TestNamePriorityTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		query    string
		wantTier int
	}{
		{
			name:     "exact stem",
			path:     "internal/ui/chat/user.go",
			query:    "user",
			wantTier: tierExactName,
		},
		{
			name:     "basename prefix",
			path:     "internal/ui/model/chat.go",
			query:    "chat.g",
			wantTier: tierPrefixName,
		},
		{
			name:     "path segment exact",
			path:     "internal/ui/chat/mcp.go",
			query:    "chat",
			wantTier: tierPathSegment,
		},
		{
			name:     "fallback",
			path:     "internal/ui/chat/search.go",
			query:    "user",
			wantTier: tierFallback,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := namePriorityTier(tt.path, tt.query)
			require.Equal(t, tt.wantTier, got)
		})
	}
}

func TestFilterPrefersPathSegmentExact(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetItems([]FileCompletionValue{
		{Path: "internal/ui/model/xychat.go"},
		{Path: "internal/ui/chat/mcp.go"},
	}, nil)

	c.Filter("chat")

	filtered := c.filtered
	require.NotEmpty(t, filtered)
	first, ok := filtered[0].(*CompletionItem)
	require.True(t, ok)
	require.Equal(t, "internal/ui/chat/mcp.go", first.Text())
}

// forceTrueColor pins lipgloss's color profile so color assertions are
// deterministic regardless of the test environment. Not parallel: it
// mutates package state.
func forceTrueColor(t *testing.T) {
	t.Helper()
	old := lipgloss.Writer
	lipgloss.Writer = colorprofile.NewWriter(io.Discard, []string{"COLORTERM=truecolor"})
	t.Cleanup(func() { lipgloss.Writer = old })
}

func TestRenderItem_FocusedMatchUsesFocusedForeground(t *testing.T) {
	forceTrueColor(t)

	// Distinct colors for every role so the rendered SGR sequences are
	// unambiguous: focused fg "38;2;51;51;51", match fg "38;2;85;85;85".
	normal := lipgloss.NewStyle().Foreground(lipgloss.Color("#111111")).Background(lipgloss.Color("#222222"))
	focused := lipgloss.NewStyle().Foreground(lipgloss.Color("#333333")).Background(lipgloss.Color("#444444"))
	match := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Underline(true)

	item := NewCompletionItem("pricing_plans.go",
		FileCompletionValue{Path: "pricing_plans.go"}, normal, focused, match)
	item.SetMatch(fuzzy.Match{Str: "pricing_plans.go", MatchedIndexes: []int{0, 1, 2}})

	// On the focused row the match highlight must not keep the match
	// foreground; themes where it equals the focused background (e.g. a
	// monochrome palette) would render matched text invisibly.
	item.SetFocused(true)
	focusedOut := item.Render(30)
	require.Contains(t, focusedOut, "38;2;51;51;51")
	require.NotContains(t, focusedOut, "38;2;85;85;85")

	// On a blurred row the match foreground applies as-is.
	item.SetFocused(false)
	blurredOut := item.Render(30)
	require.Contains(t, blurredOut, "38;2;85;85;85")
}
