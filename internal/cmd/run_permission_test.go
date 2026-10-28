package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/server"
	"github.com/stretchr/testify/require"
)

// TestRunNonInteractiveAutoApprovesItsSession is the regression test for
// `crush run` hanging in client/server mode on the first tool call that
// needs permission. Local mode auto-approves the run's own session; the
// client/server path has to ask the server for the same, otherwise the
// prompt goes to nobody (`crush run` never answers prompts and no viewer
// is on its session) and the run waits for it forever.
//
// Not parallel: it isolates the config directories with t.Setenv.
func TestRunNonInteractiveAutoApprovesItsSession(t *testing.T) {
	isolateRunConfig(t)
	llm := newRunLLM(t, "probe.txt")
	cwd := t.TempDir()
	writeRunConfig(t, cwd, llm.URL)
	srv, c, ws := startRunWorkspace(t, cwd, nil)

	// Nothing in this test answers prompts, just as nothing does for a
	// real `crush run`, so a prompt means the run would wait forever.
	// Stop the run as soon as one is raised instead of waiting out the
	// deadline.
	bws, err := srv.Backend().GetWorkspace(ws.ID)
	require.NoError(t, err)
	prompts := bws.Permissions.Subscribe(t.Context())

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	var prompted atomic.Pointer[permission.PermissionRequest]
	watched := make(chan struct{})
	go func() {
		defer close(watched)
		select {
		case ev, ok := <-prompts:
			if ok {
				prompted.Store(&ev.Payload)
				cancel()
			}
		case <-ctx.Done():
		}
	}()

	err = runNonInteractive(ctx, c, ws, "write the probe file", "", "", "", true, "", false)
	cancel()
	<-watched
	if p := prompted.Load(); p != nil {
		t.Fatalf("crush run's session %s raised a %s permission prompt that nothing answers; the run hangs on it",
			p.SessionID, p.ToolName)
	}
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(cwd, "probe.txt"))
}

// TestRunNonInteractiveWithServerLackingAutoApprove checks that a server
// predating the auto-approve endpoint (it answers 404) does not fail
// `crush run`: a run that needs no permission still completes.
//
// Not parallel: it isolates the config directories with t.Setenv.
func TestRunNonInteractiveWithServerLackingAutoApprove(t *testing.T) {
	isolateRunConfig(t)
	llm := newRunLLM(t, "")
	cwd := t.TempDir()
	writeRunConfig(t, cwd, llm.URL)
	_, c, ws := startRunWorkspace(t, cwd, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/auto-approve") {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	require.NoError(t, runNonInteractive(ctx, c, ws, "say done", "", "", "", true, "", false))
}

// isolateRunConfig points every config, data and cache location at
// fresh temporary directories so the test never reads or writes the
// host's real Crush state, and keeps the provider catalog offline.
func isolateRunConfig(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"HOME",
		"XDG_CONFIG_HOME",
		"XDG_DATA_HOME",
		"XDG_CACHE_HOME",
		"CRUSH_GLOBAL_CONFIG",
		"CRUSH_GLOBAL_DATA",
		"CRUSH_CACHE_DIR",
	} {
		t.Setenv(name, t.TempDir())
	}
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
}

// writeRunConfig writes a project config whose only provider is the
// fake LLM served at baseURL.
func writeRunConfig(t *testing.T, dir, baseURL string) {
	t.Helper()
	cfg := fmt.Sprintf(`{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true, "disable_metrics": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai-compat",
    "base_url": %q, "api_key": "test",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`, baseURL+"/v1")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "crush.json"), []byte(cfg), 0o644))
}

// startRunWorkspace serves the real server API in process and opens a
// workspace for cwd with its agent initialized for a non-interactive
// run, as `crush run` does before calling runNonInteractive. wrap, when
// non-nil, wraps the server's handler.
func startRunWorkspace(t *testing.T, cwd string, wrap func(http.Handler) http.Handler) (*server.Server, *client.Client, *proto.Workspace) {
	t.Helper()
	srv := server.NewServer(nil, "tcp", "127.0.0.1:0")
	h := srv.Handler()
	if wrap != nil {
		h = wrap(h)
	}
	hs := httptest.NewServer(h)
	t.Cleanup(hs.Close)
	u, err := url.Parse(hs.URL)
	require.NoError(t, err)

	c, err := client.NewClient(cwd, "tcp", u.Host)
	require.NoError(t, err)
	// Retiring the client tears the workspace down, closing its database
	// before t.TempDir cleanup (Windows cannot remove an open file).
	t.Cleanup(func() { _ = c.RetireClient(context.Background()) })

	ws, err := c.CreateWorkspace(t.Context(), proto.Workspace{Path: cwd, DataDir: t.TempDir()})
	require.NoError(t, err)
	require.NoError(t, c.InitiateAgentProcessing(t.Context(), ws.ID, false))
	return srv, c, ws
}

// newRunLLM serves a streaming OpenAI-compatible chat completions API.
// When toolFile is set, an agent turn without a tool result yet is
// answered with a call to the write tool creating toolFile. Everything
// else, including title generation, gets plain text that ends the turn.
func newRunLLM(t *testing.T, toolFile string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
			Tools []json.RawMessage `json:"tools"`
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || json.Unmarshal(body, &req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		toolResult := false
		for _, m := range req.Messages {
			if m.Role == "tool" {
				toolResult = true
			}
		}

		w.Header().Set("Content-Type", "text/event-stream")
		send := func(delta, finish string) {
			_, _ = fmt.Fprintf(w, `data: {"id":"c","object":"chat.completion.chunk","created":0,"model":"mock-model","choices":[{"index":0,"delta":%s,"finish_reason":%s}]}`+"\n\n", delta, finish)
		}
		if toolFile != "" && len(req.Tools) > 0 && !toolResult {
			args, _ := json.Marshal(map[string]string{"file_path": toolFile, "content": "hi\n"})
			send(fmt.Sprintf(`{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"write","arguments":%q}}]}`, args), "null")
			send(`{}`, `"tool_calls"`)
		} else {
			send(`{"role":"assistant","content":"done"}`, "null")
			send(`{}`, `"stop"`)
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv
}
