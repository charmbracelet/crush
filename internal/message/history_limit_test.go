package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUserMessageQueriesAreBounded pins the prompt-history depth at 200.
func TestUserMessageQueriesAreBounded(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t)
	for range promptHistoryProbeCount {
		_, err := svc.Create(t.Context(), sessionID, CreateMessageParams{
			Role:  User,
			Parts: []ContentPart{TextContent{Text: "prompt"}},
		})
		require.NoError(t, err)
	}

	t.Run("per session", func(t *testing.T) {
		t.Parallel()

		got, err := svc.ListUserMessages(t.Context(), sessionID)
		require.NoError(t, err)
		require.Len(t, got, 200)
		for _, m := range got {
			require.Equal(t, User, m.Role)
		}
	})

	t.Run("across all sessions", func(t *testing.T) {
		t.Parallel()

		got, err := svc.ListAllUserMessages(t.Context())
		require.NoError(t, err)
		require.Len(t, got, 200)
		for _, m := range got {
			require.Equal(t, User, m.Role)
		}
	})
}

// Past the 200 the queries return, so a missing LIMIT shows up.
const promptHistoryProbeCount = 250
