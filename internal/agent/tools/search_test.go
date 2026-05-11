package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestParseKagiSearchResults(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"data": [
			{
				"t": 0,
				"title": "First result",
				"url": "https://example.com/first",
				"snippet": " First snippet. ",
				"published": "2026-04-26"
			},
			{
				"t": 1,
				"list": ["related query"]
			},
			{
				"t": 0,
				"title": "Second result",
				"url": "https://example.com/second",
				"snippet": "Second snippet."
			}
		]
	}`)

	results, err := parseKagiSearchResults(body, 10)
	require.NoError(t, err)
	require.Equal(t, []SearchResult{
		{
			Title:    "First result",
			Link:     "https://example.com/first",
			Snippet:  "First snippet. (2026-04-26)",
			Position: 1,
		},
		{
			Title:    "Second result",
			Link:     "https://example.com/second",
			Snippet:  "Second snippet.",
			Position: 2,
		},
	}, results)
}

func TestParseKagiSearchResultsMaxResults(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"data": [
			{"t": 0, "title": "First", "url": "https://example.com/first"},
			{"t": 0, "title": "Second", "url": "https://example.com/second"}
		]
	}`)

	results, err := parseKagiSearchResults(body, 1)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "First", results[0].Title)
	require.Equal(t, 1, results[0].Position)
}

func TestWebSearchParamsSearchEngineSchema(t *testing.T) {
	t.Parallel()

	tool := NewWebSearchTool(nil, WebSearchOptions{})
	info := tool.Info()

	paramsJSON, err := json.Marshal(info.Parameters)
	require.NoError(t, err)
	require.Contains(t, string(paramsJSON), "search_engine")
}

func TestWebSearchUsesKagiDefaultEngine(t *testing.T) {
	t.Parallel()

	var capturedRequest *http.Request
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedRequest = req
		return jsonResponse(`{"data":[{"t":0,"title":"Kagi result","url":"https://example.com/kagi","snippet":"Kagi snippet"}]}`), nil
	})}
	tool := NewWebSearchTool(client, WebSearchOptions{
		DefaultEngine: config.SearchEngineKagi,
		KagiAPIKey:    "test-key",
	})

	resp := runWebSearchTool(t, tool, WebSearchParams{Query: "crush kagi", MaxResults: 2})

	require.Contains(t, toolResponseString(t, resp), "Kagi result")
	require.NotNil(t, capturedRequest)
	require.Equal(t, "kagi.com", capturedRequest.URL.Host)
	require.Equal(t, "/api/v0/search", capturedRequest.URL.Path)
	require.Equal(t, "crush kagi", capturedRequest.URL.Query().Get("q"))
	require.Equal(t, "2", capturedRequest.URL.Query().Get("limit"))
	require.Equal(t, "Bot test-key", capturedRequest.Header.Get("Authorization"))
}

func TestWebSearchPerCallSearchEngineOverride(t *testing.T) {
	t.Parallel()

	var capturedRequest *http.Request
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedRequest = req
		return jsonResponse(`{"data":[{"t":0,"title":"Kagi override","url":"https://example.com/kagi","snippet":"Kagi snippet"}]}`), nil
	})}
	tool := NewWebSearchTool(client, WebSearchOptions{
		DefaultEngine: config.SearchEngineDuckDuckGo,
		KagiAPIKey:    "test-key",
	})

	resp := runWebSearchTool(t, tool, WebSearchParams{Query: "crush kagi", SearchEngine: "kagi"})

	require.Contains(t, toolResponseString(t, resp), "Kagi override")
	require.NotNil(t, capturedRequest)
	require.Equal(t, "kagi.com", capturedRequest.URL.Host)
}

func TestWebSearchRejectsUnsupportedSearchEngineBeforeRequest(t *testing.T) {
	t.Parallel()

	called := false
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		return jsonResponse(`{}`), nil
	})}
	tool := NewWebSearchTool(client, WebSearchOptions{})

	resp := runWebSearchTool(t, tool, WebSearchParams{Query: "crush", SearchEngine: "bad"})

	require.Contains(t, toolResponseString(t, resp), "unsupported search_engine: bad")
	require.False(t, called)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (r roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return r(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func runWebSearchTool(t *testing.T, tool fantasy.AgentTool, params WebSearchParams) fantasy.ToolResponse {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "test-call",
		Name:  WebSearchToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	return resp
}

func toolResponseString(t *testing.T, resp fantasy.ToolResponse) string {
	t.Helper()

	require.Equal(t, "text", resp.Type)
	return resp.Content
}
