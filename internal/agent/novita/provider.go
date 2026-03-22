// Package novita provides the Novita AI provider configuration.
package novita

import (
	_ "embed"
	"encoding/json"
	"log/slog"
	"os"
	"sync"

	"charm.land/catwalk/pkg/catwalk"
)

//go:embed provider.json
var embedded []byte

// Enabled returns true if Novita is enabled (NOVITA_API_KEY is set).
var Enabled = sync.OnceValue(func() bool {
	return os.Getenv("NOVITA_API_KEY") != ""
})

// Embedded returns the embedded Novita provider definition.
var Embedded = sync.OnceValue(func() catwalk.Provider {
	var provider catwalk.Provider
	if err := json.Unmarshal(embedded, &provider); err != nil {
		slog.Error("Could not use embedded Novita provider data", "err", err)
	}
	return provider
})

const (
	// Name is the provider ID.
	Name = "novita"
)