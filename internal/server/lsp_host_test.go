package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// lspHostJSON is a registration as a host sends it.
const lspHostJSON = `{"protocol":1,"name":"joy","version":"0.9.0","pid":43549,` +
	`"address":"unix:///run/joy-501/lsp-3f9c2a1b7e4d5a60.sock",` +
	`"servers":[{"name":"gopls","command":"gopls"},{"name":"rust-analyzer","command":"rust-analyzer"}]}`

// postLSPHostBody calls the POST lsp-host handler directly with a raw
// body. An empty clientID leaves the client_id parameter out.
// The workspaces of these tests use a host's language servers, as one a
// host started Crush for does. Set for the whole package, so the tests
// stay parallel; it only matters to the LSP host.
func init() { _ = os.Setenv(backend.LSPHostEnv, "1") }

func postLSPHostBody(t *testing.T, c *controllerV1, wsID, clientID, body string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/v1/workspaces/" + wsID + "/lsp-host"
	if clientID != "" {
		target += "?client_id=" + clientID
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(body))
	req.SetPathValue("id", wsID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c.handlePostWorkspaceLSPHost(rec, req)
	return rec
}

// getLSPHost calls the GET lsp-host handler directly.
func getLSPHost(t *testing.T, c *controllerV1, wsID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/workspaces/"+wsID+"/lsp-host", nil)
	req.SetPathValue("id", wsID)
	rec := httptest.NewRecorder()
	c.handleGetWorkspaceLSPHost(rec, req)
	return rec
}

// claimingClient returns the ID of a new client that holds a claim on ws.
func claimingClient(t *testing.T, c *controllerV1, ws *backend.Workspace) string {
	t.Helper()
	id := uuid.New().String()
	require.NoError(t, backend.RegisterClientForTesting(c.backend, ws, id))
	return id
}

func requireErrorContains(t *testing.T, rec *httptest.ResponseRecorder, status int, want string) {
	t.Helper()
	require.Equal(t, status, rec.Code, rec.Body.String())
	var perr proto.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &perr))
	require.Contains(t, perr.Message, want)
}

func TestLSPHost_UnknownWorkspace(t *testing.T) {
	t.Parallel()
	c := newTestController()

	require.Equal(t, http.StatusNotFound, getLSPHost(t, c, "nope").Code)
	require.Equal(t, http.StatusNotFound,
		postLSPHostBody(t, c, "nope", uuid.New().String(), `{"lsp_host":`+lspHostJSON+`}`).Code)
}

// TestLSPHost_SetGetClear pins the wire shape of the endpoints.
func TestLSPHost_SetGetClear(t *testing.T) {
	t.Parallel()
	c := newTestController()
	ws := installSyntheticWorkspace(t, c) // No App: exercises the publish guard.
	client := claimingClient(t, c, ws)

	none := `{"workspace_id":"` + ws.ID + `","lsp_host":null}`
	rec := getLSPHost(t, c, ws.ID)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, none, rec.Body.String())

	rec = postLSPHostBody(t, c, ws.ID, client, `{"lsp_host":`+lspHostJSON+`}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	registered := `{"workspace_id":"` + ws.ID + `","client_id":"` + client + `","lsp_host":` + lspHostJSON + `}`
	require.JSONEq(t, registered, rec.Body.String())
	require.JSONEq(t, registered, getLSPHost(t, c, ws.ID).Body.String())

	rec = postLSPHostBody(t, c, ws.ID, client, `{"lsp_host":null}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.JSONEq(t, `{"workspace_id":"`+ws.ID+`","client_id":"`+client+`","lsp_host":null}`, rec.Body.String())
	require.JSONEq(t, none, getLSPHost(t, c, ws.ID).Body.String())
}

func TestLSPHost_RequiresAClaim(t *testing.T) {
	t.Parallel()
	c := newTestController()
	ws := installSyntheticWorkspace(t, c)

	stranger := uuid.New().String()
	requireErrorContains(t, postLSPHostBody(t, c, ws.ID, stranger, `{"lsp_host":`+lspHostJSON+`}`),
		http.StatusConflict, "no claim")
	requireErrorContains(t, postLSPHostBody(t, c, ws.ID, stranger, `{"lsp_host":null}`),
		http.StatusConflict, "no claim")
	require.JSONEq(t, `{"workspace_id":"`+ws.ID+`","lsp_host":null}`, getLSPHost(t, c, ws.ID).Body.String())
}

