package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseHostURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		host     string
		scheme   string
		addr     string
		basePath string
		wantErr  bool
	}{
		{name: "tcp", host: "tcp://127.0.0.1:8080", scheme: "tcp", addr: "127.0.0.1:8080"},
		{name: "tcp with base path", host: "tcp://127.0.0.1:8080/api", scheme: "tcp", addr: "127.0.0.1:8080", basePath: "/api"},
		{name: "http aliases tcp", host: "http://127.0.0.1:8080", scheme: "tcp", addr: "127.0.0.1:8080"},
		{name: "http with base path", host: "http://127.0.0.1:8080/api", scheme: "tcp", addr: "127.0.0.1:8080", basePath: "/api"},
		{name: "unix", host: "unix:///tmp/crush.sock", scheme: "unix", addr: "/tmp/crush.sock"},
		{name: "missing separator", host: "127.0.0.1:8080", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			u, err := ParseHostURL(tc.host)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.scheme, u.Scheme)
			require.Equal(t, tc.addr, u.Host)
			require.Equal(t, tc.basePath, u.Path)
		})
	}
}

// TestParseHostURL_RelativeUnixSocketIsAbsolute guards against a relative
// unix --host being dialed relative to --cwd instead of where it was given.
func TestParseHostURL_RelativeUnixSocketIsAbsolute(t *testing.T) {
	t.Chdir(t.TempDir())
	wd, err := os.Getwd()
	require.NoError(t, err)

	for host, want := range map[string]string{
		"unix://sub/rel.sock": filepath.Join(wd, "sub", "rel.sock"),
		"unix://@abstract":    "@abstract",
	} {
		hostURL, err := ParseHostURL(host)
		require.NoError(t, err)
		require.Equal(t, want, hostURL.Host)
	}
}
