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
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy/providers/openai"
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

// Prism router headers, present on responses served through a Prism model
// (model router). They report which model actually answered the request.
const (
	// PrismModelIDHeader is the header carrying the routed model ID.
	PrismModelIDHeader = "X-Prism-Model-Id"
	// PrismModelNameHeader is the header carrying the routed model name.
	PrismModelNameHeader = "X-Prism-Model-Name"
)

// Prism savings headers. They are only known once the request has been
// served, so on streaming responses they arrive as HTTP trailers.
const (
	// PrismHypercreditSavingsHeader is the header (or trailer) carrying
	// the hypercredits saved by routing through Prism.
	PrismHypercreditSavingsHeader = "X-Prism-Hypercredit-Savings"
	// PrismDollarSavingsHeader is the header (or trailer) carrying the
	// dollars saved by routing through Prism.
	PrismDollarSavingsHeader = "X-Prism-Dollar-Savings"
)

// Keys under which the Prism router headers are stored in
// [openai.ProviderMetadata.ExtraFields].
const (
	// PrismModelIDField is the extra field for the routed model ID.
	PrismModelIDField = "x-prism-model-id"
	// PrismModelNameField is the extra field for the routed model name.
	PrismModelNameField = "x-prism-model-name"
	// PrismHypercreditSavingsField is the extra field for the
	// hypercredits saved by routing through Prism.
	PrismHypercreditSavingsField = "x-prism-hypercredit-savings"
	// PrismDollarSavingsField is the extra field for the dollars saved
	// by routing through Prism.
	PrismDollarSavingsField = "x-prism-dollar-savings"
)

// HeaderFunc copies the Prism router headers and trailers into the openai
// provider metadata. It is wired as the openai-compat language model header
// func so turns served through a Prism model report which model actually
// answered and how much was saved.
func HeaderFunc(header http.Header, metadata *openai.ProviderMetadata) {
	copyHeaderField(header, metadata, PrismModelIDHeader, PrismModelIDField)
	copyHeaderField(header, metadata, PrismModelNameHeader, PrismModelNameField)
	copyHeaderField(header, metadata, PrismHypercreditSavingsHeader, PrismHypercreditSavingsField)
	copyHeaderField(header, metadata, PrismDollarSavingsHeader, PrismDollarSavingsField)
}

func copyHeaderField(header http.Header, metadata *openai.ProviderMetadata, headerName, fieldName string) {
	value := header.Get(headerName)
	if value == "" {
		return
	}
	if metadata.ExtraFields == nil {
		metadata.ExtraFields = make(map[string]json.RawMessage)
	}
	metadata.ExtraFields[fieldName] = json.RawMessage(strconv.Quote(value))
}

// balanceValue stores the hypercredit balance most recently fetched from
// the /v1/credits endpoint; balanceKnown tracks whether any fetch has
// reported one.
var (
	balanceValue atomic.Int64
	balanceKnown atomic.Bool
)

// setBalance stores the hypercredit balance reported by the /v1/credits
// endpoint.
func setBalance(balance int) {
	balanceValue.Store(int64(balance))
	balanceKnown.Store(true)
}

// clearBalance forgets the stored balance, so a team that turns
// hypercredit display off stops showing a stale figure.
func clearBalance() {
	balanceKnown.Store(false)
}

// Balance returns the hypercredit balance most recently fetched from the
// /v1/credits endpoint, or nil when no fetch has reported one yet. That
// includes teams with hypercredit display disabled: their balance is
// reported in dollars instead, so there is never a hypercredit figure to
// show.
func Balance() *int {
	if !balanceKnown.Load() {
		return nil
	}
	balance := int(balanceValue.Load())
	return &balance
}

// FetchCredits fetches the remaining hypercredit balance from the
// /v1/credits endpoint and stores it for [Balance] to return. It returns
// nil when the team has hypercredit display disabled: Hyper reports the
// balance in dollars for them, so there is no hypercredit figure to show.
func FetchCredits(ctx context.Context, apiKey string) (*int, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		BaseURL()+"/v1/credits",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Teams with hypercredit display disabled get a balance_usd field
	// instead of balance, and no balance is shown for them at all.
	var result struct {
		Balance *int `json:"balance"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Balance == nil {
		clearBalance()
		return nil, nil
	}
	setBalance(*result.Balance)
	return result.Balance, nil
}