func TestLSPHost_RejectsInvalidRequests(t *testing.T) {
	t.Parallel()
	c := newTestController()
	ws := installSyntheticWorkspace(t, c)
	client := claimingClient(t, c, ws)

	var servers []string
	for i := range proto.MaxLSPHostServers + 1 {
		servers = append(servers, fmt.Sprintf(`{"name":"s%d","command":"c%d"}`, i, i))
	}
	host := func(fields string) string {
		return `{"lsp_host":{` + fields + `}}`
	}

	cases := []struct {
		name     string
		clientID string
		body     string
		want     string
	}{
		{"no client_id", "", `{"lsp_host":` + lspHostJSON + `}`, "client_id is required"},
		{"bad client_id", "not-a-uuid", `{"lsp_host":` + lspHostJSON + `}`, "not a valid UUID"},
		{"bad json", client, `{"lsp_host":`, "decode"},
		{"empty body", client, ``, "decode"},
		{"host not an object", client, `{"lsp_host":5}`, "decode"},
		{"missing key", client, `{}`, `"lsp_host": null`},
		{"host without its wrapper", client, lspHostJSON, `"lsp_host": null`},
		{"no protocol", client, host(`"address":"unix:///run/joy/lsp.sock","servers":[]`), "protocol 0"},
		{"newer protocol", client, host(`"protocol":2,"address":"unix:///run/joy/lsp.sock","servers":[]`), "protocol 2"},
		{"no address", client, host(`"protocol":1,"servers":[]`), "unix://"},
		{"tcp address", client, host(`"protocol":1,"address":"tcp://127.0.0.1:1","servers":[]`), "unix://"},
		{"relative address", client, host(`"protocol":1,"address":"unix://run/joy/lsp.sock","servers":[]`), "absolute"},
		{"too many servers", client, host(`"protocol":1,"address":"unix:///run/joy/lsp.sock","servers":[` + strings.Join(servers, ",") + `]`), "at most 64"},
		{"server without command", client, host(`"protocol":1,"address":"unix:///run/joy/lsp.sock","servers":[{"name":"gopls"}]`), "servers[0]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			requireErrorContains(t, postLSPHostBody(t, c, ws.ID, tc.clientID, tc.body), http.StatusBadRequest, tc.want)
		})
	}
}

func TestLSPHost_IgnoresUnknownKeys(t *testing.T) {
	t.Parallel()
	c := newTestController()
	ws := installSyntheticWorkspace(t, c)
	client := claimingClient(t, c, ws)

	body := `{"lsp_host":{"protocol":1,"address":"unix:///run/joy/lsp.sock","features":["x"],` +
		`"servers":[{"name":"gopls","command":"gopls","languages":["go"]}]},"future_field":1}`
	rec := postLSPHostBody(t, c, ws.ID, client, body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestLSPHost_RejectsOversizedBody(t *testing.T) {
	t.Parallel()
	c := newTestController()
	ws := installSyntheticWorkspace(t, c)
	client := claimingClient(t, c, ws)

	body := `{"lsp_host":{"protocol":1,"address":"unix:///run/joy/lsp.sock","name":"` + strings.Repeat("x", maxLSPHostBody) + `"}}`
	rec := postLSPHostBody(t, c, ws.ID, client, body)
	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	require.JSONEq(t, `{"workspace_id":"`+ws.ID+`","lsp_host":null}`, getLSPHost(t, c, ws.ID).Body.String())
}

// TestLSPHostEventWireShape pins the SSE envelope of an lsp_host event.
func TestLSPHostEventWireShape(t *testing.T) {
	t.Parallel()

	var host proto.LSPHost
	require.NoError(t, json.Unmarshal([]byte(lspHostJSON), &host))

	env := wrapEvent(pubsub.Event[proto.LSPHostState]{
		Type:    pubsub.UpdatedEvent,
		Payload: proto.LSPHostState{WorkspaceID: "w1", ClientID: "c1", LSPHost: &host},
	})
	require.NotNil(t, env)
	data, err := json.Marshal(env)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"lsp_host","payload":{"type":"updated","payload":`+
		`{"workspace_id":"w1","client_id":"c1","lsp_host":`+lspHostJSON+`}}}`, string(data))

	env = wrapEvent(pubsub.Event[proto.LSPHostState]{
		Type:    pubsub.DeletedEvent,
		Payload: proto.LSPHostState{WorkspaceID: "w1"},
	})
	require.NotNil(t, env)
	data, err = json.Marshal(env)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"lsp_host","payload":{"type":"deleted","payload":`+
		`{"workspace_id":"w1","lsp_host":null}}}`, string(data))
}

