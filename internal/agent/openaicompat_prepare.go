package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	openaisdk "github.com/charmbracelet/openai-go/option"
)

func openAICompatSDKOptions() []openaisdk.RequestOption {
	return []openaisdk.RequestOption{
		openaisdk.WithMiddleware(stripFunctionStrictMiddleware),
	}
}

func stripFunctionStrictMiddleware(req *http.Request, next openaisdk.MiddlewareNext) (*http.Response, error) {
	if req.Body == nil || req.Method != http.MethodPost {
		return next(req)
	}

	body, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return next(req)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		return next(req)
	}

	if stripFunctionStrictFromPayload(payload) {
		if updated, err := json.Marshal(payload); err == nil {
			body = updated
		}
	}

	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	return next(req)
}

func stripFunctionStrictFromPayload(payload map[string]any) bool {
	tools, ok := payload["tools"].([]any)
	if !ok {
		return false
	}

	changed := false
	for _, rawTool := range tools {
		tool, ok := rawTool.(map[string]any)
		if !ok {
			continue
		}
		function, ok := tool["function"].(map[string]any)
		if !ok {
			continue
		}
		if _, exists := function["strict"]; exists {
			delete(function, "strict")
			changed = true
		}
	}
	return changed
}
