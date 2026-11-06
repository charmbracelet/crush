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
