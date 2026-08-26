package server

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// postAutoApprove drives POST /v1/workspaces/{id}/permissions/auto-approve
// with a raw body (so malformed JSON can be tested too) and returns the
// status code.
func postAutoApprove(t *testing.T, h *e2eHarness, workspaceID, body string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		h.httpSrv.URL+"/v1/workspaces/"+workspaceID+"/permissions/auto-approve",
		bytes.NewReader([]byte(body)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// deleteAutoApprove drives
// DELETE /v1/workspaces/{id}/permissions/auto-approve/{sid} and returns
// the status code.
func deleteAutoApprove(t *testing.T, h *e2eHarness, workspaceID, sessionID string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete,
		h.httpSrv.URL+"/v1/workspaces/"+workspaceID+"/permissions/auto-approve/"+sessionID, nil)
	require.NoError(t, err)
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// requestPermission calls the live permission service the way a tool
// would and returns its verdict. The bounded context is what turns the
// "nobody can answer this prompt" hang from issue 3648 into a test
// failure instead of a stuck test binary.
func requestPermission(t *testing.T, h *e2eHarness, sessionID string, wait time.Duration) (bool, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), wait)
	defer cancel()
	return h.app.Permissions.Request(ctx, permission.CreatePermissionRequest{
		SessionID:   sessionID,
		ToolCallID:  "tc-" + sessionID,
		ToolName:    "write",
		Description: "write a file",
		Action:      "write",
		Path:        h.workspace.Path,
	})
}

// TestPostPermissionsAutoApprove_GrantsWithoutAttachedClient pins the
// fix for issue 3648: `crush run` in client/server mode auto-approves
// its own session over HTTP and then nothing answers permission
// prompts, so the grant must come from the session marker alone. No SSE
// client is subscribed here on purpose — that is the shape of the bug.
func TestPostPermissionsAutoApprove_GrantsWithoutAttachedClient(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)

	require.Equal(t, http.StatusOK,
		postAutoApprove(t, h, h.workspace.ID, `{"session_id":"s-auto"}`))

	granted, err := requestPermission(t, h, "s-auto", 5*time.Second)
	require.NoError(t, err, "auto-approved session must not block on a permission prompt")
	require.True(t, granted)
}

// TestPostPermissionsAutoApprove_ScopedToOneSession checks that the
// route does not turn into workspace-wide yolo mode: an unrelated
// session still waits for an answer, and the skip-requests flag that an
// attached interactive TUI observes stays off.
func TestPostPermissionsAutoApprove_ScopedToOneSession(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)

	require.Equal(t, http.StatusOK,
		postAutoApprove(t, h, h.workspace.ID, `{"session_id":"s-auto"}`))

	granted, err := requestPermission(t, h, "s-other", 200*time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded,
		"a session that was not auto-approved must still wait for an answer")
	require.False(t, granted)

	require.False(t, h.app.Permissions.SkipRequests(),
		"auto-approving a session must not flip workspace-wide skip requests")

	// The blocked request above released the service's request lock on
	// its way out, so the auto-approved session still works.
	granted, err = requestPermission(t, h, "s-auto", 5*time.Second)
	require.NoError(t, err)
	require.True(t, granted)
}

func TestPostPermissionsAutoApprove_RejectsBadRequests(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)

	for _, tc := range []struct {
		name        string
		workspaceID string
		body        string
		want        int
	}{
		{"empty session id", h.workspace.ID, `{"session_id":""}`, http.StatusBadRequest},
		{"missing session id", h.workspace.ID, `{}`, http.StatusBadRequest},
		{"malformed body", h.workspace.ID, `not json`, http.StatusBadRequest},
		{"unknown workspace", uuid.New().String(), `{"session_id":"s-auto"}`, http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, postAutoApprove(t, h, tc.workspaceID, tc.body))
		})
	}
}

// TestDeletePermissionsAutoApprove_StopsGranting pins the exit half of
// the `crush run` contract: the server's permission service outlives the
// run whenever another client holds the workspace, so once the run gives
// its hold back the session must wait for a human again instead of
// staying in a silent per-session yolo state.
func TestDeletePermissionsAutoApprove_StopsGranting(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)

	require.Equal(t, http.StatusOK,
		postAutoApprove(t, h, h.workspace.ID, `{"session_id":"s-auto"}`))
	granted, err := requestPermission(t, h, "s-auto", 5*time.Second)
	require.NoError(t, err)
	require.True(t, granted)

	require.Equal(t, http.StatusOK,
		deleteAutoApprove(t, h, h.workspace.ID, "s-auto"))

	granted, err = requestPermission(t, h, "s-auto", 200*time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded,
		"a revoked session must block on the prompt again")
	require.False(t, granted)
}

// TestDeletePermissionsAutoApprove_ScopedToOneSession checks that
// revoking one session leaves another session's approval alone, so two
// concurrent runs cannot cancel each other's hold.
func TestDeletePermissionsAutoApprove_ScopedToOneSession(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)

	for _, sid := range []string{"s-one", "s-two"} {
		require.Equal(t, http.StatusOK,
			postAutoApprove(t, h, h.workspace.ID, `{"session_id":"`+sid+`"}`))
	}

	require.Equal(t, http.StatusOK,
		deleteAutoApprove(t, h, h.workspace.ID, "s-one"))

	granted, err := requestPermission(t, h, "s-two", 5*time.Second)
	require.NoError(t, err, "revoking one session must not touch another")
	require.True(t, granted)

	granted, err = requestPermission(t, h, "s-one", 200*time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.False(t, granted)
}

func TestDeletePermissionsAutoApprove_StatusCodes(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)

	for _, tc := range []struct {
		name        string
		workspaceID string
		sessionID   string
		want        int
	}{
		{"never approved session", h.workspace.ID, "s-unknown", http.StatusOK},
		{"unknown workspace", uuid.New().String(), "s-auto", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want,
				deleteAutoApprove(t, h, tc.workspaceID, tc.sessionID))
		})
	}
}
