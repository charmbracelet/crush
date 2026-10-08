package ledger

import (
	"testing"

	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T) (Service, string) {
	t.Helper()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	return NewService(q), sess.ID
}

func TestAppendAssignsSeqAndRoundTrips(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t)

	stored, err := svc.Append(t.Context(), sessionID, []compaction.Entry{
		{
			ID: "a", Kind: compaction.KindObservation, Relevance: compaction.RelevanceNotable,
			Text: "Migration applied cleanly", Sources: []string{"m-1"},
		},
		{
			ID: "b", Kind: compaction.KindReflection, Relevance: compaction.RelevanceDecision,
			Text: "Ledger stays append-only so an earlier reading can be reconstructed",
		},
	})
	require.NoError(t, err)
	require.Len(t, stored, 2)

	// The store owns the clock; callers never assign seq.
	require.Equal(t, 1, stored[0].Seq)
	require.Equal(t, 2, stored[1].Seq)

	got, err := svc.Ledger(t.Context(), sessionID)
	require.NoError(t, err)
	require.Len(t, got.Entries, 2)
	require.Equal(t, "Migration applied cleanly", got.Entries[0].Text)
	require.Equal(t, []string{"m-1"}, got.Entries[0].Sources)
	require.Equal(t, compaction.KindReflection, got.Entries[1].Kind)
	require.Equal(t, 2, got.CoversThrough, "the watermark is the last stored seq")

	n, err := svc.Count(t.Context(), sessionID)
	require.NoError(t, err)
	require.Equal(t, 2, n)
}

func TestAppendIsIdempotentForRetriedTurns(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t)

	entries := []compaction.Entry{{ID: "a", Kind: compaction.KindObservation, Text: "same fact"}}
	first, err := svc.Append(t.Context(), sessionID, entries)
	require.NoError(t, err)
	require.Len(t, first, 1)

	// A retried observer must not fail the append; the conflict means the
	// work is already done.
	second, err := svc.Append(t.Context(), sessionID, entries)
	require.NoError(t, err)
	require.Empty(t, second, "the entry was already stored")

	n, err := svc.Count(t.Context(), sessionID)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

func TestLedgerSurvivesRetirementAndRenders(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t)

	_, err := svc.Append(t.Context(), sessionID, []compaction.Entry{
		{
			ID: "a", Kind: compaction.KindObservation, Relevance: compaction.RelevanceContext,
			Text: "Listed files",
		},
		{
			ID: "b", Kind: compaction.KindObservation, Relevance: compaction.RelevanceDecision,
			Text: "User rejected splitting the ledger across sessions",
		},
		{ID: "c", Kind: compaction.KindDrop, Text: "superseded", Retires: []int{1}},
	})
	require.NoError(t, err)

	l, err := svc.Ledger(t.Context(), sessionID)
	require.NoError(t, err)
	require.Len(t, l.Entries, 3, "retiring an entry removes no row")

	rendered := l.Render(0)
	require.NotContains(t, rendered.Text, "Listed files")
	require.Contains(t, rendered.Text, "User rejected splitting")

	// End to end: a stored ledger that renders deterministically is the whole
	// promise of this store, so assert the render twice.
	require.Equal(t, rendered.Text, l.Render(0).Text)
}
