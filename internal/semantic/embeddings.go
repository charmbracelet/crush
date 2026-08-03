// Package semantic provides embedding generation, vector storage, and
// KNN search over code and session chunks. It uses an OpenAI-compatible
// /v1/embeddings endpoint configured through Crush's config system.
package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// EmbeddingConfig holds the configuration for an embedding provider.
type EmbeddingConfig struct {
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key"`
	Model     string `json:"model"`
	Dimension int    `json:"dimension"`
}

// DefaultEmbeddingConfig returns sensible defaults for embedding.
func DefaultEmbeddingConfig() EmbeddingConfig {
	return EmbeddingConfig{
		BaseURL:   "https://api.openai.com/v1",
		Model:     "text-embedding-3-small",
		Dimension: 768,
	}
}

// Client generates embeddings via an OpenAI-compatible API.
type Client struct {
	cfg    EmbeddingConfig
	client *http.Client
}

// NewClient creates an embedding client with the given configuration.
func NewClient(cfg EmbeddingConfig) *Client {
	return &Client{
		cfg: cfg,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// embeddingRequest is the request body for /v1/embeddings.
type embeddingRequest struct {
	Input      []string `json:"input"`
	Model      string   `json:"model"`
	Dimensions int      `json:"dimensions,omitempty"`
}

// embeddingResponse is the response from /v1/embeddings.
type embeddingResponse struct {
	Data  []embeddingData `json:"data"`
	Error *apiError       `json:"error,omitempty"`
}

type embeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// Embed generates embeddings for a batch of texts. The returned slice
// is in the same order as the input. Retries once on transient errors.
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := embeddingRequest{
		Input:      texts,
		Model:      c.cfg.Model,
		Dimensions: c.cfg.Dimension,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	url := c.cfg.BaseURL + "/embeddings"

	var resp *http.Response
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create embedding request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if c.cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
		}

		resp, err = c.client.Do(req)
		if err != nil {
			if attempt == 0 {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("embedding request failed: %w", err)
		}

		// Only close here when we are actually going to retry. Closing on
		// the final attempt too would leave the error path below reading a
		// closed body, so a retried 5xx always reported an empty message —
		// exactly the case where the server's explanation matters most.
		if attempt == 0 && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) {
			resp.Body.Close()
			time.Sleep(time.Second)
			continue
		}
		break
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var embResp embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}

	if embResp.Error != nil {
		return nil, fmt.Errorf("embedding API error: %s (%s)", embResp.Error.Message, embResp.Error.Type)
	}

	results := make([][]float32, len(texts))
	for _, d := range embResp.Data {
		if d.Index < 0 || d.Index >= len(results) {
			return nil, fmt.Errorf("embedding response index %d out of range", d.Index)
		}
		if len(d.Embedding) != c.cfg.Dimension {
			return nil, fmt.Errorf("embedding dimension mismatch: got %d, want %d", len(d.Embedding), c.cfg.Dimension)
		}
		results[d.Index] = d.Embedding
	}

	// A short or sparse Data array leaves nil holes. Callers index this
	// slice positionally against their inputs, so a hole would be written
	// to storage as an empty vector rather than surfacing as an error.
	for i, e := range results {
		if e == nil {
			return nil, fmt.Errorf("embedding API returned no vector for input %d (got %d embeddings for %d inputs)",
				i, len(embResp.Data), len(texts))
		}
	}

	return results, nil
}
