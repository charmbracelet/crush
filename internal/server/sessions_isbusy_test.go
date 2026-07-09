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

func (s *stubCoordinator) RunAccepted(ctx context.Context, accept *agent.AcceptedRun, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	return nil, nil
}

func (s *stubCoordinator) SetMainAgent(context.Context, string) error { return nil }
func (s *stubCoordinator) CurrentAgentID() string                     { return "coder" }
func (s *stubCoordinator) BeginAccepted(sessionID string) *agent.AcceptedRun {
	return nil
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
func (s *stubCoordinator) Model() agent.Model                            { return agent.Model{} }
func (s *stubCoordinator) UpdateModels(context.Context) error            { return nil }
func (s *stubCoordinator) GenerateTitle(context.Context, string, string) {}

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

// buildMultiSessionWorkspace returns a controller wired to a backend
// that owns a workspace with the given session IDs. Used to exercise
// AttachedClients counts across more than one session.
func buildMultiSessionWorkspace(t *testing.T, sessionIDs ...string) (*controllerV1, *backend.Workspace) {
	t.Helper()

	b := backend.New(context.Background(), nil, nil)
	a := &app.App{AgentCoordinator: &stubCoordinator{}}
	sessions := make([]session.Session, len(sessionIDs))
	for i, sid := range sessionIDs {
		sessions[i] = session.Session{ID: sid, Title: sid}
	}
	a.Sessions = &stubSessions{all: sessions}

	ws := &backend.Workspace{
		ID:   uuid.New().String(),
		Path: t.TempDir(),
		App:  a,
	}
	backend.InsertWorkspaceForTest(b, ws)
	// Synthetic workspaces have an incomplete App; bypass the
	// default teardown to avoid panics when the last client detaches.
	backend.SetWorkspaceShutdownFnForTest(ws, func() {})

	s := &Server{backend: b}
	return &controllerV1{backend: b, server: s}, ws
}

// listSessions invokes handleGetWorkspaceSessions and returns the
// decoded response so tests can assert per-session counts.
func listSessions(t *testing.T, c *controllerV1, wsID string) []proto.Session {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/workspaces/"+wsID+"/sessions", nil)
	req.SetPathValue("id", wsID)
	rec := httptest.NewRecorder()
	c.handleGetWorkspaceSessions(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var got []proto.Session
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	return got
}

func countsBySessionID(sessions []proto.Session) map[string]int {
	out := make(map[string]int, len(sessions))
	for _, s := range sessions {
		out[s.ID] = s.AttachedClients
	}
	return out
}

// TestSessionListIncludesAttachedClients walks two sessions through
// the same lifecycle covered by TestAttachedClients_BasicLifecycle in
// the backend package, but observed at the handler boundary.
func TestSessionListIncludesAttachedClients(t *testing.T) {
	t.Parallel()
	c, ws := buildMultiSessionWorkspace(t, "S1", "S2")

	// No attached clients yet.
	counts := countsBySessionID(listSessions(t, c, ws.ID))
	require.Equal(t, 0, counts["S1"])
	require.Equal(t, 0, counts["S2"])

	// Attach A, set to S1: S1=1.
	cidA := uuid.New().String()
	require.NoError(t, c.backend.AttachClient(ws.ID, cidA))
	t.Cleanup(func() { c.backend.DetachClient(ws.ID, cidA) })
	require.NoError(t, c.backend.SetCurrentSession(ws.ID, cidA, "S1"))
	counts = countsBySessionID(listSessions(t, c, ws.ID))
	require.Equal(t, 1, counts["S1"])
	require.Equal(t, 0, counts["S2"])

	// Attach B, set to S1: S1=2.
	cidB := uuid.New().String()
	require.NoError(t, c.backend.AttachClient(ws.ID, cidB))
	require.NoError(t, c.backend.SetCurrentSession(ws.ID, cidB, "S1"))
	counts = countsBySessionID(listSessions(t, c, ws.ID))
	require.Equal(t, 2, counts["S1"])
	require.Equal(t, 0, counts["S2"])

	// B switches to S2: counts redistribute.
	require.NoError(t, c.backend.SetCurrentSession(ws.ID, cidB, "S2"))
	counts = countsBySessionID(listSessions(t, c, ws.ID))
	require.Equal(t, 1, counts["S1"])
	require.Equal(t, 1, counts["S2"])

	// B detaches: S2 drops to 0.
	c.backend.DetachClient(ws.ID, cidB)
	counts = countsBySessionID(listSessions(t, c, ws.ID))
	require.Equal(t, 1, counts["S1"])
	require.Equal(t, 0, counts["S2"])
}

// TestSessionListExcludesHoldOnlyClient verifies that a registered
// client without an SSE stream (streams == 0) does not contribute to
// AttachedClients, even though it has an entry in the workspace's
// clients map.
func TestSessionListExcludesHoldOnlyClient(t *testing.T) {
	t.Parallel()
	c, ws := buildMultiSessionWorkspace(t, "S1")

	cid := uuid.New().String()
	require.NoError(t, backend.RegisterClientForTesting(c.backend, ws, cid))
	t.Cleanup(func() { _ = c.backend.DeleteWorkspace(ws.ID, cid) })

	counts := countsBySessionID(listSessions(t, c, ws.ID))
	require.Equal(t, 0, counts["S1"], "hold-only client must not be counted")
}

// TestSessionListExcludesUnselectedAttachedClient verifies that a
// client with a live SSE stream but no current session
// (currentSessionID == "") does not show up under any session's count.
func TestSessionListExcludesUnselectedAttachedClient(t *testing.T) {
	t.Parallel()
	c, ws := buildMultiSessionWorkspace(t, "S1")

	cid := uuid.New().String()
	require.NoError(t, c.backend.AttachClient(ws.ID, cid))
	t.Cleanup(func() { c.backend.DetachClient(ws.ID, cid) })
	// Intentionally do NOT call SetCurrentSession.

	counts := countsBySessionID(listSessions(t, c, ws.ID))
	require.Equal(t, 0, counts["S1"],
		"attached client with no current session must not contribute to S1")
}

// TestSessionGetIncludesAttachedClients verifies the single-session
// handler also populates AttachedClients.
func TestSessionGetIncludesAttachedClients(t *testing.T) {
	t.Parallel()
	c, ws := buildMultiSessionWorkspace(t, "S1")

	cid := uuid.New().String()
	require.NoError(t, c.backend.AttachClient(ws.ID, cid))
	t.Cleanup(func() { c.backend.DetachClient(ws.ID, cid) })
	require.NoError(t, c.backend.SetCurrentSession(ws.ID, cid, "S1"))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/v1/workspaces/"+ws.ID+"/sessions/S1", nil)
	req.SetPathValue("id", ws.ID)
	req.SetPathValue("sid", "S1")
	rec := httptest.NewRecorder()
	c.handleGetWorkspaceSession(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var got proto.Session
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, 1, got.AttachedClients)
}
