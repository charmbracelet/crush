package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

func TestRunNonInteractive_ModelRefresh(t *testing.T) {
	// This test captures the process stderr, so it must not run in parallel.
	const original = `{
		"options": {},
		"models": {"large": {"provider": "fixture", "model": "stale"}},
		"providers": {"fixture": {"models": [{"id": "selected"}, {"id": "small"}]}}
	}`
	const refreshed = `{
		"options": {},
		"models": {"large": {"provider": "fixture", "model": "selected"}}
	}`
	for _, tt := range []struct {
		name      string
		status    int
		body      string
		wantError string
	}{
		{"http_error", http.StatusServiceUnavailable, `{}`, "status code 503"},
		{"invalid_json", http.StatusOK, `{`, "unexpected EOF"},
		{"success", http.StatusOK, refreshed, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var mu sync.Mutex
			var requests []string
			configReads := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				requests = append(requests, r.Method+" "+r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "GET /v1/workspaces/test/config":
					configReads++
					if configReads == 1 {
						_, _ = io.WriteString(w, original)
						return
					}
					w.WriteHeader(tt.status)
					_, _ = io.WriteString(w, tt.body)
				case "POST /v1/workspaces/test/config/model", "POST /v1/workspaces/test/agent/update":
					w.WriteHeader(http.StatusOK)
				default:
					// Stop the success control at readiness: never start inference.
					// A failed refresh must not reach this path at all.
					cancel()
					w.WriteHeader(http.StatusServiceUnavailable)
				}
			}))
			defer srv.Close()
			c, err := client.NewClient(t.TempDir(), "tcp", strings.TrimPrefix(srv.URL, "http://"))
			require.NoError(t, err)
			var cfg config.Config
			require.NoError(t, json.Unmarshal([]byte(original), &cfg))
			ws := &proto.Workspace{ID: "test", Config: &cfg}
			capture, err := os.CreateTemp(t.TempDir(), "stderr")
			require.NoError(t, err)
			previousStderr := os.Stderr
			os.Stderr = capture
			t.Cleanup(func() {
				os.Stderr = previousStderr
				_ = capture.Close()
			})
			runErr := runNonInteractive(ctx, c, ws, "unused", "fixture/selected", "fixture/small", "", true, "", false)
			os.Stderr = previousStderr
			_, err = capture.Seek(0, io.SeekStart)
			require.NoError(t, err)
			output, err := io.ReadAll(capture)
			require.NoError(t, err)
			mu.Lock()
			observed := append([]string(nil), requests...)
			reads := configReads
			mu.Unlock()
			prefix := []string{
				"GET /v1/workspaces/test/config",
				"POST /v1/workspaces/test/config/model",
				"POST /v1/workspaces/test/config/model",
				"POST /v1/workspaces/test/agent/update",
				"GET /v1/workspaces/test/config",
			}
			require.Equal(t, 2, reads)
			if tt.wantError != "" {
				require.ErrorContains(t, runErr, "failed to refresh config after model override")
				require.ErrorContains(t, runErr, tt.wantError)
				require.Equal(t, prefix, observed, "refresh failure must stop before readiness, session creation, or inference")
				require.NotContains(t, string(output), "crush run:")
				require.Same(t, &cfg, ws.Config)
				return
			}
			require.ErrorIs(t, runErr, context.Canceled)
			require.Len(t, observed, len(prefix)+1)
			require.Equal(t, prefix, observed[:len(prefix)])
			require.Equal(t, "GET /v1/workspaces/test/agent", observed[len(prefix)])
			require.Equal(t, "crush run: fixture/selected\n", string(output))
			require.Equal(t, "selected", ws.Config.Models[config.SelectedModelTypeLarge].Model)
		})
	}
}
