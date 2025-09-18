//go:build windows
// +build windows

package server

import (
	"net"
	"strings"

	"gopkg.in/natefinch/npipe.v2"
)

func listen(network, address string) (net.Listener, error) {
	if !strings.HasPrefix(address, "tcp") {
		return npipe.Listen(address)
	}
	return net.Listen(network, address)
}
