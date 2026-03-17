package tools

import (
	"context"
	_ "embed"
	"log/slog"
	"net/http"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/plugin"
)

//go:embed web_search.md
var webSearchToolDescription []byte

// NewWebSearchTool creates a web search tool for sub-agents (no permissions needed).
// If a search provider is given, it is used instead of the built-in DuckDuckGo scraper.
func NewWebSearchTool(client *http.Client, searchProvider ...plugin.SearchProvider) fantasy.AgentTool {
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.MaxIdleConns = 100
		transport.MaxIdleConnsPerHost = 10
		transport.IdleConnTimeout = 90 * time.Second

		client = &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		}
	}

	var sp plugin.SearchProvider
	if len(searchProvider) > 0 {
		sp = searchProvider[0]
	}

	return fantasy.NewParallelAgentTool(
		WebSearchToolName,
		string(webSearchToolDescription),
		func(ctx context.Context, params WebSearchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Query == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}

			maxResults := params.MaxResults
			if maxResults <= 0 {
				maxResults = 10
			}
			if maxResults > 20 {
				maxResults = 20
			}

			if sp != nil {
				results, err := sp.Search(ctx, params.Query, maxResults)
				slog.Debug("Plugin search completed", "query", params.Query, "results", len(results), "err", err)
				if err != nil {
					return fantasy.NewTextErrorResponse("Failed to search: " + err.Error()), nil
				}
				return fantasy.NewTextResponse(formatPluginSearchResults(results)), nil
			}

			maybeDelaySearch()
			results, err := searchDuckDuckGo(ctx, client, params.Query, maxResults)
			slog.Debug("Web search completed", "query", params.Query, "results", len(results), "err", err)
			if err != nil {
				return fantasy.NewTextErrorResponse("Failed to search: " + err.Error()), nil
			}

			return fantasy.NewTextResponse(formatSearchResults(results)), nil
		})
}

// formatPluginSearchResults formats plugin.SearchResult into the same text format.
func formatPluginSearchResults(results []plugin.SearchResult) string {
	if len(results) == 0 {
		return "No results found. Try rephrasing your search."
	}

	internal := make([]SearchResult, len(results))
	for i, r := range results {
		internal[i] = SearchResult{
			Title:    r.Title,
			Link:     r.Link,
			Snippet:  r.Snippet,
			Position: r.Position,
		}
	}
	return formatSearchResults(internal)
}
