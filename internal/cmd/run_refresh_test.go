package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

func TestRefreshWorkspaceConfig(t *testing.T) {
	const original = `{
		"options": {},
		"models": {"large": {"provider": "fixture", "model": "stale"}}
	}`
	const refreshed = `{
		"options": {},
		"models": {"large": {"provider": "fixture", "model": "selected"}}
	}`

	for _, tt := range []struct {
		name       string
		status     int
		body       string
		wantModel  string
		wantLog    bool
		wantSame   bool
	}{
		{name: "http_error_is_diagnostic_only", status: http.StatusServiceUnavailable, body: `{}`, wantModel: "stale", wantLog: true, wantSame: true},
		{name: "invalid_json_is_diagnostic_only", status: http.StatusOK, body: `{`, wantModel: "stale", wantLog: true, wantSame: true},
		{name: "success_refreshes_workspace", status: http.StatusOK, body: refreshed, wantModel: "selected"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "GET", r.Method)
				require.Equal(t, "/v1/workspaces/test/config", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()

			c, err := client.NewClient(t.TempDir(), "tcp", strings.TrimPrefix(server.URL, "http://"))
			require.NoError(t, err)

			var cfg config.Config
			require.NoError(t, json.Unmarshal([]byte(original), &cfg))
			ws := &proto.Workspace{ID: "test", Config: &cfg}
			originalConfig := ws.Config

			var logs bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
			t.Cleanup(func() { slog.SetDefault(previous) })

			refreshWorkspaceConfig(context.Background(), c, ws)

			require.Equal(t, tt.wantModel, ws.Config.Models[config.SelectedModelTypeLarge].Model)
			if tt.wantSame {
				require.Same(t, originalConfig, ws.Config)
			} else {
				require.NotSame(t, originalConfig, ws.Config)
			}
			if tt.wantLog {
				require.Contains(t, logs.String(), "Failed to refresh config after model override")
			} else {
				require.NotContains(t, logs.String(), "Failed to refresh config after model override")
			}
		})
	}
}
