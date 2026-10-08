package compaction

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func ledgerFixture() Ledger {
	return Ledger{
		CoversThrough: 40,
		Entries: []Entry{
			{
				Seq: 1, ID: "a", Kind: KindObservation, Relevance: RelevanceContext,
				Text: "Listed files in internal/compaction",
			},
			{
				Seq: 2, ID: "b", Kind: KindObservation, Relevance: RelevanceNotable,
				Text: "Migration 20261007000000 applied cleanly",
			},
			{
				Seq: 3, ID: "c", Kind: KindReflection, Relevance: RelevanceDecision,
				Text: "Chose append-only rows over rewriting the summary because rationale is what re-summarizing loses",
			},
			{
				Seq: 4, ID: "d", Kind: KindObservation, Relevance: RelevanceDecision,
				Text: "User rejected splitting the ledger across sessions; memory stays session-scoped",
			},
		},
	}
}

func TestLedgerRenderIsDeterministic(t *testing.T) {
	t.Parallel()

	l := ledgerFixture()
	first := l.Render(RenderBudget(4096))
	for range 8 {
		require.Equal(t, first.Text, l.Render(RenderBudget(4096)).Text,
			"the same ledger must render identically every time; a render that varies "+
				"is as lossy as a model rewrite")
	}
	require.Equal(t, 4, first.Kept)
	require.Zero(t, first.Dropped)
	require.Equal(t, 40, first.CoversThrough)
}

func TestLedgerRenderKeepsDecisionsWhenBudgetIsTight(t *testing.T) {
	t.Parallel()

	// Room for roughly one entry. Losing background detail is acceptable;
	// losing why a choice was made is the failure this ordering prevents.
	got := ledgerFixture().Render(RenderBudget(30))
	require.Positive(t, got.Dropped)
	require.Zero(t, got.DroppedHigh,
		"every dropped entry should have been low relevance: %s", got.Text)
	require.Contains(t, got.Text, "append-only rows")
	require.NotContains(t, got.Text, "Listed files")
}

func TestLedgerRenderOrdersChronologicallyWithinSections(t *testing.T) {
	t.Parallel()

	got := ledgerFixture().Render(0)
	// Budget 0 means no limit, so all four entries render. The observation for
	// the migration (seq 2) must precede the rejected-approach note (seq 4),
	// and the reflection stands in its own section.
	require.Equal(t, 4, got.Kept)
	require.Less(
		t,
		indexOf(got.Text, "Migration 20261007000000"),
		indexOf(got.Text, "User rejected splitting"),
		"selection is by weight but emission must follow the order things happened",
	)
	require.Contains(t, got.Text, "Durable conclusions:")
	require.Contains(t, got.Text, "What happened:")
}

func TestLedgerDropRetiresWithoutDeleting(t *testing.T) {
	t.Parallel()

	l := ledgerFixture()
	l.Entries = append(l.Entries, Entry{
		Seq: 5, ID: "e", Kind: KindDrop,
		Text:    "superseded",
		Retires: []int{2},
	})

	got := l.Render(0)
	require.Equal(t, 3, got.Kept, "the retired observation should not render")
	require.NotContains(t, got.Text, "Migration 20261007000000")
	require.Contains(t, got.Text, "append-only rows")

	// The drop itself is bookkeeping, never checkpoint text.
	require.NotContains(t, got.Text, "superseded")
	require.Len(t, l.Entries, 5, "retiring an entry must not remove any row")
}

func TestLedgerEmpty(t *testing.T) {
	t.Parallel()

	require.True(t, Ledger{}.Empty())
	require.False(t, ledgerFixture().Empty())

	// A ledger whose every entry was retired says so, which is how the caller
	// knows to fall back to summarizing with a model.
	all := ledgerFixture()
	all.Entries = append(all.Entries, Entry{Kind: KindDrop, Retires: []int{1, 2, 3, 4}})
	require.True(t, all.Empty())
}

func TestLedgerUnobserved(t *testing.T) {
	t.Parallel()

	l := ledgerFixture()
	l.Entries[2].Sources = []string{"m-1", "m-2"}
	l.Entries[3].Sources = []string{"m-3"}

	require.Equal(t, []string{"m-9"}, l.Unobserved([]string{"m-1", "m-9"}),
		"a render should be able to admit what it never saw")
	require.Empty(t, l.Unobserved([]string{"m-2", "m-3"}))
}

func TestLedgerBudgetAccountsForEntryTextNotCount(t *testing.T) {
	t.Parallel()

	// Two entries of ~100 tokens each against a 150-token budget: exactly one
	// fits. A budget measured in entry count would have kept both, which is
	// how a render quietly overflows the window it was sized for.
	long := Ledger{Entries: []Entry{
		{
			Seq: 1, Kind: KindObservation, Relevance: RelevanceNotable,
			Text: str(100, "decision to keep the verbatim tail small"),
		},
		{
			Seq: 2, Kind: KindObservation, Relevance: RelevanceNotable,
			Text: str(100, "another claim of comparable weight"),
		},
	}}
	got := long.Render(RenderBudget(150))
	require.Equal(t, 1, got.Kept)
	require.Equal(t, 1, got.Dropped)
	require.LessOrEqual(t, got.Tokens, int64(150), "a render must fit the budget it was given")
	require.Contains(t, got.Text, "another claim",
		"at equal weight the newer entry is the one kept")
}

func str(n int, seed string) string {
	out := seed
	for len(out) < n*charsPerToken {
		out += " " + seed
	}
	return out
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
