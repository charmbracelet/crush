package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

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
