package httpx

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// The shared pool speaks HTTP/1.1 to a server offering HTTP/2, and is one pool.
func TestHTTP1Transport(t *testing.T) {
	t.Parallel()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _ = io.WriteString(w, req.Proto)
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	t.Cleanup(server.Close)

	require.Same(t, HTTP1Transport(), HTTP1Transport())
	require.True(t, http.DefaultTransport.(*http.Transport).Protocols == nil, "the default transport is not modified")

	// A clone keeps the protocol settings and only adds trust in the test certificate.
	tr := HTTP1Transport().Clone()
	tr.TLSClientConfig.RootCAs = server.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	resp, err := (&http.Client{Transport: tr}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "HTTP/1.1", string(body))
}
