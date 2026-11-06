package openai

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/crush/internal/oauth"
)

// Transport adapts requests made with a ChatGPT OAuth token to what the
// Codex backend accepts: it guarantees the account headers are present,
// keeps the credentials from leaking to non-OpenAI hosts, and drops
// request fields the backend rejects.
type Transport struct {
	// Base is the transport requests are forwarded to. Defaults to
	// http.DefaultTransport.
	Base http.RoundTripper
	// Token is the ChatGPT OAuth token. A nil token passes requests
	// through untouched.
	Token *oauth.Token
	// Originator identifies the client to the backend. Defaults to
	// "crush".
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

	clone := req.Clone(req.Context())
	host := strings.ToLower(clone.URL.Hostname())
	switch host {
	case "chatgpt.com":
		// The Codex backend attributes usage per account and expects
		// the client to identify itself.
		if t.Originator != "" {
			clone.Header.Set("originator", t.Originator)
		} else {
			clone.Header.Set("originator", "crush")
		}
		if t.Token.AccountID != "" {
			clone.Header.Set("chatgpt-account-id", t.Token.AccountID)
		}
		clone.Body, clone.ContentLength = codexRequestBody(clone.Body, clone.ContentLength)
	case "api.openai.com", "auth.openai.com":
		// Official hosts, no adaptation needed.
	default:
		// A custom base URL must never receive ChatGPT credentials:
		// the token would leak to a third party.
		clone.Header.Del("Authorization")
		clone.Header.Del("chatgpt-account-id")
	}

	return base.RoundTrip(clone)
}

// codexRequestBody strips request fields the Codex backend rejects.
// max_output_tokens is a Responses API field the backend refuses; crush
// derives it from the model catalog, so it is present on every call and
// must be removed. Bodies that are not JSON pass through unchanged.
func codexRequestBody(body io.ReadCloser, length int64) (io.ReadCloser, int64) {
	if body == nil || body == http.NoBody || length == 0 {
		return body, length
	}

	data, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil {
		return io.NopCloser(bytes.NewReader(data)), int64(len(data))
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return io.NopCloser(bytes.NewReader(data)), int64(len(data))
	}
	if _, ok := raw["max_output_tokens"]; !ok {
		return io.NopCloser(bytes.NewReader(data)), int64(len(data))
	}
	delete(raw, "max_output_tokens")

	cleaned, err := json.Marshal(raw)
	if err != nil {
		cleaned = data
	}
	return io.NopCloser(bytes.NewReader(cleaned)), int64(len(cleaned))
}
