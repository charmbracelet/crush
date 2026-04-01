package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func newSessionSearchCommand(t *testing.T, dataDir string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().String("data-dir", dataDir, "")
	cmd.Flags().String("cwd", "", "")
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd
}

func TestRunSessionSearch(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	q := db.New(conn)
	sessionSvc := session.NewService(q, conn)
	messageSvc := message.NewService(q)
	sess, err := sessionSvc.Create(t.Context(), "session search")
	require.NoError(t, err)
	_, err = messageSvc.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "search keyword appears here"}},
	})
	require.NoError(t, err)
	_, err = messageSvc.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "another unrelated line"}},
	})
	require.NoError(t, err)

	cmd := newSessionSearchCommand(t, dataDir)
	out := &bytes.Buffer{}
	cmd.SetOut(out)

	sessionSearchSession = ""
	sessionSearchLimit = 20
	err = runSessionSearch(cmd, []string{"keyword"})
	require.NoError(t, err)
	require.Contains(t, out.String(), "Found 1 matching messages")
	require.Contains(t, out.String(), "role=user")

	out.Reset()
	sessionSearchSession = session.HashID(sess.ID)[:12]
	err = runSessionSearch(cmd, []string{"keyword"})
	require.NoError(t, err)
	require.Contains(t, out.String(), "Found 1 matching messages")

	out.Reset()
	sessionSearchSession = ""
	err = runSessionSearch(cmd, []string{"missing"})
	require.NoError(t, err)
	require.Contains(t, out.String(), "No matching messages found.")

}

func TestRunSessionSearchRequiresQuery(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	cmd := newSessionSearchCommand(t, dataDir)
	sessionSearchSession = ""
	sessionSearchLimit = 20
	err := runSessionSearch(cmd, []string{"   "})
	require.Error(t, err)
	require.Contains(t, err.Error(), "query cannot be empty")
}

func TestSessionSearchCommandWiring(t *testing.T) {
	t.Parallel()

	sessionSearchSession = ""
	sessionSearchLimit = 20
	defer func() {
		sessionSearchSession = ""
		sessionSearchLimit = 20
	}()

	err := sessionSearchCmd.ParseFlags([]string{"--session", "abc", "--limit", "5"})
	require.NoError(t, err)
	require.Equal(t, "abc", sessionSearchSession)
	require.Equal(t, 5, sessionSearchLimit)
}
