package openai_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/openai"
	"github.com/stretchr/testify/require"
)

func TestTransport_RewriteOfficial(t *testing.T) {
	t.Parallel()

	token := &oauth.Token{
		AccessToken: "test-access-token",
		AccountID:   "test-account-id",
		Extra: map[string]string{
			"residency": "us",
		},
	}

	var capturedReq *http.Request
	var capturedBody []byte

	mockBase := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedReq = req
		if req.Body != nil {
			capturedBody, _ = io.ReadAll(req.Body)
		}
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	tr := &openai.Transport{
		Base:       mockBase,
		Token:      token,
		Originator: "crush",
	}

	body := []byte(`{"model":"gpt-4o","max_output_tokens":4096,"messages":[{"role":"user","content":"hello"}]}`)
	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(body))
	require.NoError(t, err)

	_, err = tr.RoundTrip(req)
	require.NoError(t, err)
	require.NotNil(t, capturedReq)

	require.Equal(t, "chatgpt.com", capturedReq.URL.Host)
	require.Equal(t, "/backend-api/codex/responses", capturedReq.URL.Path)
	require.Equal(t, "Bearer test-access-token", capturedReq.Header.Get("Authorization"))
	require.Equal(t, "test-account-id", capturedReq.Header.Get("ChatGPT-Account-Id"))
	require.Equal(t, "us", capturedReq.Header.Get("x-openai-internal-codex-residency"))
	require.Equal(t, "crush", capturedReq.Header.Get("originator"))

	// Ensure max_output_tokens was stripped
	require.NotContains(t, string(capturedBody), "max_output_tokens")
	require.Contains(t, string(capturedBody), `"model":"gpt-4o"`)
}

func TestTransport_SafeguardNonOfficial(t *testing.T) {
	t.Parallel()

	token := &oauth.Token{
		AccessToken: "secret-token",
		AccountID:   "secret-account",
	}

	var capturedReq *http.Request
	mockBase := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedReq = req
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	tr := &openai.Transport{
		Base:  mockBase,
		Token: token,
	}

	req, err := http.NewRequest(http.MethodPost, "https://custom-proxy.example.com/v1/chat/completions", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer custom-key")
	req.Header.Set("ChatGPT-Account-Id", "some-account")

	_, err = tr.RoundTrip(req)
	require.NoError(t, err)
	require.NotNil(t, capturedReq)

	require.Equal(t, "custom-proxy.example.com", capturedReq.URL.Host)
	require.Empty(t, capturedReq.Header.Get("Authorization"))
	require.Empty(t, capturedReq.Header.Get("ChatGPT-Account-Id"))
}
