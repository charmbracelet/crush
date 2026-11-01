package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateFetchURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		url string
		ok  bool
	}{
		{"http://93.184.216.34/", true},
		{"https://93.184.216.34:8443/path", true},
		{"http://127.0.0.1/", false},
		{"http://127.0.0.1:8080/admin", false},
		{"http://localhost/", false},
		{"http://10.0.0.5/", false},
		{"http://192.168.1.1/", false},
		{"http://172.16.0.9/", false},
		{"http://169.254.169.254/latest/meta-data/", false},
		{"http://[::1]/", false},
		{"http://0.0.0.0/", false},
		{"http://100.64.0.1/", false},
		{"ftp://93.184.216.34/", false},
		{"http://", false},
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()
			err := validateFetchURL(context.Background(), tc.url)
			if tc.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestFetchURLAndConvertRefusesPrivateTargets(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("secret"))
	}))
	defer server.Close()

	_, err := FetchURLAndConvert(context.Background(), server.Client(), server.URL)
	require.Error(t, err)
}
