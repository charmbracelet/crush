package backend

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// insertShellWorkspace installs a synthetic workspace backed by a real
// database, with the given coordinator (or none).
func insertShellWorkspace(t *testing.T, b *Backend, coord agent.Coordinator) *Workspace {
	t.Helper()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	q := db.New(conn)
	ws := &Workspace{
		ID:           uuid.New().String(),
		Path:         t.TempDir(),
		resolvedPath: t.TempDir(),
		clients:      make(map[string]*clientState),
		shutdownFn:   func() {},
	}
	ws.App = &app.App{
		Sessions:         session.NewService(q, conn),
		Messages:         message.NewService(q),
		AgentCoordinator: coord,
	}
	ws.ctx, ws.cancel = context.WithCancel(b.ctx)
	InsertWorkspaceForTest(b, ws)
	return ws
}

func TestRunShellCommand_SkipsPersistenceForMissingSession(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws := insertShellWorkspace(t, b, nil)

	missingSessionID := uuid.New().String()
	resp, err := b.RunShellCommand(t.Context(), ws.ID, proto.ShellCommandRequest{
		SessionID: missingSessionID,
		Command:   "echo hello",
	})
	require.NoError(t, err)
	require.Equal(t, "hello\n", resp.Output)
	require.Zero(t, resp.ExitCode)

	stored, err := ws.Messages.List(t.Context(), missingSessionID)
	require.NoError(t, err)
	require.Empty(t, stored)
}

// titleCoordinator reports the titles it is asked to generate on titles,
// once release is closed. Any other call panics.
type titleCoordinator struct {
	agent.Coordinator
	release chan struct{}
	titles  chan [2]string
}

func (c *titleCoordinator) GenerateTitle(_ context.Context, sessionID, prompt string) {
	<-c.release
	c.titles <- [2]string{sessionID, prompt}
}

// A '!' command that starts a session titles it, as it does in local mode,
// without holding back the command's output until the title is done.
func TestRunShellCommand_TitlesTheSessionItStarted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		first     bool
		noAgent   bool
		wantTitle bool
	}{
		{name: "first message", first: true, wantTitle: true},
		{name: "later message"},
		{name: "no agent", first: true, noAgent: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			coord := &titleCoordinator{
				release: make(chan struct{}),
				titles:  make(chan [2]string, 1),
			}
			var c agent.Coordinator = coord
			if tt.noAgent {
				c = nil
			}
			b, _ := newTestBackend(t)
			ws := insertShellWorkspace(t, b, c)
			sess, err := ws.Sessions.Create(t.Context(), "New Session")
			require.NoError(t, err)

			// The output must come back while the title is still being
			// generated.
			type result struct {
				resp proto.ShellCommandResponse
				err  error
			}
			done := make(chan result, 1)
			go func() {
				resp, err := b.RunShellCommand(t.Context(), ws.ID, proto.ShellCommandRequest{
					SessionID:      sess.ID,
					Command:        "echo hello",
					IsFirstMessage: tt.first,
				})
				done <- result{resp, err}
			}()
			select {
			case r := <-done:
				require.NoError(t, r.err)
				require.Equal(t, "hello\n", r.resp.Output)
			case <-time.After(5 * time.Second):
				close(coord.release)
				t.Fatal("the output waited for the title")
			}
			close(coord.release)

			if !tt.wantTitle {
				select {
				case got := <-coord.titles:
					t.Fatalf("unexpected title request %v", got)
				case <-time.After(100 * time.Millisecond):
				}
				return
			}
			select {
			case got := <-coord.titles:
				require.Equal(t, [2]string{sess.ID, "$ echo hello"}, got)
			case <-time.After(5 * time.Second):
				t.Fatal("the session was never titled")
			}
		})
	}
}
