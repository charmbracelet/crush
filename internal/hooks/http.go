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
		return &HookOutput{Decision: DecisionAllow}, nil
	}
	req.Header.Set("Content-Type", "application/json")

	for k, v := range httpCfg.Headers {
		req.Header.Set(k, expandEnvVars(v))
	}

	resp, err := h.getClient().Do(req)
	if err != nil {
		return &HookOutput{Decision: DecisionAllow}, nil
	}
	defer resp.Body.Close()

	var output HookOutput
	if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
		return &HookOutput{Decision: DecisionAllow}, nil
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
