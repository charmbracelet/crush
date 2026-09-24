package tools

import (
	"net/http"
	"time"

	"github.com/charmbracelet/crush/internal/httpretry"
)

// NewHTTPClient returns the client used by tools that talk to the network
// when the caller does not supply one. Its transport retries transient
// failures (a reset or refused connection, a server that hung up before
// answering, a 429/502/503/504) for replayable requests, so a single
// network blip never reaches the tool as an error. See [httpretry].
//
// The timeout bounds the whole exchange, retries included.
func NewHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 10
	transport.IdleConnTimeout = 90 * time.Second

	return &http.Client{
		Timeout:   timeout,
		Transport: httpretry.New(transport),
	}
}
