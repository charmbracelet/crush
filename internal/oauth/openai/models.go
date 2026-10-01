package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth"
)

// ModelInfo mirrors one entry of the Codex backend model catalog.
type ModelInfo struct {
	Slug          string `json:"slug"`
	DisplayName   string `json:"display_name"`
	Description   string `json:"description"`
	Visibility    string `json:"visibility"`
	ContextWindow int64  `json:"context_window"`

	DefaultReasoningLevel string `json:"default_reasoning_level"`
	// SupportedReasoningLevels is a list of {effort, description}
	// presets in the backend's own shape.
	SupportedReasoningLevels []struct {
		Effort string `json:"effort"`
	} `json:"supported_reasoning_levels"`
}

type modelsResponse struct {
	Models []ModelInfo `json:"models"`
}

// clientVersion is the codex client version reported to the models
// endpoint, which requires it. The backend gates models by the codex
// client's feature support; crush runs its own agent loop and does not
// depend on those features, so a deliberately high version requests the
// full catalog the subscription grants.
const clientVersion = "999.0.0"

// Models fetches the model catalog the ChatGPT plan grants access to
// from the Codex backend. Only models the backend lists as visible are
// returned; the rest are internal or retired entries.
func Models(ctx context.Context, token *oauth.Token) ([]catwalk.Model, error) {
	if token == nil {
		return nil, fmt.Errorf("an OAuth token is required to list ChatGPT models")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsEndpoint+"?client_version="+clientVersion, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	if token.AccountID != "" {
		req.Header.Set("chatgpt-account-id", token.AccountID)
	}
	req.Header.Set("originator", "crush")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Codex model catalog: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read Codex model catalog: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &oauth.TokenExchangeError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var mr modelsResponse
	if err := json.Unmarshal(body, &mr); err != nil {
		return nil, fmt.Errorf("decode Codex model catalog: %w", err)
	}

	models := make([]catwalk.Model, 0, len(mr.Models))
	for _, m := range mr.Models {
		// The backend marks internal and retired entries as anything
		// other than "list"; only listed models are user-selectable.
		if m.Slug == "" || m.Visibility != "list" {
			continue
		}
		levels := make([]string, 0, len(m.SupportedReasoningLevels))
		for _, level := range m.SupportedReasoningLevels {
			if level.Effort != "" {
				levels = append(levels, level.Effort)
			}
		}
		models = append(models, catwalk.Model{
			ID:                     m.Slug,
			Name:                   m.DisplayName,
			ContextWindow:          m.ContextWindow,
			CanReason:              len(levels) > 0,
			ReasoningLevels:        levels,
			DefaultReasoningEffort: m.DefaultReasoningLevel,
			SupportsImages:         true,
		})
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("the Codex model catalog was empty")
	}
	return models, nil
}
