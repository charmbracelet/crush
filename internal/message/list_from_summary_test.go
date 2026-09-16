package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListFromSummary(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t)

	ids := make([]string, 0, 5)
	for range 5 {
		msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{
			Role:  User,
			Parts: []ContentPart{TextContent{Text: "hi"}},
		})
		require.NoError(t, err)
		ids = append(ids, msg.ID)
	}

	t.Run("empty summary returns the whole session", func(t *testing.T) {
		t.Parallel()

		got, err := svc.ListFromSummary(t.Context(), sessionID, "")
		require.NoError(t, err)
		require.Len(t, got, len(ids))
	})

	t.Run("returns the summary message and everything after it", func(t *testing.T) {
		t.Parallel()

		got, err := svc.ListFromSummary(t.Context(), sessionID, ids[2])
		require.NoError(t, err)

		// created_at is second-resolution, so same-second messages may
		// also come back; what matters is nothing after it was dropped.
		byID := make(map[string]bool, len(got))
		for _, m := range got {
			byID[m.ID] = true
		}
		for _, id := range ids[2:] {
			require.True(t, byID[id], "message at or after the summary must be returned")
		}
	})

	// A missing summary must not silently erase the history.
	t.Run("dangling summary falls back to the whole session", func(t *testing.T) {
		t.Parallel()

		got, err := svc.ListFromSummary(t.Context(), sessionID, "does-not-exist")
		require.NoError(t, err)
		require.Len(t, got, len(ids))
	})
}
