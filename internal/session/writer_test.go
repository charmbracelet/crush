package session_test

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/lock"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestSessionWriterAcrossProcesses(t *testing.T) {
	if mode := os.Getenv("CRUSH_TEST_WRITER_MODE"); mode != "" {
		dataDir := os.Getenv("CRUSH_TEST_WRITER_DIR")
		id := os.Getenv("CRUSH_TEST_WRITER_ID")
		conn, err := db.Connect(t.Context(), dataDir)
		require.NoError(t, err)
		defer db.Release(dataDir)
		queries := db.New(conn)
		sessions := session.NewService(queries, conn)
		messages := message.NewService(queries, message.WithWriterGuard(sessions.AcquireWriter))
		if mode == "hold" {
			require.NoError(t, sessions.AcquireWriter(t.Context(), id))
			require.NoError(t, sessions.AcquireWriter(t.Context(), id))
			fmt.Println("ready")
			_, err := io.Copy(io.Discard, os.Stdin)
			require.NoError(t, err)
			return
		}

		current, err := sessions.Get(t.Context(), id)
		require.NoError(t, err, "another owner must not block reads")
		_, err = sessions.Save(t.Context(), current)
		require.ErrorIs(t, err, lock.ErrContended)
		require.ErrorIs(t, sessions.Rename(t.Context(), id, "renamed"), lock.ErrContended)
		require.ErrorIs(t, sessions.UpdateTitleAndUsage(t.Context(), id, "renamed", 1, 1, 1), lock.ErrContended)
		require.ErrorIs(t, sessions.Delete(t.Context(), id), lock.ErrContended)
		_, err = messages.Create(t.Context(), id, message.CreateMessageParams{Role: message.User})
		require.ErrorIs(t, err, lock.ErrContended)
		retained, err := messages.List(t.Context(), id)
		require.NoError(t, err)
		require.Len(t, retained, 1)
		require.ErrorIs(t, messages.Update(t.Context(), retained[0]), lock.ErrContended)
		require.ErrorIs(t, messages.Delete(t.Context(), retained[0].ID), lock.ErrContended)
		require.ErrorIs(t, messages.DeleteSessionMessages(t.Context(), id), lock.ErrContended)

		other, err := sessions.Create(t.Context(), "Independent session")
		require.NoError(t, err)
		_, err = messages.Create(t.Context(), other.ID, message.CreateMessageParams{Role: message.User})
		require.NoError(t, err, "different sessions may have independent owners")
		return
	}

	t.Parallel()
	dataDir := t.TempDir()
	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Release(dataDir)) })
	queries := db.New(conn)
	sessions := session.NewService(queries, conn)
	created, err := sessions.Create(t.Context(), "Existing session")
	require.NoError(t, err)
	_, err = message.NewService(queries).Create(t.Context(), created.ID, message.CreateMessageParams{Role: message.User})
	require.NoError(t, err)
	command := func(mode string) *exec.Cmd {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestSessionWriterAcrossProcesses$")
		cmd.Env = append(os.Environ(), "CRUSH_TEST_WRITER_MODE="+mode, "CRUSH_TEST_WRITER_DIR="+dataDir, "CRUSH_TEST_WRITER_ID="+created.ID)
		return cmd
	}
	holder := command("hold")
	stdin, err := holder.StdinPipe()
	require.NoError(t, err)
	stdout, err := holder.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, holder.Start())
	t.Cleanup(func() { _ = holder.Process.Kill() })
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "ready\n", line)
	output, err := command("contend").CombinedOutput()
	require.NoError(t, err, "%s", output)
	require.NoError(t, stdin.Close())
	require.NoError(t, holder.Wait())
	require.NoError(t, sessions.AcquireWriter(t.Context(), created.ID), "process exit must release ownership")
	retained, err := sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created.Title, retained.Title)
	_, err = db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	require.NoError(t, db.Release(dataDir))
	output, err = command("contend").CombinedOutput()
	require.NoError(t, err, "ownership must survive non-final release: %s", output)
	require.NoError(t, db.Release(dataDir))
	output, err = command("hold").CombinedOutput()
	require.NoError(t, err, "final release must permit another writer: %s", output)
}
