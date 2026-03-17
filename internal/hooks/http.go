package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
)

type httpHandler struct {
	cfg    HookConfig
	client *http.Client
	once   sync.Once
}

func newHTTPHandler(cfg HookConfig) Handler {
	return &httpHandler{cfg: cfg}
}

func (h *httpHandler) getClient() *http.Client {
	h.once.Do(func() {
		h.client = &http.Client{}
	})
	return h.client
}

func (h *httpHandler) Execute(ctx context.Context, input HookInput) (*HookOutput, error) {
	httpCfg := h.cfg.HTTP

	payload, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hook input: %w", err)
	}

	method := httpCfg.Method
	if method == "" {
		method = http.MethodPost
	}

	req, err := http.NewRequestWithContext(ctx, method, httpCfg.URL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	for k, v := range httpCfg.Headers {
		req.Header.Set(k, expandEnvVars(v))
	}

	resp, err := h.getClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("hook returned non-2xx status: %d", resp.StatusCode)
	}

	var output HookOutput
	if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
		return nil, fmt.Errorf("failed to decode hook response: %w", err)
	}

	if output.Decision == "" {
		output.Decision = DecisionAllow
	}
	return &output, nil
}

func expandEnvVars(s string) string {
	return os.Expand(s, func(key string) string {
		return os.Getenv(key)
	})
}
