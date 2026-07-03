package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStripFunctionStrictMiddleware(t *testing.T) {
	t.Parallel()

	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &capturedBody))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return stripFunctionStrictMiddleware(req, http.DefaultTransport.RoundTrip)
		}),
	}

	payload := map[string]any{
		"model": "test-model",
		"tools": []any{
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":   "echo",
					"strict": false,
				},
			},
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	_, err = client.Do(req)
	require.NoError(t, err)

	tools, ok := capturedBody["tools"].([]any)
	require.True(t, ok)
	function, ok := tools[0].(map[string]any)["function"].(map[string]any)
	require.True(t, ok)
	_, hasStrict := function["strict"]
	require.False(t, hasStrict)
}

func TestStripFunctionStrictFromPayload(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"tools": []any{
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":   "echo",
					"strict": false,
				},
			},
		},
	}

	changed := stripFunctionStrictFromPayload(payload)
	require.True(t, changed)

	tools := payload["tools"].([]any)
	function := tools[0].(map[string]any)["function"].(map[string]any)
	_, hasStrict := function["strict"]
	require.False(t, hasStrict)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
