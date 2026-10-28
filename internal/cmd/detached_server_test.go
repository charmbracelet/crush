package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDetachedServerCmd_RunsFromServerDir guards against the spawned server
// inheriting the working directory of the client that starts it. The server
// outlives that client, so the directory would stay in use for as long as
// the server runs.
func TestDetachedServerCmd_RunsFromServerDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c := detachedServerCmd("crush", dir, "server")
	require.Equal(t, dir, c.Dir,
		"the server must not inherit the spawning client's working directory")
}
