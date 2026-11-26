package config

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/catwalk/v3/pkg/embedded"
)

// catwalkClient fetches the provider catalog. Unlike catwalk.Client it also
// returns the raw body, which the cache keeps so its ETag matches Catwalk's.
type catwalkClient struct {
	url    string
	client *http.Client
}

func newCatwalkClient(url string) catwalkClient {
	return catwalkClient{url: url, client: &http.Client{Timeout: 30 * time.Second}}
}

func (c catwalkClient) GetProviders(ctx context.Context, etag string) ([]catwalk.Provider, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+"/v3/providers", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create request: %w", err)
	}
	return getWithETag[[]catwalk.Provider](c.client, req, etag)
}

func embeddedCatwalk() []catwalk.Provider { return embedded.GetAll() }
