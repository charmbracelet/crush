package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUseClientServerDefaultOn(t *testing.T) {
	t.Setenv("CRUSH_CLIENT_SERVER", "1")
	require.True(t, useClientServer())

	t.Setenv("CRUSH_CLIENT_SERVER", "true")
	require.True(t, useClientServer())

	t.Setenv("CRUSH_CLIENT_SERVER", "0")
	require.False(t, useClientServer())

	t.Setenv("CRUSH_CLIENT_SERVER", "false")
	require.False(t, useClientServer())

	// Unset → default on.
	require.NoError(t, os.Unsetenv("CRUSH_CLIENT_SERVER"))
	require.True(t, useClientServer())
}
