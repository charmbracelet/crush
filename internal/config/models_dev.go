package config

import (
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/catwalk/pkg/catwalk"
)

// ModelsDevData is the top-level response from models.dev/api.json.
// Keys are provider IDs (e.g. "openai", "anthropic").
type ModelsDevData map[string]ModelsDevProvider

type ModelsDevProvider struct {
	ID     string                    `json:"id"`
	Name   string                    `json:"name"`
	Models map[string]ModelsDevModel `json:"models"`
}

type ModelsDevModel struct {
	ID               string               `json:"id"`
	Name             string               `json:"name"`
	Family           string               `json:"family"`
	Attachment       bool                 `json:"attachment"`
	Reasoning        bool                 `json:"reasoning"`
	ToolCall         bool                 `json:"tool_call"`
	StructuredOutput bool                 `json:"structured_output"`
	Temperature      bool                 `json:"temperature"`
	Knowledge        string               `json:"knowledge"`
	ReleaseDate      string               `json:"release_date"`
	LastUpdated      string               `json:"last_updated"`
	Modalities       *ModelsDevModalities `json:"modalities,omitempty"`
	OpenWeights      bool                 `json:"open_weights"`
	Cost             *ModelsDevCost       `json:"cost,omitempty"`
	Limit            *ModelsDevLimit      `json:"limit,omitempty"`
}

type ModelsDevModalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

type ModelsDevCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
}

type ModelsDevLimit struct {
	Context int64 `json:"context"`
	Input   int64 `json:"input"`
	Output  int64 `json:"output"`
}

//go:embed models_dev_snapshot.json
var modelsDevSnapshot []byte

func embeddedModelsDevData() ModelsDevData {
	var data ModelsDevData
	if err := json.Unmarshal(modelsDevSnapshot, &data); err != nil {
		slog.Warn("Failed to unmarshal embedded models.dev snapshot", "error", err)
		return make(ModelsDevData)
	}
	return data
}

const modelsDevURL = "https://models.dev/api.json"

var modelsDevSyncer = &modelsDevSync{}

var _ syncer[ModelsDevData] = (*modelsDevSync)(nil)

type modelsDevSync struct {
	once       sync.Once
	result     ModelsDevData
	cache      cache[ModelsDevData]
	autoupdate bool
	init       atomic.Bool
}

func (s *modelsDevSync) Init(path string, autoupdate bool) {
	s.cache = newCache[ModelsDevData](path)
	s.autoupdate = autoupdate
	s.init.Store(true)
}

func (s *modelsDevSync) Get(ctx context.Context) (ModelsDevData, error) {
	if !s.init.Load() {
		panic("called Get before Init")
	}

	var throwErr error
	s.once.Do(func() {
		if !s.autoupdate {
			slog.Info("Using embedded models.dev data")
			s.result = embeddedModelsDevData()
			return
		}

		cached, _, cachedErr := s.cache.Get()
		if len(cached) == 0 || cachedErr != nil {
			cached = embeddedModelsDevData()
		}

		slog.Info("Fetching model metadata from models.dev")
		result, err := fetchModelsDev(ctx)
		if errors.Is(err, context.DeadlineExceeded) {
			slog.Warn("models.dev data not updated in time")
			s.result = cached
			return
		}
		if err != nil {
			slog.Warn("Failed to fetch models.dev data", "error", err)
			s.result = cached
			return
		}
		if len(result) == 0 {
			s.result = cached
			throwErr = errors.New("empty data from models.dev")
			return
		}

		s.result = result
		throwErr = s.cache.Store(result)
	})
	return s.result, throwErr
}

func fetchModelsDev(ctx context.Context) (ModelsDevData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsDevURL, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var data ModelsDevData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return data, nil
}

// LookupModel searches all providers in the models.dev data for a model by ID.
// Returns the model and true if found, nil and false otherwise.
func (data ModelsDevData) LookupModel(modelID string) (*ModelsDevModel, bool) {
	for _, provider := range data {
		if m, ok := provider.Models[modelID]; ok {
			return &m, true
		}
	}
	return nil, false
}

// ToCatwalkModel converts a models.dev model to a catwalk.Model.
func (m *ModelsDevModel) ToCatwalkModel() catwalk.Model {
	model := catwalk.Model{
		ID:   m.ID,
		Name: cmp.Or(m.Name, m.ID),
	}
	if m.Cost != nil {
		model.CostPer1MIn = m.Cost.Input
		model.CostPer1MOut = m.Cost.Output
		model.CostPer1MInCached = m.Cost.CacheRead
		model.CostPer1MOutCached = m.Cost.CacheWrite
	}
	if m.Limit != nil {
		model.ContextWindow = m.Limit.Context
		model.DefaultMaxTokens = m.Limit.Output
	}
	model.CanReason = m.Reasoning
	if m.Modalities != nil {
		model.SupportsImages = slices.Contains(m.Modalities.Input, "image")
	}
	return model
}

// EnrichModel fills in missing metadata on a catwalk.Model from models.dev data.
// Only zero-valued fields are overwritten.
func EnrichModel(model *catwalk.Model, data ModelsDevData) {
	devModel, found := data.LookupModel(model.ID)
	if !found {
		return
	}

	if model.Name == "" || model.Name == model.ID {
		if devModel.Name != "" {
			model.Name = devModel.Name
		}
	}
	if model.ContextWindow == 0 && devModel.Limit != nil {
		model.ContextWindow = devModel.Limit.Context
	}
	if model.DefaultMaxTokens == 0 && devModel.Limit != nil {
		model.DefaultMaxTokens = devModel.Limit.Output
	}
	if model.CostPer1MIn == 0 && devModel.Cost != nil {
		model.CostPer1MIn = devModel.Cost.Input
	}
	if model.CostPer1MOut == 0 && devModel.Cost != nil {
		model.CostPer1MOut = devModel.Cost.Output
	}
	if model.CostPer1MInCached == 0 && devModel.Cost != nil {
		model.CostPer1MInCached = devModel.Cost.CacheRead
	}
	if model.CostPer1MOutCached == 0 && devModel.Cost != nil {
		model.CostPer1MOutCached = devModel.Cost.CacheWrite
	}
	if !model.CanReason && devModel.Reasoning {
		model.CanReason = true
	}
	if !model.SupportsImages && devModel.Modalities != nil {
		model.SupportsImages = slices.Contains(devModel.Modalities.Input, "image")
	}
}

// GetModelsDevData returns the fetched models.dev data.
// Returns nil if the syncer hasn't been initialized yet.
func GetModelsDevData() ModelsDevData {
	return modelsDevSyncer.result
}
