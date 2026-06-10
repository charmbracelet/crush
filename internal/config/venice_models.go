package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	xetag "github.com/charmbracelet/x/etag"
)

var _ liveProviderClient = realVeniceModelsClient{}

type realVeniceModelsClient struct {
	baseURL string
	apiKey  string
}

func (r realVeniceModelsClient) Get(ctx context.Context, etag string) (catwalk.Provider, error) {
	var result catwalk.Provider
	baseURL := strings.TrimRight(r.baseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return result, fmt.Errorf("could not create request: %w", err)
	}
	xetag.Request(req, etag)
	if apiKey := strings.TrimSpace(r.apiKey); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotModified {
		return result, catwalk.ErrNotModified
	}
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var models veniceModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return result, fmt.Errorf("failed to decode response: %w", err)
	}

	result = catwalk.Provider{
		ID:          catwalk.InferenceProviderVenice,
		APIEndpoint: baseURL,
		Type:        catwalk.TypeOpenAICompat,
		Models:      veniceModelsToCatwalkModels(models),
	}
	return result, nil
}

type veniceModelsResponse struct {
	Data []veniceModel `json:"data"`
}

type veniceModel struct {
	ID        string          `json:"id"`
	ModelSpec veniceModelSpec `json:"model_spec"`
	Type      string          `json:"type"`
}

type veniceModelSpec struct {
	AvailableContextTokens int64                   `json:"availableContextTokens"`
	MaxCompletionTokens    int64                   `json:"maxCompletionTokens"`
	Capabilities           veniceModelCapabilities `json:"capabilities"`
	Name                   string                  `json:"name"`
	Offline                bool                    `json:"offline"`
	Pricing                veniceModelPricing      `json:"pricing"`
}

type veniceModelCapabilities struct {
	SupportsReasoning      bool     `json:"supportsReasoning"`
	ReasoningEffortOptions []string `json:"reasoningEffortOptions"`
	DefaultReasoningEffort string   `json:"defaultReasoningEffort"`
	SupportsVision         bool     `json:"supportsVision"`
}

type veniceModelPricing struct {
	Input      veniceModelPricingValue `json:"input"`
	Output     veniceModelPricingValue `json:"output"`
	CacheInput veniceModelPricingValue `json:"cache_input"`
}

type veniceModelPricingValue struct {
	USD veniceUSD `json:"usd"`
}

type veniceUSD float64

func (v *veniceUSD) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*v = 0
		return nil
	}

	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		*v = veniceUSD(number)
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("failed to decode usd value: %w", err)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		*v = 0
		return nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("failed to parse usd value: %w", err)
	}
	*v = veniceUSD(parsed)
	return nil
}

func veniceModelsToCatwalkModels(response veniceModelsResponse) []catwalk.Model {
	models := make([]catwalk.Model, 0, len(response.Data))
	for _, model := range response.Data {
		if !strings.EqualFold(model.Type, "text") || model.ModelSpec.Offline {
			continue
		}

		models = append(models, catwalk.Model{
			ID:                     model.ID,
			Name:                   model.ModelSpec.Name,
			CostPer1MIn:            float64(model.ModelSpec.Pricing.Input.USD),
			CostPer1MOut:           float64(model.ModelSpec.Pricing.Output.USD),
			CostPer1MInCached:      float64(model.ModelSpec.Pricing.CacheInput.USD),
			ContextWindow:          model.ModelSpec.AvailableContextTokens,
			DefaultMaxTokens:       model.ModelSpec.MaxCompletionTokens,
			CanReason:              model.ModelSpec.Capabilities.SupportsReasoning,
			ReasoningLevels:        model.ModelSpec.Capabilities.ReasoningEffortOptions,
			DefaultReasoningEffort: model.ModelSpec.Capabilities.DefaultReasoningEffort,
			SupportsImages:         model.ModelSpec.Capabilities.SupportsVision,
		})
	}
	return models
}
