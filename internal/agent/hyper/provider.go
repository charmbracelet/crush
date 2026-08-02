// Package hyper provides a fantasy.Provider that proxies requests to Hyper.
package hyper

import (
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/catwalk/pkg/catwalk"
)

//go:generate wget -O provider.json https://hyper.charm.land/v1/provider

//go:embed provider.json
var embedded []byte

// Embedded returns the embedded Hyper provider.
var Embedded = sync.OnceValue(func() catwalk.Provider {
	var provider catwalk.Provider
	if err := json.Unmarshal(embedded, &provider); err != nil {
		slog.Error("Could not use embedded provider data", "err", err)
	}
	if e := os.Getenv("HYPER_URL"); e != "" {
		provider.APIEndpoint = e + "/api/v1/fantasy"
	}
	return provider
})

const (
	// Name is the default name of this meta provider.
	Name = "hyper"
	// DisplayName is the display name of Hyper.
	DisplayName = "Charm Hyper"
	// defaultBaseURL is the default proxy URL.
	defaultBaseURL = "https://hyper.charm.land"
)

// BaseURL returns the base URL, which is either $HYPER_URL or the default.
var BaseURL = sync.OnceValue(func() string {
	return cmp.Or(os.Getenv("HYPER_URL"), defaultBaseURL)
})

// lastKnownBalance stores the most recently extracted hypercredit balance
// from API response metadata. FetchCredits checks this before making a
// separate HTTP call.
var lastKnownBalance atomic.Int64

// hasBalance tracks whether lastKnownBalance has been set.
var hasBalance atomic.Bool

// SetBalance stores a credit balance extracted from API response metadata.
func SetBalance(balance int) {
	lastKnownBalance.Store(int64(balance))
	hasBalance.Store(true)
}

// FetchCredits returns the remaining hypercredit balance. It first checks
// for a balance extracted from the most recent API response's usage
// metadata. If none is available, it falls back to calling the /v1/credits
// endpoint directly.
func FetchCredits(ctx context.Context, apiKey string) (int, error) {
	if hasBalance.Load() {
		hasBalance.Store(false)
		return int(lastKnownBalance.Load()), nil
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		BaseURL()+"/v1/credits",
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Balance int `json:"balance"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Balance, nil
}