// postLSPHost drives the real POST lsp-host endpoint for the given client
// and returns the status and, on success, the state it responds with.
func (h *e2eHarness) postLSPHost(t *testing.T, ctx context.Context, workspaceID, clientID string, host *proto.LSPHost) (int, proto.LSPHostState) {
	t.Helper()
	body, err := json.Marshal(proto.LSPHostRequest{LSPHost: host})
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		h.httpSrv.URL+"/v1/workspaces/"+workspaceID+"/lsp-host?client_id="+clientID,
		strings.NewReader(string(body)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var st proto.LSPHostState
	if resp.StatusCode == http.StatusOK {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&st))
	}
	return resp.StatusCode, st
}

// getLSPHost drives the real GET lsp-host endpoint.
func (h *e2eHarness) getLSPHost(t *testing.T, ctx context.Context, workspaceID string) proto.LSPHostState {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		h.httpSrv.URL+"/v1/workspaces/"+workspaceID+"/lsp-host", nil)
	require.NoError(t, err)
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var st proto.LSPHostState
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&st))
	return st
}

// retireClient drives DELETE /v1/clients/{id}, which a host sends when it
// disconnects.
func (h *e2eHarness) retireClient(t *testing.T, ctx context.Context, clientID string) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, h.httpSrv.URL+"/v1/clients/"+clientID, nil)
	require.NoError(t, err)
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// nextLSPHost returns the next lsp_host event on evc that match accepts;
// a nil match accepts any.
func nextLSPHost(t *testing.T, ctx context.Context, evc <-chan any, match func(pubsub.Event[proto.LSPHostState]) bool) pubsub.Event[proto.LSPHostState] {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	ev, ok := drainUntil(ctx, evc, match)
	require.True(t, ok, "expected an lsp_host event")
	return ev
}

func e2eLSPHost(name string) *proto.LSPHost {
	return &proto.LSPHost{
		Protocol: 1,
		Name:     name,
		Address:  "unix:///run/" + name + "/lsp.sock",
		Servers:  []proto.LSPHostServer{{Name: "gopls", Command: "gopls"}},
	}
}

