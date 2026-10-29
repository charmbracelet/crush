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
