//go:build windows
// +build windows

package server

import (
	"net"
	"strings"

	"github.com/Microsoft/go-winio"
)

func listen(network, address string) (net.Listener, error) {
	if !strings.HasPrefix(address, "tcp") {
		return winio.ListenPipe(address, nil)
	}
	return net.Listen(network, address)
}
