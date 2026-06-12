package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"charm.land/catwalk/pkg/catwalk"
)

// DefaultDiscoverTimeout is the timeout for model discovery requests.
const DefaultDiscoverTimeout = 3 * time.Second

type modelsResponse struct {
	Data []modelEntry `json:"data"`
}

type modelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

type Config struct {
	ID             string
	BaseURL        string
	APIKey         string
	ExtraHeaders   map[string]string
	ExistingModels []catwalk.Model
}

// DiscoverModels fetches the model list from the provider's /v1/models
// endpoint and returns catwalk.Model entries with sensible defaults.
// Duplicate detection: user-specified models take precedence over
// discovered ones.
func DiscoverModels(ctx context.Context, cfg Config) ([]catwalk.Model, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")

	modelsURL := baseURL + "/models"
	if !strings.Contains(baseURL, "/v1") {
		modelsURL = baseURL + "/v1/models"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	for k, v := range cfg.ExtraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching models from %s: %w", modelsURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("model discovery returned status %d from %s", resp.StatusCode, modelsURL)
	}

	var mResp modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&mResp); err != nil {
		return nil, fmt.Errorf("decoding model discovery response: %w", err)
	}

	if len(mResp.Data) == 0 {
		return nil, fmt.Errorf("model discovery returned no models from %s", modelsURL)
	}

	// Build a set of user-defined model IDs so they take precedence.
	seen := make(map[string]bool)
	for _, m := range cfg.ExistingModels {
		seen[m.ID] = true
	}

	models := make([]catwalk.Model, 0, len(mResp.Data))
	for _, entry := range mResp.Data {
		if seen[entry.ID] {
			continue
		}
		models = append(models, catwalk.Model{
			ID:               entry.ID,
			Name:             entry.ID,
			ContextWindow:    0, // Unknown; enrichers should fill this in.
			DefaultMaxTokens: 0,
		})
	}

	slog.Debug("Discovered models", "count", len(models), "endpoint", modelsURL)
	return models, nil
}
