package server

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCrossOriginHandler guards the TCP API against DNS rebinding and
// cross-site browser requests while leaving local clients and sockets alone.
func TestCrossOriginHandler(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, network, host string
		header              http.Header
		want                int
	}{
		{"dns rebinding", "tcp", "rebind.attacker.example:4141", http.Header{"Sec-Fetch-Site": {"same-origin"}}, http.StatusForbidden},
		{"cross-site origin", "tcp", "127.0.0.1:4141", http.Header{"Origin": {"https://evil.example"}}, http.StatusForbidden},
		{"cross-site metadata", "tcp", "127.0.0.1:4141", http.Header{"Sec-Fetch-Site": {"cross-site"}}, http.StatusForbidden},
		{"listen host", "tcp", "devbox.example:4141", nil, http.StatusOK},
		{"ip literal", "tcp", "[::1]:4141", nil, http.StatusOK},
		{"unix socket", "unix", "%2Ftmp%2Fcrush-test.sock", nil, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := NewServer(nil, tc.network, "devbox.example:4141")
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/health", nil)
			req.Host = tc.host
			maps.Copy(req.Header, tc.header)
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)
			require.Equal(t, tc.want, rec.Code)
		})
	}
}
