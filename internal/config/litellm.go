package config

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

// LiteLLMType is the provider type string users set in crush.json to
// enable automatic model discovery from a LiteLLM proxy.
const LiteLLMType catwalk.Type = "litellm"

// litellmModelsResponse mirrors the OpenAI-compatible /v1/models
// response returned by a LiteLLM proxy.
type litellmModelsResponse struct {
	Data []litellmModel `json:"data"`
}

// litellmModel is a single entry in the LiteLLM /v1/models list.
type litellmModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// fetchLiteLLMModels calls the LiteLLM /v1/models endpoint and returns
// a slice of catwalk.Model with sensible defaults. baseURL should be
// the root of the LiteLLM proxy (e.g. "http://localhost:36253"). The
// apiKey is sent as a Bearer token when non-empty.
func fetchLiteLLMModels(baseURL, apiKey string) ([]catwalk.Model, error) {
	baseURL = strings.TrimRight(baseURL, "/")

	// Build the models endpoint URL. Accept both forms:
	//   http://localhost:36253
	//   http://localhost:36253/v1
	modelsURL := baseURL + "/models"
	if !strings.Contains(baseURL, "/v1") {
		modelsURL = baseURL + "/v1/models"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching models from %s: %w", modelsURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LiteLLM returned status %d from %s", resp.StatusCode, modelsURL)
	}

	var modelsResp litellmModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("decoding LiteLLM models response: %w", err)
	}

	if len(modelsResp.Data) == 0 {
		return nil, fmt.Errorf("LiteLLM returned no models from %s", modelsURL)
	}

	models := make([]catwalk.Model, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		models = append(models, catwalk.Model{
			ID:               m.ID,
			Name:             m.ID,
			ContextWindow:    128_000, // Safe default; LiteLLM doesn't expose this.
			DefaultMaxTokens: 16_384,
		})
	}

	slog.Info("Discovered models from LiteLLM", "count", len(models), "endpoint", modelsURL)
	return models, nil
}
