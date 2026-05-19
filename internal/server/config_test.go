package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestHandleGetWorkspaceConfigHas_MissingWorkspaceReturnsNotFound(t *testing.T) {
	t.Parallel()

	cfg := &config.ConfigStore{}
	server := &Server{
		backend: backend.New(context.Background(), cfg, nil),
	}
	controller := &controllerV1{backend: server.backend, server: server}
	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/missing/config/has?scope=global&key=models.large", nil)
	req.SetPathValue("id", "missing")
	recorder := httptest.NewRecorder()

	controller.handleGetWorkspaceConfigHas(recorder, req)

	require.Equal(t, http.StatusNotFound, recorder.Code)
}
