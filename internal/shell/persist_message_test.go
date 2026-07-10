package shell

import (
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPersistOutput_SkipsMissingSession(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	messages := message.NewService(db.New(conn))

	missingID := uuid.New().String()
	err = PersistOutput(t.Context(), messages, missingID, "cat file.txt", "hello", 0)
	require.NoError(t, err)

	stored, err := messages.List(t.Context(), missingID)
	require.NoError(t, err)
	require.Empty(t, stored)
}

func TestPersistOutput_NoOpForEmptySessionID(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	messages := message.NewService(db.New(conn))

	require.NoError(t, PersistOutput(t.Context(), messages, "", "echo hi", "hi", 0))
}

func TestPersistOutput_PersistsForExistingSession(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	messages := message.NewService(q)

	sess, err := sessions.Create(t.Context(), "shell test")
	require.NoError(t, err)

	err = PersistOutput(t.Context(), messages, sess.ID, "cat file.txt", "hello", 0)
	require.NoError(t, err)

	stored, err := messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	require.Equal(t, message.User, stored[0].Role)
	shellParts := stored[0].ShellCommands()
	require.Len(t, shellParts, 1)
	require.Equal(t, "cat file.txt", shellParts[0].Command)
	require.Equal(t, "hello", shellParts[0].Output)
	require.Zero(t, shellParts[0].ExitCode)
}
