//go:build !windows
// +build !windows

package client

import (
	"context"
	"net"
	"syscall"
)

func dialPipeContext(context.Context, string) (net.Conn, error) {
	return nil, syscall.EAFNOSUPPORT
}
