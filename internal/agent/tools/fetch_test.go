package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func runFetchTool(t *testing.T, client *http.Client, url string) (fantasy.ToolResponse, error) {
	t.Helper()

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	tool := NewFetchTool(&mockPermissionService{}, t.TempDir(), client)

	input, err := json.Marshal(FetchParams{URL: url, Format: "text"})
	require.NoError(t, err)

	return tool.Run(ctx, fantasy.ToolCall{
		ID:    "fetch-call",
		Name:  FetchToolName,
		Input: string(input),
	})
}

// A fetch that never completes must be reported to the model as a failed tool
// result. Returning a Go error instead ends the whole agent turn.
func TestFetchToolReportsNetworkFailuresAsToolErrors(t *testing.T) {
	t.Parallel()

	t.Run("connection refused", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("unused"))
		}))
		url := server.URL
		server.Close()

		resp, err := runFetchTool(t, server.Client(), url)
		require.NoError(t, err)
		require.True(t, resp.IsError)
		require.Contains(t, resp.Content, "Failed to fetch URL")
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.Write([]byte("too late"))
		}))
		defer server.Close()

		client := &http.Client{Timeout: 20 * time.Millisecond}
		resp, err := runFetchTool(t, client, server.URL)
		require.NoError(t, err)
		require.True(t, resp.IsError)
		require.Contains(t, resp.Content, "Failed to fetch URL")
		require.Contains(t, resp.Content, "context deadline exceeded")
	})
}

func TestFetchToolReportsStatusCodesAsToolErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	resp, err := runFetchTool(t, server.Client(), server.URL)
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "Request failed with status code: 404 Not Found")
}

func TestFetchToolReturnsContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("hello from fetch"))
	}))
	defer server.Close()

	resp, err := runFetchTool(t, server.Client(), server.URL)
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "hello from fetch")
}
