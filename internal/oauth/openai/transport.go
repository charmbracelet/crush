package openai

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/charmbracelet/crush/internal/oauth"
)

// Transport scopes ChatGPT/Codex OAuth credentials to OpenAI's official API
// and rewrites Responses API requests to the Codex backend.
type Transport struct {
	Base                  http.RoundTripper
	Token                 *oauth.Token
	Originator, UserAgent string
}

// RoundTrip implements http.RoundTripper.
func (t Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	b := t.Base
	if b == nil {
		b = http.DefaultTransport
	}
	q := r.Clone(r.Context())
	original := q.URL
	rewrite := strings.Contains(original.Path, "/v1/responses") || strings.Contains(original.Path, "/chat/completions")
	// OAuth credentials are intentionally scoped to the official ChatGPT backend.
	official := strings.EqualFold(original.Hostname(), "api.openai.com")
	if t.Token != nil && !official {
		// This transport is only installed for the official provider's active
		// OAuth flow. Do not try to identify the credential by its current
		// value: api_key can retain an older OAuth access token after refresh.
		q.Header.Del("Authorization")
		q.Header.Del("ChatGPT-Account-Id")
		q.Header.Del("x-openai-internal-codex-residency")
	}
	if t.Token != nil && rewrite && official {
		u, _ := url.Parse(CodexEndpoint)
		q.URL = u
		q.Host = u.Host
		q.Header.Del("Authorization")
		q.Header.Set("Authorization", "Bearer "+t.Token.AccessToken)
		if t.Token.AccountID != "" {
			q.Header.Set("ChatGPT-Account-Id", t.Token.AccountID)
		}
		if t.Token.Residency != "" {
			q.Header.Set("x-openai-internal-codex-residency", t.Token.Residency)
		}
		if t.Originator != "" {
			q.Header.Set("originator", t.Originator)
		}
		if t.UserAgent != "" {
			q.Header.Set("User-Agent", t.UserAgent)
		}
	}
	return b.RoundTrip(q)
}
