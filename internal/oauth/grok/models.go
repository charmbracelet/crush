package grok

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth"
)

// APIBaseURL is the xAI API the Grok account's access token authorizes.
const APIBaseURL = "https://api.x.ai/v1"

// modelsEndpoint is a variable so tests can point it at a stub server.
var modelsEndpoint = APIBaseURL + "/models"

// modelCapabilities mirrors the request parameters a model accepts.
type modelCapabilities struct {
	// ReasoningEffort lists the effort values the model honors.
	ReasoningEffort []string `json:"reasoning_effort"`
	// DefaultReasoningEffort is used when no effort is selected.
	DefaultReasoningEffort string `json:"default_reasoning_effort"`
}

// modelInfo mirrors one entry of the xAI model listing.
type modelInfo struct {
	ID string `json:"id"`
	// ContextLength is the maximum context length in tokens.
	ContextLength int64 `json:"context_length"`
	// CompletionTextTokenPrice is in USD cents per 100M tokens and is
	// set only for text-generating models; image and video generation
	// models carry image_price instead. Its presence is the
	// language-model filter.
	CompletionTextTokenPrice *int64 `json:"completion_text_token_price"`
	// PromptImageTokenPrice is in USD cents per 100M tokens; a non-zero
	// value marks models that accept image input.
	PromptImageTokenPrice int64              `json:"prompt_image_token_price"`
	Capabilities          *modelCapabilities `json:"capabilities"`
}

type modelsResponse struct {
	Models []modelInfo `json:"data"`
}

// Models fetches the model catalog the Grok plan grants from the xAI API.
// Only text-generating entries are returned: the listing also carries
// image and video generation models, which are not chat models.
func Models(ctx context.Context, token *oauth.Token) ([]catwalk.Model, error) {
	if token == nil {
		return nil, fmt.Errorf("an OAuth token is required to list Grok models")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Grok model catalog: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read Grok model catalog: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &oauth.TokenExchangeError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var mr modelsResponse
	if err := json.Unmarshal(body, &mr); err != nil {
		return nil, fmt.Errorf("decode Grok model catalog: %w", err)
	}

	models := make([]catwalk.Model, 0, len(mr.Models))
	for _, m := range mr.Models {
		// Entries without completion pricing generate images or video
		// instead of text; they are not chat models.
		if m.ID == "" || m.CompletionTextTokenPrice == nil {
			continue
		}
		var levels []string
		var defaultEffort string
		if m.Capabilities != nil {
			levels = m.Capabilities.ReasoningEffort
			defaultEffort = m.Capabilities.DefaultReasoningEffort
		}
		models = append(models, catwalk.Model{
			ID:                     m.ID,
			Name:                   modelName(m.ID),
			ContextWindow:          m.ContextLength,
			CanReason:              len(levels) > 0,
			ReasoningLevels:        levels,
			DefaultReasoningEffort: defaultEffort,
			SupportsImages:         m.PromptImageTokenPrice > 0,
		})
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("the Grok model catalog was empty")
	}
	return models, nil
}

// modelName derives a display name from a model ID: "grok-4-fast"
// becomes "Grok 4 Fast". The xAI listing carries no display names.
func modelName(id string) string {
	words := make([]string, 0, strings.Count(id, "-")+1)
	for w := range strings.SplitSeq(id, "-") {
		if w == "" {
			continue
		}
		words = append(words, strings.ToUpper(w[:1])+w[1:])
	}
	return strings.Join(words, " ")
}
