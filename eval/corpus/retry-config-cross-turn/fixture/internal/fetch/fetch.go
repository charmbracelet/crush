// Package fetch orchestrates config + client for the CLI.
package fetch

import (
	"fmt"
	"net/http"

	"fetcher/internal/client"
	"fetcher/internal/config"
)

// Fetcher bundles the loaded config and its HTTP client.
type Fetcher struct {
	cfg config.Config
	cli *client.Client
}

// New builds a Fetcher.
func New(cfg config.Config) *Fetcher {
	return &Fetcher{cfg: cfg, cli: client.New(cfg)}
}

// Endpoint returns the configured base URL.
func (f *Fetcher) Endpoint() string {
	return f.cfg.Endpoint
}

// Get issues a GET through the underlying client.
func (f *Fetcher) Get(path string) (*http.Response, error) {
	resp, err := f.cli.Get(path)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", path, err)
	}
	return resp, nil
}