// TestE2E_LSPHostLifecycle drives the host API over HTTP and SSE the way
// two hosts on one workspace do: a registration needs the client's stream,
// reaches every client as an event, the newest wins, and a host that
// leaves hands over to the one before it.
func TestE2E_LSPHostLifecycle(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)
	ctx := t.Context()
	wsID := h.workspace.ID

	older, newer, watcher := uuid.New().String(), uuid.New().String(), uuid.New().String()

	// Before its event stream is attached a client holds no claim.
	status, _ := h.postLSPHost(t, ctx, wsID, older, e2eLSPHost("older"))
	require.Equal(t, http.StatusConflict, status)

	evcOlder, cancelOlder := h.subscribeSSE(t, ctx, wsID, older)
	t.Cleanup(cancelOlder)
	evcNewer, cancelNewer := h.subscribeSSE(t, ctx, wsID, newer)
	t.Cleanup(cancelNewer)
	evcWatcher, cancelWatcher := h.subscribeSSE(t, ctx, wsID, watcher)
	t.Cleanup(cancelWatcher)
	h.waitForAttached(t, 3)
	all := []<-chan any{evcOlder, evcNewer, evcWatcher}

	status, st := h.postLSPHost(t, ctx, wsID, older, e2eLSPHost("older"))
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, proto.LSPHostState{WorkspaceID: wsID, ClientID: older, LSPHost: e2eLSPHost("older")}, st)
	for _, evc := range all {
		ev := nextLSPHost(t, ctx, evc, nil)
		require.Equal(t, pubsub.UpdatedEvent, ev.Type)
		require.Equal(t, proto.LSPHostState{WorkspaceID: wsID, ClientID: older, LSPHost: e2eLSPHost("older")}, ev.Payload)
	}

	status, _ = h.postLSPHost(t, ctx, wsID, newer, e2eLSPHost("newer"))
	require.Equal(t, http.StatusOK, status)
	for _, evc := range all {
		ev := nextLSPHost(t, ctx, evc, nil)
		require.Equal(t, pubsub.UpdatedEvent, ev.Type)
		require.Equal(t, newer, ev.Payload.ClientID)
		require.Equal(t, "newer", ev.Payload.LSPHost.Name)
	}
	require.Equal(t, newer, h.getLSPHost(t, ctx, wsID).ClientID)

	// The newest host exits: it retires its client, and the older host is
	// the workspace's again without saying anything.
	h.retireClient(t, ctx, newer)
	for _, evc := range []<-chan any{evcOlder, evcWatcher} {
		ev := nextLSPHost(t, ctx, evc, nil)
		require.Equal(t, pubsub.UpdatedEvent, ev.Type)
		require.Equal(t, older, ev.Payload.ClientID)
		require.Equal(t, "older", ev.Payload.LSPHost.Name)
	}
	st = h.getLSPHost(t, ctx, wsID)
	require.Equal(t, older, st.ClientID)
	require.Equal(t, "older", st.LSPHost.Name)

	// The last host clears its registration.
	status, st = h.postLSPHost(t, ctx, wsID, older, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, st.LSPHost)
	ev := nextLSPHost(t, ctx, evcWatcher, nil)
	require.Equal(t, pubsub.DeletedEvent, ev.Type)
	require.Equal(t, proto.LSPHostState{WorkspaceID: wsID}, ev.Payload)
	require.Equal(t, proto.LSPHostState{WorkspaceID: wsID}, h.getLSPHost(t, ctx, wsID))
}

// TestE2E_LSPHostReplayedOnAttach verifies that a client attaching after
// the host announced itself is told about it first, and that nothing is
// replayed once there is no host.
func TestE2E_LSPHostReplayedOnAttach(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)
	ctx := t.Context()
	wsID := h.workspace.ID

	host := uuid.New().String()
	_, cancelHost := h.subscribeSSE(t, ctx, wsID, host)
	t.Cleanup(cancelHost)
	h.waitForAttached(t, 1)
	status, _ := h.postLSPHost(t, ctx, wsID, host, e2eLSPHost("early"))
	require.Equal(t, http.StatusOK, status)

	evc, cancel := h.subscribeSSE(t, ctx, wsID, uuid.New().String())
	t.Cleanup(cancel)
	ev := nextLSPHost(t, ctx, evc, nil)
	require.Equal(t, pubsub.UpdatedEvent, ev.Type)
	require.Equal(t, proto.LSPHostState{WorkspaceID: wsID, ClientID: host, LSPHost: e2eLSPHost("early")}, ev.Payload)

	status, _ = h.postLSPHost(t, ctx, wsID, host, nil)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, pubsub.DeletedEvent, nextLSPHost(t, ctx, evc, nil).Type)

	// A late client after the clear gets no replay: the first lsp_host
	// event it sees is the next registration.
	late, cancelLate := h.subscribeSSE(t, ctx, wsID, uuid.New().String())
	t.Cleanup(cancelLate)
	h.waitForAttached(t, 3)
	status, _ = h.postLSPHost(t, ctx, wsID, host, e2eLSPHost("next"))
	require.Equal(t, http.StatusOK, status)
	ev = nextLSPHost(t, ctx, late, nil)
	require.Equal(t, pubsub.UpdatedEvent, ev.Type)
	require.Equal(t, "next", ev.Payload.LSPHost.Name)
}
