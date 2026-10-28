package server

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// serveRequest sends one request with the given Host and extra headers
// through srv's full handler chain.
func serveRequest(t *testing.T, srv *Server, method, path, host string, header http.Header, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	req.Host = host
	maps.Copy(req.Header, header)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

// TestCrossOriginHandler_RejectsDNSRebinding covers a page on a domain the
// attacker controls that re-resolves its own name to 127.0.0.1. The
// browser then treats the server as same-origin with the page, so only the
// Host header gives the request away. Served, it reads every workspace's
// environment, API keys included.
func TestCrossOriginHandler_RejectsDNSRebinding(t *testing.T) {
	t.Parallel()

	srv := NewServer(nil, "tcp", "127.0.0.1:4141")
	backend.InsertWorkspaceForTest(srv.Backend(), &backend.Workspace{
		ID:   uuid.New().String(),
		Path: t.TempDir(),
		Cfg:  config.NewTestStore(&config.Config{Options: &config.Options{}}),
		Env:  []string{"ANTHROPIC_API_KEY=sk-ant-rebinding-secret"},
	})

	rec := serveRequest(t, srv, http.MethodGet, "/v1/workspaces",
		"rebind.attacker.example:4141",
		http.Header{"Sec-Fetch-Site": {"same-origin"}}, "")

	require.NotContains(t, rec.Body.String(), "sk-ant-rebinding-secret")
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestCrossOriginHandler_RejectsCrossSiteRequests covers requests that a
// page on another site makes without a CORS preflight, such as a form or
// fetch POST with a text/plain body. They need no DNS trickery: the
// browser sends them to 127.0.0.1 as they are. Served, this one shuts an
// idle server down.
func TestCrossOriginHandler_RejectsCrossSiteRequests(t *testing.T) {
	t.Parallel()

	for name, header := range map[string]http.Header{
		"cross-site origin":   {"Origin": {"https://evil.example"}},
		"opaque origin":       {"Origin": {"null"}},
		"cross-site metadata": {"Sec-Fetch-Site": {"cross-site"}},
		"same-site page": {
			"Origin":         {"http://127.0.0.1:3000"},
			"Sec-Fetch-Site": {"same-site"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			srv := NewServer(nil, "tcp", "127.0.0.1:4141")
			header.Set("Content-Type", "text/plain")
			rec := serveRequest(t, srv, http.MethodPost, "/v1/control",
				"127.0.0.1:4141", header, `{"command":"shutdown_if_idle"}`)

			require.Equal(t, http.StatusForbidden, rec.Code)
		})
	}
}

// TestCrossOriginHandler_AllowsLocalClients covers the Host the crush
// client and its readiness probe send over TCP (the --host address), the
// other names DNS cannot point elsewhere (localhost and IP literals), and
// a browser tab the user opened on the API docs.
func TestCrossOriginHandler_AllowsLocalClients(t *testing.T) {
	t.Parallel()

	srv := NewServer(nil, "tcp", "devbox.example:4141")
	for _, tc := range []struct {
		host   string
		header http.Header
	}{
		{host: "devbox.example:4141"},
		{host: "localhost:4141"},
		{host: "127.0.0.1:4141"},
		{host: "[::1]:4141"},
		{host: "192.168.1.5:4141"},
		{host: "localhost:4141", header: http.Header{"Sec-Fetch-Site": {"none"}}},
		{host: "127.0.0.1:4141", header: http.Header{
			"Origin":         {"http://127.0.0.1:4141"},
			"Sec-Fetch-Site": {"same-origin"},
		}},
	} {
		rec := serveRequest(t, srv, http.MethodGet, "/v1/health", tc.host, tc.header, "")
		require.Equal(t, http.StatusOK, rec.Code, "host %q, header %v", tc.host, tc.header)
	}
}

// TestCrossOriginHandler_IgnoresLocalSockets covers the default
// transports, which browsers cannot reach. Clients there send whatever
// Host their HTTP stack picks, such as the encoded socket path.
func TestCrossOriginHandler_IgnoresLocalSockets(t *testing.T) {
	t.Parallel()

	for _, network := range []string{"unix", "npipe"} {
		srv := NewServer(nil, network, "crush-test.sock")
		rec := serveRequest(t, srv, http.MethodGet, "/v1/health",
			"%2Ftmp%2Fcrush-test.sock", nil, "")
		require.Equal(t, http.StatusOK, rec.Code, network)
	}
}
