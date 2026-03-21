package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"charm.land/catwalk/pkg/catwalk"
)

const (
	ollamaProviderID   = "ollama"
	ollamaProviderName = "Ollama"
	ollamaDefaultHost  = "http://localhost:11434"
	ollamaAPIPath      = "/api/tags"
	// Ollama exposes an OpenAI-compatible endpoint at /v1.
	ollamaOpenAIPath = "/v1/"

	// ollamaTimeout is the maximum time to wait for Ollama to respond
	// during auto-detection. Kept short so startup is not noticeably
	// delayed when Ollama is not running.
	ollamaTimeout = 2 * time.Second
)

// ollamaTagsResponse mirrors the JSON returned by Ollama's GET /api/tags
// endpoint. See https://docs.ollama.com/api/tags.
type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

// ollamaModel is a single entry in the /api/tags response.
type ollamaModel struct {
	Name       string         `json:"name"`
	Model      string         `json:"model"`
	ModifiedAt string         `json:"modified_at"`
	Size       int64          `json:"size"`
	Details    ollamaDetails  `json:"details"`
	ModelInfo  map[string]any `json:"model_info,omitempty"`
}

// ollamaDetails holds metadata about a model.
type ollamaDetails struct {
	ParameterSize   string   `json:"parameter_size"`
	QuantizationLvl string   `json:"quantization_level"`
	Family          string   `json:"family"`
	Families        []string `json:"families"`
	Format          string   `json:"format"`
}

// ollamaBaseURL returns the Ollama host, respecting the OLLAMA_HOST
// environment variable that Ollama itself honours.
func ollamaBaseURL() string {
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		return host
	}
	return ollamaDefaultHost
}

// discoverOllamaModels queries a running Ollama instance and returns
// the models it has available. If Ollama is unreachable or returns an
// error, it returns nil and a non-nil error.
func discoverOllamaModels(ctx context.Context) ([]catwalk.Model, error) {
	base := ollamaBaseURL()
	url := base + ollamaAPIPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama request: %w", err)
	}

	client := &http.Client{Timeout: ollamaTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach Ollama at %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama returned status %d from %s", resp.StatusCode, url)
	}

	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	if len(tags.Models) == 0 {
		return nil, nil
	}

	models := make([]catwalk.Model, 0, len(tags.Models))
	for _, m := range tags.Models {
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
