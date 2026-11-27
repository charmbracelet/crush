package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDetachedServerCmd_RunsFromServerDir guards the long-lived server
// inheriting, and so pinning, the spawning client's working directory.
func TestDetachedServerCmd_RunsFromServerDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.Equal(t, dir, detachedServerCmd("crush", dir, "server").Dir)
}

// TestDetachedServerCmd_MarksItSpawned guards the marker that tells a
// spawned server to keep its idle shutdown, so it cannot outlive the
// client that started it. A server the user runs explicitly has no such
// marker and stays up when idle.
func TestDetachedServerCmd_MarksItSpawned(t *testing.T) {
	t.Setenv(spawnedServerEnv, "")

	c := detachedServerCmd("crush", t.TempDir(), "server")
	require.Contains(t, c.Env, spawnedServerEnv+"=1",
		"a spawned server must be marked so it still shuts down when idle")
}
