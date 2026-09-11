package message

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriterGuardBlocksAllTranscriptMutations(t *testing.T) {
	t.Parallel()
	blocked := false
	denial := errors.New("other owner")
	svc, id := newTestService(t, WithWriterGuard(func(context.Context, string) error {
		if blocked {
			return denial
		}
		return nil
	}))
	msg, err := svc.Create(t.Context(), id, CreateMessageParams{Role: User, Parts: []ContentPart{TextContent{Text: "existing"}}})
	require.NoError(t, err)
	blocked = true
	_, err = svc.Create(t.Context(), id, CreateMessageParams{Role: User})
	require.ErrorIs(t, err, denial)
	require.ErrorIs(t, svc.Update(t.Context(), msg), denial)
	require.ErrorIs(t, svc.Delete(t.Context(), msg.ID), denial)
	require.ErrorIs(t, svc.DeleteSessionMessages(t.Context(), id), denial)
	retained, err := svc.List(t.Context(), id)
	require.NoError(t, err)
	require.Len(t, retained, 1)
}
