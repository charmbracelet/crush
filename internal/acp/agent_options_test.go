package acp

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/app"
	acp "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
)

func newTestAgent(stub *stubSessionService) *crushAgent {
	return &crushAgent{
		app: &app.App{Sessions: stub},
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestNewSessionStoresAdditionalDirs(t *testing.T) {
	stub := &stubSessionService{}
	a := newTestAgent(stub)
	resp, err := a.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:                   "/workspace",
		AdditionalDirectories: []string{"/extra1", "/extra2"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.SessionId)

	list, err := a.ListSessions(context.Background(), acp.ListSessionsRequest{})
	require.NoError(t, err)
	require.Len(t, list.Sessions, 1)
	require.Equal(t, []string{"/extra1", "/extra2"}, list.Sessions[0].AdditionalDirectories)
}

func TestUnstableDeleteSession(t *testing.T) {
	stub := &stubSessionService{}
	a := newTestAgent(stub)
	_, err := a.UnstableDeleteSession(context.Background(), acp.UnstableDeleteSessionRequest{SessionId: "abc"})
	require.NoError(t, err)

	// A stored additional-directories key is cleared without error.
	a.additionalDirs.Store("abc", []string{"/x"})
	_, err = a.UnstableDeleteSession(context.Background(), acp.UnstableDeleteSessionRequest{SessionId: "abc"})
	require.NoError(t, err)
}

func TestLogoutNoop(t *testing.T) {
	a := &crushAgent{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	_, err := a.Logout(context.Background(), acp.LogoutRequest{})
	require.NoError(t, err)
}

func TestFSReadWriteFallback(t *testing.T) {
	// Zero clientCaps means fs is unsupported, so the helpers fall back to the
	// local filesystem.
	a := &crushAgent{}
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "file.txt")
	require.NoError(t, a.fsWriteTextFile(context.Background(), "", p, "hello world"))
	got, err := a.fsReadTextFile(context.Background(), "", p, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "hello world", got)
}
