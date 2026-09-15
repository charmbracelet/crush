package completions

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
)

func TestFilterPrefersExactBasenameStem(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetItems([]FileCompletionValue{
		{Path: "internal/ui/chat/search.go"},
		{Path: "internal/ui/chat/user.go"},
	}, nil, nil)

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
	}, nil, nil)

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
	}, nil, nil)

	c.Filter("chat")

	filtered := c.filtered
	require.NotEmpty(t, filtered)
	first, ok := filtered[0].(*CompletionItem)
	require.True(t, ok)
	require.Equal(t, "internal/ui/chat/mcp.go", first.Text())
}

// TestSetItems_SubagentsAppearsInList verifies that a subagent passed via the
// third argument to SetItems is represented in the filtered list as an item
// whose Text() equals the subagent name.
func TestSetItems_SubagentsAppearsInList(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetItems(nil, nil, []SubagentCompletionValue{
		{Name: "code-reviewer", Description: "reviews code"},
	})

	var found bool
	for _, item := range c.filtered {
		ci, ok := item.(*CompletionItem)
		if !ok {
			continue
		}
		if ci.Text() == "code-reviewer" {
			found = true
			break
		}
	}
	require.True(t, found, "expected to find a completion item with text %q", "code-reviewer")
}

// TestSetItems_SubagentAndFilesCoexist verifies that when SetItems is called
// with both file and subagent entries, items for both appear in the filtered
// list.
func TestSetItems_SubagentAndFilesCoexist(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetItems(
		[]FileCompletionValue{{Path: "cmd/main.go"}},
		nil,
		[]SubagentCompletionValue{{Name: "tester", Description: "writes tests"}},
	)

	texts := make([]string, 0, len(c.filtered))
	for _, item := range c.filtered {
		ci, ok := item.(*CompletionItem)
		if !ok {
			continue
		}
		texts = append(texts, ci.Text())
	}

	require.Contains(t, texts, "cmd/main.go", "file item must appear in filtered list")
	require.Contains(t, texts, "tester", "subagent item must appear in filtered list")
}

// TestSetItems_NilSubagents_NoError verifies that calling SetItems with a nil
// subagents slice does not panic and still populates file items normally.
func TestSetItems_NilSubagents_NoError(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	require.NotPanics(t, func() {
		c.SetItems([]FileCompletionValue{{Path: "internal/foo.go"}}, nil, nil)
	})

	require.NotEmpty(t, c.filtered, "file items must still be populated when subagents is nil")

	var found bool
	for _, item := range c.filtered {
		ci, ok := item.(*CompletionItem)
		if !ok {
			continue
		}
		if ci.Text() == "internal/foo.go" {
			found = true
			break
		}
	}
	require.True(t, found, "expected to find file item %q when subagents is nil", "internal/foo.go")
}

// TestSetItems_PreservesSubagentOrder verifies that multiple subagents appear in
// the filtered list in the order they were passed to SetItems. This pins the
// ordering contract so frontend display matches input order.
func TestSetItems_PreservesSubagentOrder(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetItems(nil, nil, []SubagentCompletionValue{
		{Name: "zeta"},
		{Name: "alpha"},
		{Name: "mu"},
	})

	require.Len(t, c.filtered, 3)

	got := make([]string, 0, 3)
	for _, item := range c.filtered {
		ci, ok := item.(*CompletionItem)
		require.True(t, ok)
		got = append(got, ci.Text())
	}
	require.Equal(t, []string{"zeta", "alpha", "mu"}, got)
}
