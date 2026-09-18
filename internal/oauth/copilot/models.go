package copilot

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

// ModelInfo is a single entry of the Copilot /models payload. Only the
// fields Crush needs are decoded.
type ModelInfo struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Enabled            bool     `json:"model_picker_enabled"`
	SupportedEndpoints []string `json:"supported_endpoints"`
	Capabilities struct {
		Limits struct {
			ContextWindow int64 `json:"max_context_window_tokens"`
		}
		Supports struct {
			Vision          bool     `json:"vision"`
			Thinking        bool     `json:"thinking"`
			ReasoningEffort []string `json:"reasoning_effort"`
		}
	} `json:"capabilities"`
}

type modelsResponse struct {
	Models []ModelInfo `json:"data"`
}

// fetchCatalog GETs the Copilot /models catalog using the OAuth token
// obtained from the device flow.
func fetchCatalog(ctx context.Context, token *oauth.Token) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("OpenAI-Intent", "model-access")
	req.Header.Set("originator", "crush")
	// The API rejects catalog requests without the editor identity
	// headers, asking for IDE auth.
	for k, v := range Headers() {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Copilot model catalog: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read Copilot model catalog: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &oauth.TokenExchangeError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var payload modelsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode Copilot model catalog: %w", err)
	}
	return payload.Models, nil
}

// Models fetches the model catalog the GitHub Copilot subscription
// grants, using the OAuth token obtained from the device flow. Models
// the plan does not enable for the model picker are filtered out.
func Models(ctx context.Context, token *oauth.Token) ([]catwalk.Model, error) {
	if token == nil {
		return nil, fmt.Errorf("an OAuth token is required to list GitHub Copilot models")
	}

	catalog, err := fetchCatalog(ctx, token)
	if err != nil {
		return nil, err
	}

	models := make([]catwalk.Model, 0, len(catalog)+1)
	hasAuto := false
	for _, m := range catalog {
		if !m.Enabled {
			continue
		}
		if m.ID == AutoModelID {
			hasAuto = true
		}
		models = append(models, catwalk.Model{
			ID:              m.ID,
			Name:            m.Name,
			ContextWindow:   m.Capabilities.Limits.ContextWindow,
			CanReason:       m.Capabilities.Supports.Thinking || len(m.Capabilities.Supports.ReasoningEffort) > 0,
			ReasoningLevels: m.Capabilities.Supports.ReasoningEffort,
			SupportsImages:  m.Capabilities.Supports.Vision,
		})
	}
	if len(catalog) == 0 {
		return nil, fmt.Errorf("the Copilot model catalog was empty")
	}

	// Auto is the only model some accounts, such as Copilot Free and
	// student plans, may use: the server lists every model with the
	// picker disabled. The catalog does not always list it, so offer
	// it first when missing, mirroring the official VS Code
	// extension's picker. The transport resolves it to a session-granted
	// model at request time.
	if !hasAuto {
		// Inherit the default model's limits as a safe approximation;
		// the session substitutes the concrete model anyway. When the
		// picker is disabled for every model, fall back to the first
		// catalog entry, whose capabilities are still populated.
		reference := catalog[0]
		for _, m := range catalog {
			if m.Enabled {
				reference = m
				break
			}
		}
		auto := catwalk.Model{
			ID:             AutoModelID,
			Name:           "Auto",
			ContextWindow:  reference.Capabilities.Limits.ContextWindow,
			CanReason:      reference.Capabilities.Supports.Thinking || len(reference.Capabilities.Supports.ReasoningEffort) > 0,
			SupportsImages: reference.Capabilities.Supports.Vision,
		}
		models = append([]catwalk.Model{auto}, models...)
	}
	return models, nil
}
