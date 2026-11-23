//go:build windows
// +build windows

package server

import (
	"fmt"
	"net"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// listen binds a net.Listener on the given network and address.
//
// On Windows there is no Unix-socket stale-file recovery to perform,
// so removedStale is always false. The signature matches the
// non-Windows implementation so callers can use a single code path.
func listen(network, address string) (net.Listener, bool, error) {
	switch network {
	case "npipe":
		sd, err := pipeSecurityDescriptor()
		if err != nil {
			return nil, false, err
		}
		cfg := &winio.PipeConfig{
			SecurityDescriptor: sd,
			MessageMode:        true,
			InputBufferSize:    65536,
			OutputBufferSize:   65536,
		}
		ln, err := winio.ListenPipe(address, cfg)
		if err != nil {
			return nil, false, err
		}
		return ln, false, nil
	default:
		ln, err := net.Listen(network, address) //nolint:noctx
		if err != nil {
			return nil, false, err
		}
		return ln, false, nil
	}
}

// pipeSecurityDescriptor returns the SDDL for the server's named pipe. The
// pipe is owned by the user running the server, and its protected DACL
// grants full access (GA) only to that user and LocalSystem (SY).
//
// The default descriptor instead grants full access to LocalSystem,
// Administrators and the token's default owner, and read access to
// Everyone and Anonymous. For an elevated server the default owner is
// Administrators, so the same user's non-elevated clients could not
// connect.
func pipeSecurityDescriptor() (string, error) {
	tokenUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("failed to get current process user: %w", err)
	}
	sid := tokenUser.User.Sid.String()
	return fmt.Sprintf("O:%sD:P(A;;GA;;;%s)(A;;GA;;;SY)", sid, sid), nil
}
