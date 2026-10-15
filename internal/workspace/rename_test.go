package workspace

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

// renameServer giả lập server: oldServer thì không có endpoint đổi tên (404
// không thân lỗi), missing thì session không tồn tại (404 có thân lỗi).
func renameServer(t *testing.T, oldServer, missing bool) (*ClientWorkspace, *[]string) {
	t.Helper()
	var (
		mu    sync.Mutex
		calls []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()
		switch {
		case r.URL.Path == "/v1/workspaces/ws-1/sessions/s1/rename":
			if missing {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(proto.Error{Message: "session not found"})
				return
			}
			if oldServer {
				http.NotFound(w, r)
				return
			}
			var body proto.SessionRename
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "new title", body.Title)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/sessions/s1":
			_ = json.NewEncoder(w).Encode(proto.Session{ID: "s1", Title: "old", Cost: 1.5})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/workspaces/ws-1/sessions/s1":
			var s proto.Session
			require.NoError(t, json.NewDecoder(r.Body).Decode(&s))
			require.Equal(t, "new title", s.Title)
			_ = json.NewEncoder(w).Encode(s)
		}
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)
	return NewClientWorkspace(c, proto.Workspace{ID: "ws-1"}), &calls
}

// Đổi tên đi qua endpoint riêng (chỉ đổi tiêu đề), không đọc-rồi-ghi cả hàng.
func TestRenameSessionUsesAtomicEndpoint(t *testing.T) {
	t.Parallel()
	ws, calls := renameServer(t, false, false)
	require.NoError(t, ws.RenameSession(t.Context(), "s1", "new title"))
	require.Equal(t, []string{"POST /v1/workspaces/ws-1/sessions/s1/rename"}, *calls)
}

// Server cũ không có endpoint: lùi về đường cũ (GET rồi PUT).
func TestRenameSessionFallsBackOnOldServer(t *testing.T) {
	t.Parallel()
	ws, calls := renameServer(t, true, false)
	require.NoError(t, ws.RenameSession(t.Context(), "s1", "new title"))
	require.Equal(t, []string{
		"POST /v1/workspaces/ws-1/sessions/s1/rename",
		"GET /v1/workspaces/ws-1/sessions/s1",
		"PUT /v1/workspaces/ws-1/sessions/s1",
	}, *calls)
}

// 404 có thân lỗi là session không có, không phải server cũ: báo lỗi, không lùi.
func TestRenameSessionMissingSessionIsAnError(t *testing.T) {
	t.Parallel()
	ws, calls := renameServer(t, false, true)
	require.ErrorContains(t, ws.RenameSession(t.Context(), "s1", "new title"), "session not found")
	require.Len(t, *calls, 1)
}
