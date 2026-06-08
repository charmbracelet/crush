package completions

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"
	"github.com/stretchr/testify/require"
)

// TestCompletionItem_MutatorsBumpVersion covers F6 §4.5 for the
// completions popup: SetMatch and SetFocused must bump Version()
// when the observable state changes, and dedupe (no bump) when the
// supplied value is identical to the current state. Without
// dedupe, the steady completions popup would invalidate the
// list-level memo on every keystroke that produced the same match.
func TestCompletionItem_MutatorsBumpVersion(t *testing.T) {
	t.Parallel()

	mkItem := func() *CompletionItem {
		return NewCompletionItem(
			"internal/ui/chat/user.go",
			FileCompletionValue{Path: "internal/ui/chat/user.go"},
			lipgloss.NewStyle(),
			lipgloss.NewStyle(),
			lipgloss.NewStyle(),
		)
	}

	t.Run("SetFocused", func(t *testing.T) {
		t.Parallel()
		item := mkItem()

		// First transition (false -> true) must bump.
		before := item.Version()
		item.SetFocused(true)
		require.Greater(t, item.Version(), before, "SetFocused(true) must bump")

		// Re-applying the same focus state must not bump.
		stable := item.Version()
		item.SetFocused(true)
		require.Equal(t, stable, item.Version(), "SetFocused with same value must not bump")

		// Transition back must bump.
		item.SetFocused(false)
		require.Greater(t, item.Version(), stable, "SetFocused(false) must bump")
	})

	t.Run("SetMatch", func(t *testing.T) {
		t.Parallel()
		item := mkItem()

		match := fuzzy.Match{
			Str:            "user",
			Index:          0,
			Score:          5,
			MatchedIndexes: []int{0, 1, 2, 3},
		}
		before := item.Version()
		item.SetMatch(match)
		require.Greater(t, item.Version(), before, "SetMatch with new value must bump")

		// Re-applying an equivalent match (same fields, equal slice
		// contents) must not bump.
		stable := item.Version()
		same := fuzzy.Match{
			Str:            "user",
			Index:          0,
			Score:          5,
			MatchedIndexes: []int{0, 1, 2, 3},
		}
		item.SetMatch(same)
		require.Equal(t, stable, item.Version(), "SetMatch with equivalent value must not bump")

		// A different match (different MatchedIndexes) must bump.
		different := fuzzy.Match{
			Str:            "user",
			Index:          0,
			Score:          5,
			MatchedIndexes: []int{0, 2},
		}
		item.SetMatch(different)
		require.Greater(t, item.Version(), stable, "SetMatch with different indexes must bump")
	})
}
