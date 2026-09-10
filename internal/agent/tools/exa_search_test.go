package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/stretchr/testify/require"
)

func TestParseExaResponse(t *testing.T) {
	t.Parallel()

	payload := `{"result":{"content":[{"type":"text","text":"Title: One\nURL: https://one.example\nHighlights: First"}]}}`
	tests := []struct {
		name    string
		body    string
		want    string
		wantErr string
	}{
		{name: "plain JSON", body: payload, want: "Title: One\nURL: https://one.example\nHighlights: First"},
		{name: "SSE", body: "event: message\ndata: " + payload + "\n\n", want: "Title: One\nURL: https://one.example\nHighlights: First"},
		{name: "multiple data lines", body: "data: " + payload + "\ndata: " + strings.Replace(payload, "One", "Two", 1), want: "Title: One\nURL: https://one.example\nHighlights: First\nTitle: Two\nURL: https://one.example\nHighlights: First"},
		{name: "no data lines", body: "event: ping\n", wantErr: "no search results"},
		{name: "malformed JSON", body: "data: {bad}", wantErr: "no search results"},
		{name: "empty content", body: `{"result":{"content":[]}}`, wantErr: "no search results"},
		{name: "JSON-RPC error", body: `{"error":{"code":-32600,"message":"invalid request"}}`, wantErr: "-32600: invalid request"},
		{name: "tool error", body: `{"result":{"isError":true,"content":[{"type":"text","text":"search failed"}]}}`, wantErr: "exa tool call failed: search failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseExaResponse([]byte(tt.body))
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseExaSearchResults(t *testing.T) {
	t.Parallel()

	blob := `Title: First
URL: https://first.example
Published: 2025-01-01
Author: Author
Highlights: > A useful first result.
...
More detail.

Title: Second
URL: https://second.example
Highlights: Second result.

Title: Third
Highlights: Third result.`
	results := parseExaSearchResults(blob, 3)
	require.Len(t, results, 3)
	require.Equal(t, SearchResult{
		Title:    "First",
		Link:     "https://first.example",
		Snippet:  "A useful first result. More detail.",
		Position: 1,
	}, results[0])
	require.Equal(t, 2, results[1].Position)
	require.Equal(t, 3, results[2].Position)
	require.Empty(t, results[2].Link)
}

func TestParseExaSearchResultsDegradesUnparseableBlob(t *testing.T) {
	t.Parallel()

	results := parseExaSearchResults(strings.Repeat("a", exaResultLimit+100), 10)
	require.Len(t, results, 1)
	require.Equal(t, "Exa search results", results[0].Title)
	require.LessOrEqual(t, len([]rune(results[0].Snippet)), exaResultLimit)
}

func TestFormatExaSearchResultsHasOutputCap(t *testing.T) {
	t.Parallel()

	results := []SearchResult{{Title: "Title", Snippet: strings.Repeat("x", exaOutputLimit)}}
	require.LessOrEqual(t, len(formatExaSearchResults(results)), exaOutputLimit)
}

func TestDegradedExaResultOmitsEmptyURL(t *testing.T) {
	t.Parallel()

	results := parseExaSearchResults("unstructured result text", 1)
	require.NotContains(t, formatExaSearchResults(results), "URL:")
}

func TestExaFailuresFallBackToDuckDuckGo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		exaReply     func() (*http.Response, error)
		wantExaCalls int32
	}{
		{
			name:         "server error is retried once",
			exaReply:     func() (*http.Response, error) { return response(http.StatusInternalServerError, `{}`), nil },
			wantExaCalls: 2,
		},
		{
			name:         "rate limit is retried once",
			exaReply:     func() (*http.Response, error) { return response(http.StatusTooManyRequests, `{}`), nil },
			wantExaCalls: 2,
		},
		{
			name:         "transport failure is not retried",
			exaReply:     func() (*http.Response, error) { return nil, context.DeadlineExceeded },
			wantExaCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var exaCalls atomic.Int32
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if isExaRequest(req) {
					exaCalls.Add(1)
					return tt.exaReply()
				}
				return response(http.StatusOK, `<html><a class="result-link" href="https://ddg.example">DDG</a><td class="result-snippet">Fallback</td></html>`), nil
			})}
			tool := NewWebSearchTool(client, WebSearchOptions{DefaultEngine: config.SearchEngineExa})

			result := runWebSearchTool(t, tool, WebSearchParams{Query: "test", MaxResults: 1})
			require.Contains(t, result.Content, "fell back to DuckDuckGo")
			require.Contains(t, result.Content, "DDG")
			require.Equal(t, tt.wantExaCalls, exaCalls.Load())
		})
	}
}

func TestSearchExaCallerCancellationDoesNotFallback(t *testing.T) {
	t.Parallel()

	calledDDG := atomic.Bool{}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if isExaRequest(req) {
			<-req.Context().Done()
			return nil, req.Context().Err()
		}
		calledDDG.Store(true)
		return response(http.StatusOK, `{}`), nil
	})}
	tool := NewWebSearchTool(client, WebSearchOptions{DefaultEngine: config.SearchEngineExa})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	input, err := json.Marshal(WebSearchParams{Query: "test"})
	require.NoError(t, err)
	_, err = tool.Run(ctx, fantasy.ToolCall{Name: WebSearchToolName, Input: string(input)})
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, calledDDG.Load())
}

func TestSearchExaOutboundRequest(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "key with spaces", req.URL.Query().Get("exaApiKey"))
		require.Equal(t, "crush", req.Header.Get("x-exa-integration"))
		require.Equal(t, "crush/"+version.Version, req.Header.Get("User-Agent"))
		return response(http.StatusOK, `{"result":{"content":[{"type":"text","text":"Title: Result\nURL: https://example.com\nHighlights: Text"}]}}`), nil
	})}
	results, err := searchExa(context.Background(), client, "key with spaces", "test", 1)
	require.NoError(t, err)
	require.Len(t, results, 1)
}

func TestSearchExaErrorPreservesCauseWithoutEndpoint(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})}
	_, err := searchExa(context.Background(), client, "secret", "test", 1)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NotContains(t, err.Error(), "mcp.exa.ai")
	require.NotContains(t, err.Error(), "secret")
}

func isExaRequest(req *http.Request) bool {
	return strings.Contains(req.URL.Host, "exa.ai")
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (r roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return r(req)
}

func runWebSearchTool(t *testing.T, tool fantasy.AgentTool, params WebSearchParams) fantasy.ToolResponse {
	t.Helper()
	input, err := json.Marshal(params)
	require.NoError(t, err)
	response, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "test-call",
		Name:  WebSearchToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	return response
}
