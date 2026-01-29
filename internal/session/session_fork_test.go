package session

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestFork(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	defer conn.Close()

	q := db.New(conn)
	svc := NewService(q, conn)
	msgSvc := message.NewService(q)

	sourceSession, err := svc.Create(ctx, "Source Session")
	require.NoError(t, err)

	for range 5 {
		_, err = msgSvc.Create(ctx, sourceSession.ID, message.CreateMessageParams{
			Role: message.User,
			Parts: []message.ContentPart{
				message.TextContent{Text: "Test message"},
			},
		})
		require.NoError(t, err)
	}

	getMessages := func(sessionID string) []message.Message {
		msgs, err := msgSvc.List(ctx, sessionID)
		require.NoError(t, err)
		return msgs
	}

	sourceMessages := getMessages(sourceSession.ID)
	require.Len(t, sourceMessages, 5)

	targetMessageID := sourceMessages[2].ID
	newSession, err := svc.Fork(ctx, sourceSession.ID, targetMessageID, msgSvc)
	require.NoError(t, err)
	require.NotEmpty(t, newSession.ID)
	require.NotEqual(t, sourceSession.ID, newSession.ID)
	require.Contains(t, newSession.Title, "Forked:")
	require.Contains(t, newSession.Title, sourceSession.Title)

	forkedMessages := getMessages(newSession.ID)
	require.Len(t, forkedMessages, 2)

	for i, msg := range forkedMessages {
		require.Equal(t, sourceMessages[i].Role, msg.Role)
		require.Equal(t, sourceMessages[i].Parts[0], msg.Parts[0])
	}
}

func TestForkInvalidMessageID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	defer conn.Close()

	q := db.New(conn)
	svc := NewService(q, conn)
	msgSvc := message.NewService(q)

	sourceSession, err := svc.Create(ctx, "Source Session")
	require.NoError(t, err)

	_, err = svc.Fork(ctx, sourceSession.ID, "invalid-id", msgSvc)
	require.Error(t, err)
	require.Contains(t, err.Error(), "message not found")
}
