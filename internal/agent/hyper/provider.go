// Package hyper provides a fantasy.Provider that proxies requests to Hyper.
package hyper

import (
	"cmp"
	_ "embed"
	"encoding/json"
	"log/slog"
	"os"
	"sync"

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
	// defaultBaseURL is the default proxy URL.
	defaultBaseURL = "https://hyper.charm.land"
)

// BaseURL returns the base URL, which is either $HYPER_URL or the default.
var BaseURL = sync.OnceValue(func() string {
	return cmp.Or(os.Getenv("HYPER_URL"), defaultBaseURL)
})
