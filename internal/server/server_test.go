package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

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
