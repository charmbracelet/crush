package acp

import (
	"context"
	"database/sql"
	"errors"
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

func TestInitializeOmitsAdditionalDirectories(t *testing.T) {
	// The capability is not advertised because Crush never enforces a root set
	// (plan O23). Advertising it promised a scope that did not exist.
	a := newTestAgent(&stubSessionService{})
	resp, err := a.Initialize(context.Background(), acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
	})
	require.NoError(t, err)
	require.Nil(t, resp.AgentCapabilities.SessionCapabilities.AdditionalDirectories)
}

func TestNewSessionIgnoresAdditionalDirectories(t *testing.T) {
	// A client that sends roots anyway must not be rejected: the field is
	// simply not applied.
	a := newTestAgent(&stubSessionService{})
	resp, err := a.NewSession(context.Background(), acp.NewSessionRequest{
		AdditionalDirectories: []string{"/extra1"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.SessionId)

	list, err := a.ListSessions(context.Background(), acp.ListSessionsRequest{})
	require.NoError(t, err)
	require.Len(t, list.Sessions, 1)
	require.Empty(t, list.Sessions[0].AdditionalDirectories)
}

func TestValidateCwd(t *testing.T) {
	// No config store means no working directory, so nothing can be compared
	// and the check is a no-op rather than a refusal.
	a := newTestAgent(&stubSessionService{})
	require.NoError(t, a.validateCwd(""))
	require.NoError(t, a.validateCwd("/anything"))
}

func TestValidateCwdRequiresAbsolute(t *testing.T) {
	// overview.md:160 makes every protocol path absolute, cwd included.
	a := newTestAgent(&stubSessionService{})
	err := a.validateCwd("relative/project")
	var reqErr *acp.RequestError
	require.ErrorAs(t, err, &reqErr)
	require.Equal(t, -32602, reqErr.Code)
}

func TestListSessionsRejectsRelativeCwd(t *testing.T) {
	a := newTestAgent(&stubSessionService{})
	rel := "relative/project"
	_, err := a.ListSessions(context.Background(), acp.ListSessionsRequest{Cwd: &rel})
	var reqErr *acp.RequestError
	require.ErrorAs(t, err, &reqErr)
	require.Equal(t, -32602, reqErr.Code)
}

func TestRequireSessionTracksConnectionActiveSet(t *testing.T) {
	// The store knows every id, but a session never set up on this
	// connection is not active here (error.md:56).
	a := newTestAgent(&stubSessionService{})
	err := a.requireSession(context.Background(), "sess-unknown")
	var reqErr *acp.RequestError
	require.ErrorAs(t, err, &reqErr)
	require.Equal(t, -32002, reqErr.Code)

	// Creating it on the connection makes it active.
	resp, err := a.NewSession(context.Background(), acp.NewSessionRequest{})
	require.NoError(t, err)
	require.NoError(t, a.requireSession(context.Background(), resp.SessionId))

	// Closing frees it again.
	_, err = a.CloseSession(context.Background(), acp.CloseSessionRequest{SessionId: resp.SessionId})
	require.NoError(t, err)
	err = a.requireSession(context.Background(), resp.SessionId)
	require.ErrorAs(t, err, &reqErr)
	require.Equal(t, -32002, reqErr.Code)
}

func TestUnstableDeleteSession(t *testing.T) {
	// A session the store does not know deletes silently
	// (session-delete.md:85).
	stub := &stubSessionService{deleteErr: sql.ErrNoRows}
	a := newTestAgent(stub)
	_, err := a.UnstableDeleteSession(context.Background(), acp.UnstableDeleteSessionRequest{SessionId: "abc"})
	require.NoError(t, err)

	// A real failure still surfaces.
	boom := errors.New("disk on fire")
	stub.deleteErr = boom
	_, err = a.UnstableDeleteSession(context.Background(), acp.UnstableDeleteSessionRequest{SessionId: "abc"})
	require.ErrorIs(t, err, boom)
}

func TestListSessionsPaginates(t *testing.T) {
	stub := &stubSessionService{}
	a := newTestAgent(stub)
	for range listPageSize + 5 {
		_, err := a.NewSession(context.Background(), acp.NewSessionRequest{})
		require.NoError(t, err)
	}

	first, err := a.ListSessions(context.Background(), acp.ListSessionsRequest{})
	require.NoError(t, err)
	require.Len(t, first.Sessions, listPageSize)
	require.NotNil(t, first.NextCursor, "a full page must hand back a cursor")

	second, err := a.ListSessions(context.Background(), acp.ListSessionsRequest{Cursor: first.NextCursor})
	require.NoError(t, err)
	require.Len(t, second.Sessions, 5)
	require.Nil(t, second.NextCursor, "the last page must not return a cursor")

	// Cursors are opaque: a garbage token is rejected with -32602.
	bad := "not-a-cursor"
	_, err = a.ListSessions(context.Background(), acp.ListSessionsRequest{Cursor: &bad})
	var reqErr *acp.RequestError
	require.ErrorAs(t, err, &reqErr)
	require.Equal(t, -32602, reqErr.Code)
}

func TestListSessionsFiltersByCwd(t *testing.T) {
	stub := &stubSessionService{}
	a := newTestAgent(stub)
	_, err := a.NewSession(context.Background(), acp.NewSessionRequest{})
	require.NoError(t, err)

	// No config store means no working directory, so the filter is a no-op
	// and every session is listed.
	other := "/some/other/project"
	got, err := a.ListSessions(context.Background(), acp.ListSessionsRequest{Cwd: &other})
	require.NoError(t, err)
	require.Len(t, got.Sessions, 1)
}

func TestListSessionsEmptyReturnsEmptySlice(t *testing.T) {
	a := newTestAgent(&stubSessionService{})
	resp, err := a.ListSessions(context.Background(), acp.ListSessionsRequest{})
	require.NoError(t, err)
	// The SDK validates that sessions is non-nil, so an empty result must
	// still serialize as [] rather than null.
	require.NotNil(t, resp.Sessions)
	require.Empty(t, resp.Sessions)
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
