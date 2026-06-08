//go:build windows
// +build windows

package server

import (
	"net"

	"github.com/Microsoft/go-winio"
)

// listen binds a net.Listener on the given network and address.
//
// On Windows there is no Unix-socket stale-file recovery to perform,
// so removedStale is always false. The signature matches the
// non-Windows implementation so callers can use a single code path.
func listen(network, address string) (net.Listener, bool, error) {
	switch network {
	case "npipe":
		cfg := &winio.PipeConfig{
			MessageMode:      true,
			InputBufferSize:  65536,
			OutputBufferSize: 65536,
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
