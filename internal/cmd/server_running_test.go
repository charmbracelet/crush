package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServerRunning(t *testing.T) {
	orig := clientHost
	t.Cleanup(func() { clientHost = orig })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	addr := strings.TrimPrefix(srv.URL, "http://")

	clientHost = "tcp://" + addr
	require.True(t, serverRunning(t.Context()))

	srv.Close()
	require.False(t, serverRunning(t.Context()))
}
