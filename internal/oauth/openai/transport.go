package openai

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/charmbracelet/crush/internal/oauth"
)

// Transport intercepts requests and routes authenticated OpenAI calls to the ChatGPT Codex backend.
type Transport struct {
	Base       http.RoundTripper
	Token      *oauth.Token
	Originator string
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	if t.Token == nil {
		return base.RoundTrip(req)
	}

	cloned := req.Clone(req.Context())
	hostname := cloned.URL.Hostname()
	isOfficial := strings.EqualFold(hostname, "api.openai.com") || strings.EqualFold(hostname, "chatgpt.com")

	// Security safeguard: Never send ChatGPT OAuth credentials to third-party endpoints.
	if !isOfficial {
		cloned.Header.Del("Authorization")
		cloned.Header.Del("ChatGPT-Account-Id")
		cloned.Header.Del("x-openai-internal-codex-residency")
		return base.RoundTrip(cloned)
	}

	path := cloned.URL.Path
	isRewriteTarget := strings.Contains(path, "/responses") || strings.Contains(path, "/chat/completions")

	if isRewriteTarget {
		endpointURL, err := url.Parse(CodexEndpoint)
		if err == nil {
			cloned.URL = endpointURL
			cloned.Host = endpointURL.Host
		}

		cloned.Header.Set("Authorization", "Bearer "+t.Token.AccessToken)
		if t.Token.AccountID != "" {
			cloned.Header.Set("ChatGPT-Account-Id", t.Token.AccountID)
		}
		if res := t.Token.Extra["residency"]; res != "" {
			cloned.Header.Set("x-openai-internal-codex-residency", res)
		}

		originator := t.Originator
		if originator == "" {
			originator = "crush"
		}
		cloned.Header.Set("originator", originator)

		if cloned.Header.Get("User-Agent") == "" {
			cloned.Header.Set("User-Agent", "crush")
		}

		// Sanitize request body: remove max_output_tokens which Codex backend rejects.
		if cloned.Body != nil && cloned.Body != http.NoBody {
			bodyBytes, err := io.ReadAll(cloned.Body)
			_ = cloned.Body.Close()
			if err == nil {
				cleaned := sanitizeCodexBody(bodyBytes)
				cloned.Body = io.NopCloser(bytes.NewReader(cleaned))
				cloned.ContentLength = int64(len(cleaned))
			}
		}
	}

	return base.RoundTrip(cloned)
}

func sanitizeCodexBody(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return data
	}
	if _, ok := raw["max_output_tokens"]; !ok {
		return data
	}
	delete(raw, "max_output_tokens")
	cleaned, err := json.Marshal(raw)
	if err != nil {
		return data
	}
	return cleaned
}
