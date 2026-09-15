// Package client issues HTTP requests against the configured
// endpoint.
package client

import (
	"fmt"
	"net/http"
	"time"

	"fetcher/internal/config"
)

// Client wraps http.Client with config defaults.
type Client struct {
	cfg config.Config
	hc  *http.Client
}

// New builds a Client from the loaded config.
func New(cfg config.Config) *Client {
	return &Client{
		cfg: cfg,
		hc:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Do issues req once and returns the response. Callers that need
// retries must loop themselves — there is no retry support yet.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do %s %s: %w", req.Method, req.URL, err)
	}
	return resp, nil
}

// Get issues a GET against endpoint+path.
func (c *Client) Get(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.cfg.Endpoint+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	return c.Do(req)
}
