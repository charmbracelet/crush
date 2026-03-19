package completions

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"
	"github.com/stretchr/testify/require"
)

func TestRankPrefersStrongBasenameMatch(t *testing.T) {
	t.Parallel()

	c := &Completions{
		items: []*CompletionItem{
			NewCompletionItem("internal/ui/chat/search.go", FileCompletionValue{Path: "internal/ui/chat/search.go"}, lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle()),
			NewCompletionItem("user.go", FileCompletionValue{Path: "user.go"}, lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle()),
			NewCompletionItem("internal/user_service.go", FileCompletionValue{Path: "internal/user_service.go"}, lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle()),
		},
		paths: []string{"internal/ui/chat/search.go", "user.go", "internal/user_service.go"},
		bases: []string{"search.go", "user.go", "user_service.go"},
	}

	ranked := c.rank(queryContext{query: "user"})
	require.NotEmpty(t, ranked)
	require.Equal(t, "user.go", ranked[0].Text())
}

func TestRankReturnsOriginalOrderForEmptyQuery(t *testing.T) {
	t.Parallel()

	c := &Completions{
		items: []*CompletionItem{
			NewCompletionItem("b/user.go", FileCompletionValue{Path: "b/user.go"}, lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle()),
			NewCompletionItem("a/user.go", FileCompletionValue{Path: "a/user.go"}, lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle()),
		},
		paths: []string{"b/user.go", "a/user.go"},
		bases: []string{"user.go", "user.go"},
	}

	ranked := c.rank(queryContext{query: ""})
	require.Len(t, ranked, 2)
	require.Equal(t, "b/user.go", ranked[0].Text())
	require.Equal(t, "a/user.go", ranked[1].Text())
}

func TestRankPrefersPathMatchesWhenPathHintPresent(t *testing.T) {
	t.Parallel()

	c := &Completions{
		items: []*CompletionItem{
			NewCompletionItem("user.go", FileCompletionValue{Path: "user.go"}, lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle()),
			NewCompletionItem("internal/user.go", FileCompletionValue{Path: "internal/user.go"}, lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle()),
			NewCompletionItem("internal/ui/chat/search.go", FileCompletionValue{Path: "internal/ui/chat/search.go"}, lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle()),
		},
		paths: []string{"user.go", "internal/user.go", "internal/ui/chat/search.go"},
		bases: []string{"user.go", "user.go", "search.go"},
	}

	ranked := c.rank(queryContext{query: "internal/u"})
	require.NotEmpty(t, ranked)
	require.Equal(t, "internal/user.go", ranked[0].Text())
}

func TestRankDotHintPrefersSuffixPathMatch(t *testing.T) {
	t.Parallel()

	c := &Completions{
		items: []*CompletionItem{
			NewCompletionItem("docs/go-guide.md", FileCompletionValue{Path: "docs/go-guide.md"}, lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle()),
			NewCompletionItem("src/user.go", FileCompletionValue{Path: "src/user.go"}, lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle()),
		},
		paths: []string{"docs/go-guide.md", "src/user.go"},
		bases: []string{"go-guide.md", "user.go"},
	}

	ranked := c.rank(queryContext{query: ".go"})
	require.NotEmpty(t, ranked)
	require.Equal(t, "src/user.go", ranked[0].Text())
}

func TestRemapMatchToPath(t *testing.T) {
	t.Parallel()

	match := remapMatchToPath(
		fuzzy.Match{MatchedIndexes: []int{0, 1, 2}},
		"internal/user.go",
	)
	require.Equal(t, []int{9, 10, 11}, match.MatchedIndexes)
}

func TestHasPathHint(t *testing.T) {
	t.Parallel()

	require.True(t, hasPathHint("internal/u"))
	require.True(t, hasPathHint("main.go"))
	require.False(t, hasPathHint("v0.1"))
	require.False(t, hasPathHint("main"))
}
