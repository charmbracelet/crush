// Package deepseek provides native DeepSeek provider support.
package deepseek

import (
	"cmp"
	_ "embed"
	"encoding/json"
	"log/slog"
	"os"
	"sync"

	"charm.land/catwalk/pkg/catwalk"
)

//go:embed provider.json
var embedded []byte

// Embedded returns the embedded DeepSeek provider.
var Embedded = sync.OnceValue(func() catwalk.Provider {
	var provider catwalk.Provider
	if err := json.Unmarshal(embedded, &provider); err != nil {
		slog.Error("Could not use embedded provider data", "err", err)
	}
	return provider
})

const (
	// Name is the name of the provider.
	Name = "deepseek"
	// DisplayName is the display name.
	DisplayName = "DeepSeek"
)

// BaseURL returns the base URL, which is either $DEEPSEEK_URL or the default.
var BaseURL = sync.OnceValue(func() string {
	return cmp.Or(os.Getenv("DEEPSEEK_URL"), "https://api.deepseek.com")
})
