package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

var errUnknownAgentForTest = errors.New("main agent not found: bogus")

func postAgentMain(t *testing.T, c *controllerV1, wsID, agentID string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(proto.AgentSetMainRequest{AgentID: agentID})
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/workspaces/"+wsID+"/agent/main", bytes.NewReader(body))
	req.SetPathValue("id", wsID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c.handlePostWorkspaceAgentMain(rec, req)
	return rec
}

func TestPostAgentMain_Success(t *testing.T) {
	t.Parallel()

	coord := newRunCoordinator(func(context.Context) error { return nil })
	c, wsID := buildAgentWorkspace(t, coord)

	rec := postAgentMain(t, c, wsID, "plan")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "plan", coord.lastMainAgentSet.Load())
}

func TestPostAgentMain_CoordinatorError(t *testing.T) {
	t.Parallel()

	coord := newRunCoordinator(func(context.Context) error { return nil })
	coord.setMainAgentErr = errUnknownAgentForTest
	c, wsID := buildAgentWorkspace(t, coord)

	rec := postAgentMain(t, c, wsID, "bogus")
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestPostAgentMain_WorkspaceNotFound(t *testing.T) {
	t.Parallel()

	c, _ := buildAgentWorkspace(t, newRunCoordinator(func(context.Context) error { return nil }))

	rec := postAgentMain(t, c, "does-not-exist", "plan")
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPostAgentMain_BadRequestBody(t *testing.T) {
	t.Parallel()

	c, wsID := buildAgentWorkspace(t, newRunCoordinator(func(context.Context) error { return nil }))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/workspaces/"+wsID+"/agent/main", bytes.NewReader([]byte("not-json")))
	req.SetPathValue("id", wsID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c.handlePostWorkspaceAgentMain(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
