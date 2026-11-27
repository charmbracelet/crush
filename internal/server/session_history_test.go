package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// fixedHistory serves a fixed list of file versions.
type fixedHistory struct {
	history.Service
	files []history.File
}

func (h *fixedHistory) ListBySession(context.Context, string) ([]history.File, error) {
	return h.files, nil
}

// TestSessionHistoryKeepsEveryField verifies that a session's file history
// reaches clients with its session ID and timestamps, which the sidebar
// needs to list the most recently modified files first.
func TestSessionHistoryKeepsEveryField(t *testing.T) {
	t.Parallel()

	b := backend.New(context.Background(), nil, nil)
	file := history.File{
		ID:        "f1",
		SessionID: "s1",
		Path:      "/p/main.go",
		Content:   "package main",
		Version:   2,
		CreatedAt: 10,
		UpdatedAt: 20,
	}
	ws := &backend.Workspace{
		ID:   uuid.New().String(),
		Path: t.TempDir(),
		App:  &app.App{History: &fixedHistory{files: []history.File{file}}},
	}
	backend.InsertWorkspaceForTest(b, ws)
	c := &controllerV1{backend: b, server: &Server{backend: b}}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/v1/workspaces/"+ws.ID+"/sessions/s1/history", nil)
	req.SetPathValue("id", ws.ID)
	req.SetPathValue("sid", "s1")
	rec := httptest.NewRecorder()
	c.handleGetWorkspaceSessionHistory(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var got []proto.File
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, []proto.File{{
		ID:        "f1",
		SessionID: "s1",
		Path:      "/p/main.go",
		Content:   "package main",
		Version:   2,
		CreatedAt: 10,
		UpdatedAt: 20,
	}}, got)
}
