package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestGitBranchEndpoint covers GET /v1/workspaces/{id}/git/branch:
// the server must report the branch checked out in the workspace's
// working directory so client/server TUIs can display it.
//
// Cannot run in parallel: it isolates HOME/XDG_* via t.Setenv so
// config.Init does not read the host machine's real config.
func TestGitBranchEndpoint(t *testing.T) {
	h := newRealCreateHarness(t)
	h.backend.SetCreateGrace(200 * time.Millisecond)
	h.backend.SetDetachGrace(200 * time.Millisecond)

	wsPath := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = wsPath
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+os.DevNull,
			"GIT_CONFIG_SYSTEM="+os.DevNull,
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	runGit("init")
	runGit("checkout", "-b", "feature/git-branch-endpoint")

	ws := h.postWorkspace(t, proto.Workspace{
		Path:     wsPath,
		DataDir:  t.TempDir(),
		ClientID: uuid.New().String(),
	})

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		h.httpSrv.URL+"/v1/workspaces/"+ws.ID+"/git/branch", nil)
	require.NoError(t, err)
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out proto.GitBranchResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.Equal(t, "feature/git-branch-endpoint", out.Branch)
}

// TestGitBranchEndpoint_UnknownWorkspace verifies the endpoint maps a
// missing workspace to a 404-style error response rather than
// panicking.
func TestGitBranchEndpoint_UnknownWorkspace(t *testing.T) {
	h := newE2EHarness(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		h.httpSrv.URL+"/v1/workspaces/"+uuid.New().String()+"/git/branch", nil)
	require.NoError(t, err)
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.NotEqual(t, http.StatusOK, resp.StatusCode)
}
