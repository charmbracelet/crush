package tools

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/httpretry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPClientRetriesAResetConnection(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			conn, _, err := w.(http.Hijacker).Hijack()
			require.NoError(t, err)
			if tcp, ok := conn.(*net.TCPConn); ok {
				_ = tcp.SetLinger(0)
			}
			_ = conn.Close()
			return
		}
		_, _ = io.WriteString(w, "deploy status: GREEN")
	}))
	t.Cleanup(srv.Close)

	client := NewHTTPClient(5 * time.Second)
	_, isRetrying := client.Transport.(*httpretry.Transport)
	assert.True(t, isRetrying, "tool clients must use the retrying transport")
	assert.Equal(t, 5*time.Second, client.Timeout)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, "deploy status: GREEN", string(body))
	assert.EqualValues(t, 2, calls.Load())
}
