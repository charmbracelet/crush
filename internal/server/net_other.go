//go:build !windows
// +build !windows

package server

import "net"

func listen(network, address string) (net.Listener, error) {
	return net.Listen(network, address)
}
