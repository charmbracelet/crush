package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/hyper"
)

type realHyperClient struct {
	baseURL      string
	resolveKey   func() string
	refreshToken func(context.Context) error
	// managed, when non-nil and true, means the credential is owned by an
	// external process: a 401 is reported as is, never refreshed.
	managed func() bool
}

// Get fetches the Hyper provider, refreshing the token and retrying once on
// a 401.
func (r realHyperClient) Get(ctx context.Context, etag string) (catwalk.Provider, []byte, error) {
	result, raw, err := r.doGet(ctx, etag)
	if err != nil && isHTTPUnauthorized(err) && r.managed != nil && r.managed() {
		slog.Info("Received 401 fetching Hyper provider with a managed credential; not refreshing")
		return result, raw, err
	}
	if err != nil && isHTTPUnauthorized(err) && r.refreshToken != nil {
		slog.Info("Received 401 fetching Hyper provider, refreshing token and retrying")
		if refreshErr := r.refreshToken(ctx); refreshErr != nil {
			slog.Warn("Failed to refresh Hyper token", "error", refreshErr)
			return result, raw, err
		}
		result, raw, err = r.doGet(ctx, "")
	}
	return result, raw, err
}

func (r realHyperClient) doGet(ctx context.Context, etag string) (catwalk.Provider, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/v1/provider/v3", nil)
	if err != nil {
		return catwalk.Provider{}, nil, fmt.Errorf("could not create request: %w", err)
	}
	if apiKey := r.resolveKey(); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return getWithETag[catwalk.Provider](&http.Client{Timeout: 30 * time.Second}, req, etag)
}

func embeddedHyper() catwalk.Provider { return hyper.Embedded() }

// errUnauthorized is a sentinel for HTTP 401 responses from the Hyper API.
var errUnauthorized = errors.New("unauthorized")

// isHTTPUnauthorized reports whether err is or wraps errUnauthorized.
func isHTTPUnauthorized(err error) bool {
	return errors.Is(err, errUnauthorized)
}
