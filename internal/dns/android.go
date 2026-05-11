// Package dns configures Go's DNS resolver for Termux/Android where
// Go's pure-Go resolver reads /etc/resolv.conf which points to
// non-functional loopback nameservers.
// The package uses runtime detection — no build tags required.
package dns

import (
	"context"
	"net"
	"os"
)

func init() {
	if os.Getenv("TERMUX_VERSION") == "" {
		return
	}

	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial:     dialWithFallback([]string{"8.8.8.8:53", "1.1.1.1:53"}),
	}
}

// dialWithFallback returns a resolver Dial func that tries each
// nameserver in order, falling through on failure.
func dialWithFallback(nameservers []string) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, _ string) (net.Conn, error) {
		var lastErr error
		d := net.Dialer{
			Resolver: nil,
		}
		for _, ns := range nameservers {
			conn, err := d.DialContext(ctx, network, ns)
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
}
