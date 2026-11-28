package tools

import (
	"net/http"
	"sync"
	"time"
)

// sharedTransport is the connection pool behind every web tool that is not
// handed its own client. Tools are rebuilt on every turn, so a pool per
// construction would drop keep-alive connections between turns and leave the
// old pool's idle sockets open until they time out.
var sharedTransport = sync.OnceValue(func() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 10
	transport.IdleConnTimeout = 90 * time.Second
	return transport
})

// DefaultHTTPClient returns a client with the given timeout on the shared
// connection pool. Clients are cheap; the pool is what is worth keeping.
func DefaultHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: sharedTransport(),
	}
}
