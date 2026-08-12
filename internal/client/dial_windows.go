//go:build windows
// +build windows

package client

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

func dialPipeContext(ctx context.Context, address string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, address)
}
