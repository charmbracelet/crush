package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// buildLevelWorkspace inserts a workspace with a live permission service so
// the approval-level endpoints have real state to read back.
func buildLevelWorkspace(t *testing.T) (*controllerV1, string, permission.Service) {
	t.Helper()

	svc := permission.NewPermissionService(t.TempDir(), false, nil)
	b := backend.New(context.Background(), nil, nil)
	ws := &backend.Workspace{
		ID:   uuid.New().String(),
		Path: t.TempDir(),
		App:  &app.App{Permissions: svc},
	}
	backend.InsertWorkspaceForTest(b, ws)
	backend.SetWorkspaceShutdownFnForTest(ws, func() {})

	return &controllerV1{backend: b, server: &Server{backend: b}}, ws.ID, svc
}

func postPermissionLevel(t *testing.T, c *controllerV1, wsID, level string) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(proto.PermissionLevelRequest{Level: level})
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/workspaces/"+wsID+"/permissions/level", bytes.NewReader(body))
	req.SetPathValue("id", wsID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c.handlePostWorkspacePermissionsLevel(rec, req)
	return rec
}

func TestPostPermissionLevelSetsTheService(t *testing.T) {
	t.Parallel()

	c, wsID, svc := buildLevelWorkspace(t)

	for _, name := range []string{"auto", "bypass", "prompt"} {
		rec := postPermissionLevel(t, c, wsID, name)
		require.Equal(t, http.StatusOK, rec.Code, name)

		want, ok := permission.ParseLevel(name)
		require.True(t, ok)
		require.Equal(t, want, svc.Level(), "%s must move the service to that level", name)
	}
}

func TestPostPermissionLevelRejectsUnknownName(t *testing.T) {
	t.Parallel()

	c, wsID, svc := buildLevelWorkspace(t)

	rec := postPermissionLevel(t, c, wsID, "yolo")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, permission.LevelPrompt, svc.Level(),
		"a rejected request must leave the level untouched")
}

func TestGetPermissionLevelReportsTheService(t *testing.T) {
	t.Parallel()

	c, wsID, svc := buildLevelWorkspace(t)
	svc.SetLevel(permission.LevelAuto)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/workspaces/"+wsID+"/permissions/level", nil)
	req.SetPathValue("id", wsID)
	rec := httptest.NewRecorder()
	c.handleGetWorkspacePermissionsLevel(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got proto.PermissionLevelRequest
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	require.Equal(t, "auto", got.Level)
}
