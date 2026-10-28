//go:build windows

package server

import (
	"os/user"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultHost_SID(t *testing.T) {
	// Pipe names are machine-wide, so the default pipe must be named after
	// the SID of the user that owns the process, even when user.Current
	// fails because it cannot also look up the profile directory or the
	// account name.
	token, err := syscall.OpenCurrentProcessToken()
	require.NoError(t, err)
	defer token.Close()
	tu, err := token.GetTokenUser()
	require.NoError(t, err)
	sid, err := tu.User.Sid.String()
	require.NoError(t, err)

	require.Equal(t, "npipe:////./pipe/crush-"+sid+".sock", DefaultHost())

	// Servers started by older versions are named after the Uid that
	// user.Current reports, so clients must keep finding them.
	if usr, err := user.Current(); err == nil {
		require.Equal(t, "npipe:////./pipe/crush-"+usr.Uid+".sock", DefaultHost())
	}
}
