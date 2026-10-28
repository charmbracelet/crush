//go:build windows
// +build windows

package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// currentUser returns the SID the pipe server must run as. Tests replace it
// to pretend the server belongs to another user.
var currentUser = func() (*windows.SID, error) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	return u.User.Sid, nil
}

func dialPipeContext(ctx context.Context, address string) (net.Conn, error) {
	if err := verifyPipeServer(ctx, address); err != nil {
		return nil, err
	}
	return winio.DialPipeContext(ctx, address)
}

// verifyPipeServer fails unless the process serving the named pipe at
// address runs as the current user. Pipe names share one machine-wide
// namespace, so another user can create ours before our server does.
//
// The check runs on a short-lived connection of its own: the handle behind
// go-winio's connection is overlapped and bound to go-winio's completion
// port, which must not see I/O that go-winio did not issue. The pipe can
// only change hands between the two connections if our server exits and
// another user takes the name in that instant.
func verifyPipeServer(ctx context.Context, address string) error {
	h, err := openPipe(ctx, address)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)

	var pid uint32
	if err := windows.GetNamedPipeServerProcessId(h, &pid); err != nil {
		return fmt.Errorf("failed to verify the owner of named pipe %q: %w", address, err)
	}
	ours, err := runsAsCurrentUser(pid)
	if err != nil {
		return fmt.Errorf("failed to verify the owner of named pipe %q: %w", address, err)
	}
	if !ours {
		return fmt.Errorf("named pipe %q is owned by another user", address)
	}
	return nil
}

// openPipe connects to the named pipe at address, retrying while all of
// its instances are busy. The server may not impersonate the connection.
func openPipe(ctx context.Context, address string) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(address)
	if err != nil {
		return 0, err
	}
	for {
		h, err := windows.CreateFile(name, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING,
			windows.SECURITY_SQOS_PRESENT|windows.SECURITY_ANONYMOUS, 0)
		if err == nil {
			return h, nil
		}
		if !errors.Is(err, windows.ERROR_PIPE_BUSY) {
			return 0, &os.PathError{Op: "open", Path: address, Err: err}
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// runsAsCurrentUser reports whether the process with the given ID runs as
// the current user.
func runsAsCurrentUser(pid uint32) (bool, error) {
	p, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(p)

	var token windows.Token
	if err := windows.OpenProcessToken(p, windows.TOKEN_QUERY, &token); err != nil {
		return false, err
	}
	defer token.Close()

	server, err := token.GetTokenUser()
	if err != nil {
		return false, err
	}
	me, err := currentUser()
	if err != nil {
		return false, err
	}
	return server.User.Sid.Equals(me), nil
}

// verifyUnixPeer accepts any server: on Windows a unix socket is only
// used when --host names one, and like tcp it is not verified.
func verifyUnixPeer(net.Conn, string) error {
	return nil
}
