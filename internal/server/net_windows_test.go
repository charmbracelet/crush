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

func TestListen_RestrictsPipeToOwner(t *testing.T) {
	t.Parallel()

	path := fmt.Sprintf(`\\.\pipe\crush-test-%d-%d.sock`, os.Getpid(), time.Now().UnixNano())

	ln, _, err := listen("npipe", path)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = ln.Close()
	})

	// Clients can only open the pipe while an Accept is pending. Keep the
	// server end open until the test is done with the client end.
	done := make(chan struct{})
	t.Cleanup(func() {
		close(done)
	})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		<-done
		_ = conn.Close()
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	conn, err := winio.DialPipeContext(ctx, path)
	require.NoError(t, err, "owner should be able to connect")
	t.Cleanup(func() {
		_ = conn.Close()
	})

	handle, ok := conn.(interface{ Fd() uintptr })
	require.True(t, ok, "expected pipe conn to expose its handle, got %T", conn)
	sd, err := windows.GetSecurityInfo(
		windows.Handle(handle.Fd()),
		windows.SE_KERNEL_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	require.NoError(t, err)

	tokenUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	require.NoError(t, err)
	user := tokenUser.User.Sid

	owner, _, err := sd.Owner()
	require.NoError(t, err)
	require.True(t, owner.Equals(user), "pipe owner is %s, want %s", owner, user)

	dacl, _, err := sd.DACL()
	require.NoError(t, err)
	require.NotNil(t, dacl, "a NULL DACL grants everyone full access")
	var trustees []string
	for i := range uint32(dacl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		require.NoError(t, windows.GetAce(dacl, i, &ace))
		require.EqualValues(t, windows.ACCESS_ALLOWED_ACE_TYPE, ace.Header.AceType)
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		trustees = append(trustees, sid.String())
	}
	require.ElementsMatch(t, []string{user.String(), "S-1-5-18"}, trustees,
		"pipe DACL must only grant its owner and LocalSystem access, got %s", sd)
}
