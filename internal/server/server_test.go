package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseHostURL(t *testing.T) {
	t.Parallel()

	// A path without a drive letter is not absolute on Windows, so it is
	// resolved against the current drive.
	unixSock, err := filepath.Abs("/tmp/crush.sock")
	require.NoError(t, err)

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
		{name: "unix", host: "unix:///tmp/crush.sock", scheme: "unix", addr: unixSock},
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

// TestParseHostURL_RelativeUnixSocketIsAbsolute checks that a relative
// unix socket path is resolved against the directory it was parsed in.
// The client parses --host before changing into --cwd, so a path left
// relative would be dialed in the project directory rather than where the
// server it spawned is listening.
func TestParseHostURL_RelativeUnixSocketIsAbsolute(t *testing.T) {
	t.Chdir(t.TempDir())
	wd, err := os.Getwd()
	require.NoError(t, err)

	cases := []struct {
		name string
		host string
		want string
	}{
		{name: "relative", host: "unix://rel.sock", want: filepath.Join(wd, "rel.sock")},
		{name: "relative subdir", host: "unix://sub/rel.sock", want: filepath.Join(wd, "sub", "rel.sock")},
		{name: "absolute", host: "unix://" + filepath.Join(wd, "abs.sock"), want: filepath.Join(wd, "abs.sock")},
		// On Linux a leading '@' names an abstract socket, not a path.
		{name: "abstract", host: "unix://@abstract", want: "@abstract"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hostURL, err := ParseHostURL(tc.host)
			require.NoError(t, err)
			require.Equal(t, "unix", hostURL.Scheme)
			require.Equal(t, tc.want, hostURL.Host)
		})
	}
}
