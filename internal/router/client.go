package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a generic HTTP client for any backend implementing the
// "System One" contract. Self-hosted servers expose it at POST
// {base_url}/v1/systemone; OpenRouter's own alpha Decisions API exposes the
// same contract at POST https://openrouter.ai/api/alpha/decisions. path
// carries that difference; baseURL/apiKey/model vary per backend.
type Client struct {
	baseURL     string
	path        string
	apiKey      string
	model       string
	nestedInput bool
	httpClient  *http.Client
}

// Option customizes a [Client].
type Option func(*Client)

// WithNestedInput wraps state and questions under an "input" object
// instead of the flat System One shape, matching Cloudflare Workers AI.
func WithNestedInput() Option {
	return func(c *Client) { c.nestedInput = true }
}

// NewClient builds a System One client against baseURL+path. timeout
// bounds the whole request; callers should keep it short since this runs
// in front of every user message.
func NewClient(baseURL, path, apiKey, model string, timeout time.Duration, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		path:       path,
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: timeout},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Decide sends state and questions to the configured backend and returns
// its answers (keyed the same as the questions map) and the classifier
// call's own cost — 0 when the backend doesn't report one, which is the
// expected case for a self-hosted local server rather than an error.
func (c *Client) Decide(ctx context.Context, state string, questions map[string]QuestionSpec) (map[string]Answer, float64, error) {
	var reqBody []byte
	var err error
	if c.nestedInput {
		reqBody, err = json.Marshal(nestedDecideRequest{
			Model: c.model,
			Input: nestedDecideRequestInput{State: state, Questions: questions},
		})
	} else {
		reqBody, err = json.Marshal(decideRequest{Model: c.model, State: state, Questions: questions})
	}
	if err != nil {
		return nil, 0, fmt.Errorf("router: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+c.path, bytes.NewReader(reqBody))
	if err != nil {
		return nil, 0, fmt.Errorf("router: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("router: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, 0, fmt.Errorf("router: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var decoded decideResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, 0, fmt.Errorf("router: decode response: %w", err)
	}
	var cost float64
	if decoded.Usage != nil {
		cost = decoded.Usage.Cost
	}
	return decoded.Answers, cost, nil
}
