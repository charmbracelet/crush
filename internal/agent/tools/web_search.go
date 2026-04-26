package tools

import (
	"context"
	_ "embed"
	"log/slog"
	"net/http"
	"time"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/config"
)

//go:embed web_search.md
var webSearchToolDescription []byte

type WebSearchOptions struct {
	DefaultEngine config.SearchEngine
	KagiAPIKey    string
}

// NewWebSearchTool creates a web search tool for sub-agents (no permissions needed).
func NewWebSearchTool(client *http.Client, opts WebSearchOptions) fantasy.AgentTool {
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

	return fantasy.NewParallelAgentTool(
		WebSearchToolName,
		FirstLineDescription(webSearchToolDescription),
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

			engine := opts.DefaultEngine
			if !engine.Valid() {
				engine = config.SearchEngineDuckDuckGo
			}
			if params.SearchEngine != "" {
				engine = config.SearchEngine(params.SearchEngine)
			}
			if !engine.Valid() {
				return fantasy.NewTextErrorResponse("unsupported search_engine: " + params.SearchEngine), nil
			}

			var (
				results []SearchResult
				err     error
			)
			switch engine {
			case config.SearchEngineDuckDuckGo:
				maybeDelaySearch()
				results, err = searchDuckDuckGo(ctx, client, params.Query, maxResults)
			case config.SearchEngineKagi:
				results, err = searchKagi(ctx, client, opts.KagiAPIKey, params.Query, maxResults)
			}
			slog.Debug("Web search completed", "engine", engine, "query", params.Query, "results", len(results), "err", err)
			if err != nil {
				return fantasy.NewTextErrorResponse("Failed to search: " + err.Error()), nil
			}

			return fantasy.NewTextResponse(formatSearchResults(results)), nil
		})
}
