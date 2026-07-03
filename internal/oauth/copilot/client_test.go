package copilot

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitiatorTransportSetsHeader(t *testing.T) {
	tests := []struct {
		name string
		// body builds the request body; nil means a bodyless request.
		body func() (int, string)
		want string
	}{
		{
			name: "nil body defaults to user",
			body: func() (int, string) { return 0, "" }, // req.Body == nil
			want: "user",
		},
		{
			name: "NoBody defaults to user",
			body: func() (int, string) { return -1, "" }, // req.Body == http.NoBody
			want: "user",
		},
		{
			name: "user-only history stays user",
			body: func() (int, string) { return 1, `{"messages":[{"role":"user"}]}` },
			want: "user",
		},
		{
			name: "assistant history becomes agent",
			body: func() (int, string) { return 1, `{"messages":[{"role":"assistant"}]}` },
			want: "agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Get("X-Initiator")
			}))
			defer srv.Close()

			kind, payload := tt.body()
			var req *http.Request
			var err error
			switch kind {
			case 0:
				req, err = http.NewRequest(http.MethodGet, srv.URL, nil)
			case -1:
				req, err = http.NewRequest(http.MethodGet, srv.URL, http.NoBody)
			default:
				req, err = http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(payload))
			}
			require.NoError(t, err)

			client := &http.Client{Transport: &initiatorTransport{}}
			resp, err := client.Do(req)
			require.NoError(t, err)
			resp.Body.Close()

			require.Equal(t, tt.want, got)
		})
	}
}
