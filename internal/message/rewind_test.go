package message

import (
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRestoreConversationPreservesIDsAndSummaryCursor(t *testing.T) {
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	defer conn.Close()
	q := db.New(conn)
	sessions := session.NewService(q, conn)
	messages := NewService(q)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)
	first, err := messages.Create(t.Context(), sess.ID, CreateMessageParams{Role: User, Parts: []ContentPart{TextContent{Text: "original prompt"}}})
	require.NoError(t, err)
	before, err := SnapshotConversation(t.Context(), messages, sess)
	require.NoError(t, err)
	second, err := messages.Create(t.Context(), sess.ID, CreateMessageParams{Role: Assistant, Parts: []ContentPart{TextContent{Text: "summary"}}, IsSummaryMessage: true})
	require.NoError(t, err)
	sess.SummaryMessageID = second.ID
	sess, err = sessions.Save(t.Context(), sess)
	require.NoError(t, err)
	after, err := SnapshotConversation(t.Context(), messages, sess)
	require.NoError(t, err)
	require.NoError(t, RestoreConversation(t.Context(), messages, conn, sess.ID, before))
	rows, err := messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, first.ID, rows[0].ID)
	restored, err := sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Empty(t, restored.SummaryMessageID)
	require.NoError(t, RestoreConversation(t.Context(), messages, conn, sess.ID, after))
	rows, err = messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	restored, err = sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, second.ID, restored.SummaryMessageID)
	other, err := sessions.Create(t.Context(), "other")
	require.NoError(t, err)
	require.Error(t, RestoreConversation(t.Context(), messages, conn, other.ID, before))
}
