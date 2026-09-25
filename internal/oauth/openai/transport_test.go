package openai

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestTransport_CodexRequest(t *testing.T) {
	t.Parallel()

	token := &oauth.Token{
		AccessToken: "at-codex",
		AccountID:   "acct-1",
	}

	var (
		gotReq  *http.Request
		gotBody []byte
	)
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotReq = req
		if req.Body != nil {
			gotBody, _ = io.ReadAll(req.Body)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})

	tr := &Transport{Base: base, Token: token}

	body := []byte(`{"model":"gpt-5.1-codex","max_output_tokens":4096,"input":[]}`)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://chatgpt.com/backend-api/codex/responses", bytes.NewReader(body))
	require.NoError(t, err)

	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.NotNil(t, gotReq)

	require.Equal(t, "crush", gotReq.Header.Get("originator"))
	require.Equal(t, "acct-1", gotReq.Header.Get("chatgpt-account-id"))
	// max_output_tokens is rejected by the Codex backend and must be
	// stripped from the body.
	require.NotContains(t, string(gotBody), "max_output_tokens")
	require.Contains(t, string(gotBody), `"model":"gpt-5.1-codex"`)
}

func TestTransport_KeepsBodyWithoutMaxOutputTokens(t *testing.T) {
	t.Parallel()

	var gotBody []byte
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotBody, _ = io.ReadAll(req.Body)
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})

	tr := &Transport{Base: base, Token: &oauth.Token{}}

	body := []byte(`{"model":"gpt-5.1-codex"}`)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://chatgpt.com/backend-api/codex/responses", bytes.NewReader(body))
	require.NoError(t, err)

	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.JSONEq(t, `{"model":"gpt-5.1-codex"}`, string(gotBody))
}

func TestTransport_NonOpenAIHostStripsCredentials(t *testing.T) {
	t.Parallel()

	var gotReq *http.Request
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotReq = req
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})

	tr := &Transport{Base: base, Token: &oauth.Token{AccessToken: "secret", AccountID: "acct"}}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://proxy.example.com/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("chatgpt-account-id", "acct")

	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Empty(t, gotReq.Header.Get("Authorization"))
	require.Empty(t, gotReq.Header.Get("chatgpt-account-id"))
}

func TestTransport_NilTokenPassThrough(t *testing.T) {
	t.Parallel()

	var gotReq *http.Request
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotReq = req
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})

	tr := &Transport{Base: base}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com/x", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer user-key")

	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, "Bearer user-key", gotReq.Header.Get("Authorization"))
}

// TestTransport_RecordsPlanUsage covers the free half of the feature: the
// figures ride along on responses Crush already makes, so no fetch is needed
// to keep the readout current during a session.
func TestTransport_RecordsPlanUsage(t *testing.T) {
	// Not parallel: the snapshot is process-global.
	resetUsage(t)

	base := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
			Header:     planHeaders(),
		}, nil
	})

	tr := &Transport{Base: base, Token: &oauth.Token{AccessToken: "at", AccountID: "acct"}}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"https://chatgpt.com/backend-api/codex/responses", http.NoBody)
	require.NoError(t, err)

	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	u, ok := LatestUsage()
	require.True(t, ok, "a plan-billed response must record its usage")
	require.Equal(t, 1.0, u.Spent())
	require.Len(t, u.Windows, 2)
}

// TestTransport_IgnoresPlanUsageFromOtherHosts keeps a custom base URL from
// feeding the readout numbers it made up.
func TestTransport_IgnoresPlanUsageFromOtherHosts(t *testing.T) {
	resetUsage(t)

	base := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
			Header:     planHeaders(),
		}, nil
	})

	tr := &Transport{Base: base, Token: &oauth.Token{AccessToken: "at"}}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"https://evil.example.com/v1/responses", http.NoBody)
	require.NoError(t, err)

	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	_, ok := LatestUsage()
	require.False(t, ok, "only the real backend is trusted for plan usage")
}

// resetUsage clears the process-global snapshot around a test that writes it.
func resetUsage(t *testing.T) {
	t.Helper()
	usageMu.Lock()
	latestUsage = nil
	usageMu.Unlock()
	t.Cleanup(func() {
		usageMu.Lock()
		latestUsage = nil
		usageMu.Unlock()
	})
}
