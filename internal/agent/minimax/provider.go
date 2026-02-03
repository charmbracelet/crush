// Package minimax provides a catwalk.Provider for MiniMax Coding Plan models.
package minimax

import (
	"cmp"
	_ "embed"
	"encoding/json"
	"log/slog"
	"os"
	"strconv"
	"sync"

	"charm.land/catwalk/pkg/catwalk"
)

//go:embed provider.json
var embedded []byte

// Enabled returns true if minimax is enabled.
var Enabled = sync.OnceValue(func() bool {
	b, _ := strconv.ParseBool(
		cmp.Or(
			os.Getenv("MINIMAX"),
			os.Getenv("MINIMAX_ENABLE"),
			os.Getenv("MINIMAX_ENABLED"),
		),
	)
	return b
})

// Embedded returns the embedded MiniMax provider.
var Embedded = sync.OnceValue(func() catwalk.Provider {
	var provider catwalk.Provider
	if err := json.Unmarshal(embedded, &provider); err != nil {
		slog.Error("Could not use embedded MiniMax provider data", "err", err)
	}
	if e := os.Getenv("MINIMAX_URL"); e != "" {
		provider.APIEndpoint = e
	}
	return provider
})

const (
	// Name is the default name of this provider.
	Name = "minimax"
	// defaultBaseURL is the default MiniMax API URL.
	defaultBaseURL = "https://api.minimax.io/anthropic"
)

// BaseURL returns the base URL, which is either $MINIMAX_URL or the default.
var BaseURL = sync.OnceValue(func() string {
	return cmp.Or(os.Getenv("MINIMAX_URL"), defaultBaseURL)
})
