//go:build windows
// +build windows

package server

import (
	"net"

	"github.com/Microsoft/go-winio"
)

func listen(network, address string) (net.Listener, error) {
	switch network {
	case "npipe":
		cfg := &winio.PipeConfig{
			MessageMode:      true,
			InputBufferSize:  65536,
			OutputBufferSize: 65536,
		}
		return winio.ListenPipe(address, cfg)
	default:
		return net.Listen(network, address) //nolint:noctx
	}
}
