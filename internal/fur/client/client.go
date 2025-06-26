// Package client provides a client for interacting with the fur service.
package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/charmbracelet/crush/internal/fur/provider"
)

const defaultURL = "http://localhost:8080"

// Client represents a client for the fur service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new client instance
// Uses FUR_URL environment variable or falls back to localhost:8080.
func New() *Client {
	baseURL := os.Getenv("FUR_URL")
	if baseURL == "" {
		baseURL = defaultURL
	}

	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// NewWithURL creates a new client with a specific URL.
func NewWithURL(url string) *Client {
	return &Client{
		baseURL:    url,
		httpClient: &http.Client{},
	}
}

// GetProviders retrieves all available providers from the service.
func (c *Client) GetProviders() ([]provider.Provider, error) {
	url := fmt.Sprintf("%s/providers", c.baseURL)

	resp, err := c.httpClient.Get(url) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var providers []provider.Provider
	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return providers, nil
}
