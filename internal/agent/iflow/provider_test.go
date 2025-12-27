package iflow

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIFlowTransport(t *testing.T) {
	// Create a mock server to receive the request
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &capturedBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello"}}]}`))
	}))
	defer server.Close()

	// Create the transport
	transport := &iflowTransport{
		base: http.DefaultTransport,
	}

	// Create a request with max_tokens and max_token
	payload := map[string]any{
		"model":      "test-model",
		"messages":   []any{map[string]any{"role": "user", "content": "hi"}},
		"max_tokens": 100,
		"max_token":  100,
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, server.URL, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Send the request through the transport
	client := &http.Client{Transport: transport}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify that max_tokens and max_token were removed
	assert.NotContains(t, capturedBody, "max_tokens")
	assert.NotContains(t, capturedBody, "max_token")
	assert.Equal(t, "test-model", capturedBody["model"])
}
