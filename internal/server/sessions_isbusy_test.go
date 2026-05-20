package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// stubCoordinator is a minimal agent.Coordinator that only reports
// per-session busy state. Every other method returns a zero value so
// the type satisfies the interface without dragging in the full
// coordinator dependency graph.
type stubCoordinator struct {
	busy map[string]bool
}

func (s *stubCoordinator) Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	return nil, nil
}
func (s *stubCoordinator) Cancel(string) {}
func (s *stubCoordinator) CancelAll()    {}
func (s *stubCoordinator) IsBusy() bool  { return false }
func (s *stubCoordinator) IsSessionBusy(id string) bool {
	return s.busy[id]
}
func (s *stubCoordinator) QueuedPrompts(string) int          { return 0 }
func (s *stubCoordinator) QueuedPromptsList(string) []string { return nil }
func (s *stubCoordinator) ClearQueue(string)                 {}
func (s *stubCoordinator) Summarize(context.Context, string) error {
	return nil
}
func (s *stubCoordinator) Model() agent.Model                 { return agent.Model{} }
func (s *stubCoordinator) UpdateModels(context.Context) error { return nil }

// stubSessions is a minimal session.Service that returns a fixed list
// (and supports Get by ID). All other methods return zero values; the
// IsBusy tests do not exercise them.
type stubSessions struct {
	session.Service // embed nil to inherit the unexported broker methods
	all             []session.Session
}

func (s *stubSessions) List(context.Context) ([]session.Session, error) {
	return s.all, nil
}

func (s *stubSessions) Get(_ context.Context, id string) (session.Session, error) {
	for _, sess := range s.all {
		if sess.ID == id {
			return sess, nil
		}
	}
	return session.Session{}, errors.New("not found")
}

// buildBusyWorkspace returns a controller wired to a backend that owns
// a single workspace whose AgentCoordinator reports the named session
// as busy.
func buildBusyWorkspace(t *testing.T, sessionID string, busy bool) (*controllerV1, string) {
	t.Helper()

	b := backend.New(context.Background(), nil, nil)
	wsID := uuid.New().String()
	coord := &stubCoordinator{busy: map[string]bool{sessionID: busy}}
	a := &app.App{AgentCoordinator: coord}
	a.Sessions = &stubSessions{all: []session.Session{{ID: sessionID, Title: "t"}}}

	ws := &backend.Workspace{
		ID:   wsID,
		Path: t.TempDir(),
		App:  a,
	}
	backend.InsertWorkspaceForTest(b, ws)

	s := &Server{backend: b}
	return &controllerV1{backend: b, server: s}, wsID
}

func TestSessionListIncludesIsBusy(t *testing.T) {
	t.Parallel()
	const sid = "s-busy"
	c, wsID := buildBusyWorkspace(t, sid, true)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/workspaces/"+wsID+"/sessions", nil)
	req.SetPathValue("id", wsID)
	rec := httptest.NewRecorder()
	c.handleGetWorkspaceSessions(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var got []proto.Session
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 1)
	require.Equal(t, sid, got[0].ID)
	require.True(t, got[0].IsBusy, "expected IsBusy=true for the busy session")
}

func TestSessionListIdleSessionIsNotBusy(t *testing.T) {
	t.Parallel()
	const sid = "s-idle"
	c, wsID := buildBusyWorkspace(t, sid, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/workspaces/"+wsID+"/sessions", nil)
	req.SetPathValue("id", wsID)
	rec := httptest.NewRecorder()
	c.handleGetWorkspaceSessions(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var got []proto.Session
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 1)
	require.False(t, got[0].IsBusy, "expected IsBusy=false for idle session")
}

func TestSessionGetIncludesIsBusy(t *testing.T) {
	t.Parallel()
	const sid = "s-busy"
	c, wsID := buildBusyWorkspace(t, sid, true)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/workspaces/"+wsID+"/sessions/"+sid, nil)
	req.SetPathValue("id", wsID)
	req.SetPathValue("sid", sid)
	rec := httptest.NewRecorder()
	c.handleGetWorkspaceSession(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var got proto.Session
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, sid, got.ID)
	require.True(t, got.IsBusy)
}

// TestIsSessionBusyNilSafe verifies the helper tolerates a missing
// workspace, app, or coordinator — phase A handlers rely on this so
// they can pass GetWorkspace's result through without an extra guard.
func TestIsSessionBusyNilSafe(t *testing.T) {
	t.Parallel()

	require.False(t, isSessionBusy(nil, "x"))
	require.False(t, isSessionBusy(&backend.Workspace{}, "x"))
	require.False(t, isSessionBusy(&backend.Workspace{App: &app.App{}}, "x"))
}

// TestProtoSessionIsBusyBackwardCompat verifies older consumers that
// unmarshal proto.Session without knowing about IsBusy still succeed
// and ignore the new field harmlessly.
func TestProtoSessionIsBusyBackwardCompat(t *testing.T) {
	t.Parallel()

	wire := proto.Session{ID: "s1", Title: "t", IsBusy: true}
	raw, err := json.Marshal(wire)
	require.NoError(t, err)

	// Old client shape: same struct minus IsBusy. We model this by
	// unmarshaling into a struct that doesn't declare the field.
	type oldSession struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	var old oldSession
	require.NoError(t, json.Unmarshal(raw, &old))
	require.Equal(t, "s1", old.ID)
	require.Equal(t, "t", old.Title)
}
