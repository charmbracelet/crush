package config

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	ollamaapi "github.com/ollama/ollama/api"
	"github.com/ollama/ollama/envconfig"
)

const (
	ollamaProviderID   = "ollama"
	ollamaProviderName = "Ollama"
	ollamaDefaultHost  = "http://localhost:11434"
	// Ollama exposes an OpenAI-compatible endpoint at /v1.
	ollamaOpenAIPath = "/v1/"

	// ollamaTimeout is the maximum time to wait for Ollama to respond
	// during auto-detection. Kept short so startup is not noticeably
	// delayed when Ollama is not running.
	ollamaTimeout = 2 * time.Second
)

// ollamaBaseURL returns the Ollama host, respecting the OLLAMA_HOST
// environment variable that Ollama itself honours.
func ollamaBaseURL() string {
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		return host
	}
	return ollamaDefaultHost
}

// newOllamaClient creates an official Ollama API client using the
// environment-configured host (via OLLAMA_HOST) and a short-timeout
// HTTP client so startup is not noticeably delayed.
func newOllamaClient() *ollamaapi.Client {
	return ollamaapi.NewClient(envconfig.Host(), &http.Client{Timeout: ollamaTimeout})
}

// discoverOllamaModels queries a running Ollama instance and returns
// the models it has available. If Ollama is unreachable or returns an
// error, it returns nil and a non-nil error.
func discoverOllamaModels(ctx context.Context) ([]catwalk.Model, error) {
	resp, err := newOllamaClient().List(ctx)
	if err != nil {
		return nil, err
	}

	if len(resp.Models) == 0 {
		return nil, nil
	}

	models := make([]catwalk.Model, 0, len(resp.Models))
	for _, m := range resp.Models {
		id := m.Name
		if id == "" {
			id = m.Model
		}
		if id == "" {
			continue
		}
		models = append(models, catwalk.Model{
			ID:               id,
			Name:             id,
			DefaultMaxTokens: 8192,
		})
	}
	return models, nil
}

// maybeAutoDetectOllama attempts to discover a running Ollama instance and
// register it as a provider. It is a no-op when:
//   - the user already configured an "ollama" provider in crush.json
//   - the DisableOllamaAutoDetect option is set
//   - Ollama is not reachable or has no models
func maybeAutoDetectOllama(cfg *Config) {
	if cfg.Options != nil && cfg.Options.DisableOllamaAutoDetect {
		slog.Debug("Ollama auto-detection disabled via config")
		return
	}

	if _, exists := cfg.Providers.Get(ollamaProviderID); exists {
		slog.Debug("Ollama provider already configured, skipping auto-detection")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), ollamaTimeout)
	defer cancel()

	models, err := discoverOllamaModels(ctx)
	if err != nil {
		slog.Debug("Ollama auto-detection: not available", "error", err)
		return
	}
	if len(models) == 0 {
		slog.Debug("Ollama auto-detection: no models found")
		return
	}

	base := ollamaBaseURL()
	cfg.Providers.Set(ollamaProviderID, ProviderConfig{
		ID:      ollamaProviderID,
		Name:    ollamaProviderName,
		BaseURL: base + ollamaOpenAIPath,
		Type:    catwalk.TypeOpenAICompat,
		APIKey:  "ollama", // Ollama ignores the key but some clients require a non-empty value.
		Models:  models,
	})

	slog.Info("Auto-detected Ollama", "models", len(models), "endpoint", base)
}
