package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// installSyntheticWorkspace creates a synthetic [backend.Workspace]
// registered with the controller's backend, suitable for handler-level
// tests that do not need a real [app.App]. The workspace's ID is a
// fresh UUID and its path is a tempdir; teardown is the caller's
// responsibility (handlers should not rely on synthetic workspaces
// disappearing automatically).
func installSyntheticWorkspace(t *testing.T, c *controllerV1) *backend.Workspace {
	t.Helper()
	ws := &backend.Workspace{
		ID:   uuid.New().String(),
		Path: t.TempDir(),
	}
	backend.InsertWorkspaceForTest(c.backend, ws)
	return ws
}

// newTestController builds a controllerV1 around a backend without a
// real config store, suitable for handler-level 400 tests.
func newTestController() *controllerV1 {
	s := &Server{}
	s.backend = backend.New(context.Background(), nil, nil)
	return &controllerV1{backend: s.backend, server: s}
}

func TestPostWorkspaces_RejectsMissingClientID(t *testing.T) {
	t.Parallel()
	c := newTestController()

	body, err := json.Marshal(proto.Workspace{Path: t.TempDir()})
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/workspaces", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	c.handlePostWorkspaces(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var perr proto.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &perr))
	require.Contains(t, perr.Message, "client_id")
}

func TestPostWorkspaces_RejectsMalformedClientID(t *testing.T) {
	t.Parallel()
	c := newTestController()

	body, err := json.Marshal(proto.Workspace{Path: t.TempDir(), ClientID: "not-a-uuid"})
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/workspaces", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	c.handlePostWorkspaces(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeleteWorkspace_RejectsMissingClientID(t *testing.T) {
	t.Parallel()
	c := newTestController()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/v1/workspaces/abc", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	c.handleDeleteWorkspaces(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeleteWorkspace_RejectsMalformedClientID(t *testing.T) {
	t.Parallel()
	c := newTestController()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/v1/workspaces/abc?client_id=nope", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	c.handleDeleteWorkspaces(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSubscribeEvents_RejectsMissingClientID(t *testing.T) {
	t.Parallel()
	c := newTestController()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/workspaces/abc/events", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	c.handleGetWorkspaceEvents(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSubscribeEvents_RejectsMalformedClientID(t *testing.T) {
	t.Parallel()
	c := newTestController()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/workspaces/abc/events?client_id=nope", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	c.handleGetWorkspaceEvents(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// postCurrentSession is a small helper that POSTs the JSON body to
// /v1/workspaces/{id}/current-session?client_id=cid and returns the
// recorder. It does not require a real listener.
func postCurrentSession(t *testing.T, c *controllerV1, wsID, clientID, sessionID string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(proto.CurrentSession{SessionID: sessionID})
	require.NoError(t, err)
	url := "/v1/workspaces/" + wsID + "/current-session"
	if clientID != "" {
		url += "?client_id=" + clientID
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(body))
	req.SetPathValue("id", wsID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c.handlePostWorkspaceCurrentSession(rec, req)
	return rec
}

func TestPostCurrentSession_RejectsMissingClientID(t *testing.T) {
	t.Parallel()
	c := newTestController()

	body, err := json.Marshal(proto.CurrentSession{SessionID: "S1"})
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/workspaces/abc/current-session", bytes.NewReader(body))
	req.SetPathValue("id", "abc")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	c.handlePostWorkspaceCurrentSession(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPostCurrentSession_RejectsMalformedClientID(t *testing.T) {
	t.Parallel()
	c := newTestController()

	rec := postCurrentSession(t, c, "abc", "not-a-uuid", "S1")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPostCurrentSession_RejectsBadBody(t *testing.T) {
	t.Parallel()
	c := newTestController()

	cid := uuid.New().String()
	url := "/v1/workspaces/abc/current-session?client_id=" + cid
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader([]byte("not-json")))
	req.SetPathValue("id", "abc")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	c.handlePostWorkspaceCurrentSession(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPostCurrentSession_UnknownWorkspace(t *testing.T) {
	t.Parallel()
	c := newTestController()

	rec := postCurrentSession(t, c, uuid.New().String(), uuid.New().String(), "S1")
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPostCurrentSession_UnknownClient(t *testing.T) {
	t.Parallel()
	c := newTestController()
	ws := installSyntheticWorkspace(t, c)

	rec := postCurrentSession(t, c, ws.ID, uuid.New().String(), "S1")
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPostCurrentSession_HoldOnly(t *testing.T) {
	t.Parallel()
	c := newTestController()
	ws := installSyntheticWorkspace(t, c)

	cid := uuid.New().String()
	require.NoError(t, backend.RegisterClientForTesting(c.backend, ws, cid))
	t.Cleanup(func() { _ = c.backend.DeleteWorkspace(ws.ID, cid) })

	rec := postCurrentSession(t, c, ws.ID, cid, "S1")
	require.Equal(t, http.StatusNotFound, rec.Code, "hold-only client must be rejected")
}

func TestPostCurrentSession_AttachedClientSucceeds(t *testing.T) {
	t.Parallel()
	c := newTestController()
	ws := installSyntheticWorkspace(t, c)

	cid := uuid.New().String()
	require.NoError(t, c.backend.AttachClient(ws.ID, cid, 0))
	t.Cleanup(func() { c.backend.DetachClient(ws.ID, cid) })

	rec := postCurrentSession(t, c, ws.ID, cid, "S1")
	require.Equal(t, http.StatusOK, rec.Code)

	// Clearing also returns 200.
	rec = postCurrentSession(t, c, ws.ID, cid, "")
	require.Equal(t, http.StatusOK, rec.Code)
}
