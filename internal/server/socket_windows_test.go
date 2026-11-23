//go:build windows

package server

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// TestDefaultHost_SID guards against users sharing the machine-wide default
// pipe: it must be named after the process owner's SID.
func TestDefaultHost_SID(t *testing.T) {
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	require.NoError(t, err)
	require.Equal(t, "npipe:////./pipe/crush-"+tu.User.Sid.String()+".sock", DefaultHost())
}
