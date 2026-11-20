package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func (h *e2eHarness) do(t *testing.T, method, path string, body any) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, h.httpSrv.URL+path, r)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, out
}

func TestMCPToolEndpoints(t *testing.T) {
	h := newE2EHarness(t)
	base := "/v1/workspaces/" + h.workspace.ID + "/mcp/"

	status, body := h.do(t, http.MethodGet, base+"tools", nil)
	require.Equal(t, http.StatusOK, status, string(body))
	var tools map[string]any
	require.NoError(t, json.Unmarshal(body, &tools))

	status, body = h.do(t, http.MethodPost, base+"call-tool", map[string]any{"name": "absent", "tool": "echo"})
	require.Equal(t, http.StatusNotFound, status, string(body))

	status, body = h.do(t, http.MethodPost, base+"call-tool", map[string]any{"name": "absent"})
	require.Equal(t, http.StatusBadRequest, status, string(body))

	status, _ = h.do(t, http.MethodPost, "/v1/workspaces/missing/mcp/call-tool", map[string]any{"name": "a", "tool": "b"})
	require.Equal(t, http.StatusNotFound, status)
}
