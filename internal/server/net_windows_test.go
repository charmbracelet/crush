//go:build windows

package server

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// TestListen_RestrictsPipeToOwner guards against a server pipe whose DACL
// lets users other than its owner (and LocalSystem) connect.
func TestListen_RestrictsPipeToOwner(t *testing.T) {
	t.Parallel()
	path := fmt.Sprintf(`\\.\pipe\crush-test-%d-%d.sock`, os.Getpid(), time.Now().UnixNano())
	ln, _, err := listen("npipe", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	// Clients can only open the pipe while an Accept is pending.
	go func() {
		if conn, err := ln.Accept(); err == nil {
			<-t.Context().Done()
			_ = conn.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	conn, err := winio.DialPipeContext(ctx, path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	sd, err := windows.GetSecurityInfo(windows.Handle(conn.(interface{ Fd() uintptr }).Fd()),
		windows.SE_KERNEL_OBJECT, windows.DACL_SECURITY_INFORMATION)
	require.NoError(t, err)
	dacl, _, err := sd.DACL()
	require.NoError(t, err)
	require.NotNil(t, dacl, "a NULL DACL grants everyone full access")
	var trustees []string
	for i := range uint32(dacl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		require.NoError(t, windows.GetAce(dacl, i, &ace))
		trustees = append(trustees, (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String())
	}
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	require.NoError(t, err)
	require.ElementsMatch(t, []string{tu.User.Sid.String(), "S-1-5-18"}, trustees)
}
