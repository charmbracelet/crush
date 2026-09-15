// Package client is the outbound HTTP client for the upstream API.
package client

import (
	"fmt"
	"net/http"
	"time"

	"taskapi/internal/config"
)

// Client wraps http.Client with the configured defaults.
type Client struct {
	cfg config.Config
	hc  *http.Client
}

// New builds a Client honoring the configured timeout.
func New(cfg config.Config) *Client {
	return &Client{
		cfg: cfg,
		hc: &http.Client{
			Timeout: time.Duration(cfg.TimeoutMS) * time.Second,
		},
	}
}

// Timeout reports the configured request timeout.
func (c *Client) Timeout() time.Duration {
	return c.hc.Timeout
}

// Endpoint returns the upstream base URL.
func (c *Client) Endpoint() string {
	return c.cfg.Endpoint
}

// Get issues a GET against the upstream API.
func (c *Client) Get(path string) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.cfg.Endpoint, path)
	resp, err := c.hc.Get(url)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	return resp, nil
}

// Head issues a HEAD against the upstream API — used for cheap
// liveness probes that do not need a body.
func (c *Client) Head(path string) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.cfg.Endpoint, path)
	resp, err := c.hc.Head(url)
	if err != nil {
		return nil, fmt.Errorf("head %s: %w", url, err)
	}
	return resp, nil
}
