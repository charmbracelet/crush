//go:build windows

package shell

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestDetachedSysProcAttr(t *testing.T) {
	t.Parallel()

	attr := detachedSysProcAttr(true)
	require.NotNil(t, attr)
	require.True(t, attr.HideWindow)
	require.NotZero(t, attr.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP)
	require.NotZero(t, attr.CreationFlags&windows.CREATE_NO_WINDOW)
}
