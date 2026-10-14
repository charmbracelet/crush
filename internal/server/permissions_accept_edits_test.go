package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

// acceptEditsURL builds the accept-edits endpoint URL; an empty session
// leaves the query parameter out.
func acceptEditsURL(h *e2eHarness, wsID, session string) string {
	u := h.httpSrv.URL + "/v1/workspaces/" + wsID + "/permissions/accept-edits"
	if session != "" {
		u += "?session_id=" + url.QueryEscape(session)
	}
	return u
}

func postAcceptEdits(t *testing.T, h *e2eHarness, wsID, session string, on bool) int {
	t.Helper()
	body, err := json.Marshal(proto.PermissionAcceptEditsRequest{AcceptEdits: on})
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, acceptEditsURL(h, wsID, session), bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func getAcceptEdits(t *testing.T, h *e2eHarness, wsID, session string) (int, bool) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, acceptEditsURL(h, wsID, session), nil)
	require.NoError(t, err)
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var out proto.PermissionAcceptEditsRequest
	if resp.StatusCode == http.StatusOK {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	}
	return resp.StatusCode, out.AcceptEdits
}

// TestPermissionsAcceptEditsEndpoint drives the accept-edits endpoint
// through the real router into the workspace's permission service.
func TestPermissionsAcceptEditsEndpoint(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)
	wsID := h.workspace.ID
	perms := h.app.Permissions

	require.Equal(t, http.StatusOK, postAcceptEdits(t, h, wsID, "s1", true))
	require.True(t, perms.AcceptEdits("s1"))
	require.False(t, perms.AcceptEdits("s2"), "only the named session")

	code, on := getAcceptEdits(t, h, wsID, "s1")
	require.Equal(t, http.StatusOK, code)
	require.True(t, on)
	code, on = getAcceptEdits(t, h, wsID, "s2")
	require.Equal(t, http.StatusOK, code)
	require.False(t, on)

	require.Equal(t, http.StatusOK, postAcceptEdits(t, h, wsID, "s1", false))
	require.False(t, perms.AcceptEdits("s1"))
	_, on = getAcceptEdits(t, h, wsID, "s1")
	require.False(t, on)
}

// TestPermissionsAcceptEditsEndpointRejects: no session is a 400 (fail
// closed, nothing is switched on) and an unknown workspace is a 404.
func TestPermissionsAcceptEditsEndpointRejects(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)
	wsID := h.workspace.ID

	require.Equal(t, http.StatusBadRequest, postAcceptEdits(t, h, wsID, "", true))
	require.False(t, h.app.Permissions.AcceptEdits(""))
	code, _ := getAcceptEdits(t, h, wsID, "")
	require.Equal(t, http.StatusBadRequest, code)

	require.Equal(t, http.StatusNotFound, postAcceptEdits(t, h, "does-not-exist", "s1", true))
	code, _ = getAcceptEdits(t, h, "does-not-exist", "s1")
	require.Equal(t, http.StatusNotFound, code)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, acceptEditsURL(h, wsID, "s1"), bytes.NewReader([]byte("not-json")))
	require.NoError(t, err)
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.False(t, h.app.Permissions.AcceptEdits("s1"))
}
