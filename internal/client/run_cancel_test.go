package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := NewClient("", "tcp", u.Host)
	require.NoError(t, err)
	return c
}

// WithRunBoundToClient gắn lượt vào client gửi: body mang client ID của nó;
// không đánh dấu thì không gắn.
func TestSendMessageBindsRunOnlyWhenAsked(t *testing.T) {
	t.Parallel()
	var (
		mu             sync.Mutex
		binds          []string
		nonInteractive []bool
	)
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		var msg proto.AgentMessage
		require.NoError(t, json.NewDecoder(r.Body).Decode(&msg))
		mu.Lock()
		binds = append(binds, msg.BindToClient)
		nonInteractive = append(nonInteractive, msg.NonInteractive)
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	})
	require.NoError(t, c.SendMessage(WithNonInteractiveRun(WithRunBoundToClient(t.Context())), "ws", "s", "r1", "", "p"))
	require.NoError(t, c.SendMessage(context.WithoutCancel(t.Context()), "ws", "s", "r2", "", "p"))
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{c.ClientID(), ""}, binds)
	require.Equal(t, []bool{true, false}, nonInteractive)
}

// Huỷ theo RunID đi đúng đường dẫn; server cũ (404) cho ErrCancelRunUnsupported.
func TestCancelAgentRun(t *testing.T) {
	t.Parallel()
	var path string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	require.NoError(t, c.CancelAgentRun(t.Context(), "ws", "s1", "run-1"))
	require.Equal(t, "/v1/workspaces/ws/agent/sessions/s1/runs/run-1/cancel", path)

	old := testClient(t, func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	require.ErrorIs(t, old.CancelAgentRun(t.Context(), "ws", "s1", "run-1"), ErrCancelRunUnsupported)
}

// ListJobs đi đúng đường dẫn; server cũ (404 không thân lỗi) cho lỗi nói rõ.
func TestListJobs(t *testing.T) {
	t.Parallel()
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/workspaces/ws/jobs", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":"01A","status":"running","command":"sleep 1","working_dir":"/p","started_at":"2026-10-15T00:00:00Z","exit_code":0,"background":true,"output_bytes":0}]`))
	})
	jobs, err := c.ListJobs(t.Context(), "ws")
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, "01A", jobs[0].ID)
	require.True(t, jobs[0].Background)

	old := testClient(t, func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	_, err = old.ListJobs(t.Context(), "ws")
	require.ErrorIs(t, err, ErrJobsUnsupported)
}
