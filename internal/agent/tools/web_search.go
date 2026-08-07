package tools

import (
	"context"
	_ "embed"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
)

//go:embed web_search.md.tpl
var webSearchDescriptionTmpl []byte

var webSearchDescriptionTpl = template.Must(
	template.New("webSearchDescription").
		Parse(string(webSearchDescriptionTmpl)),
)

type WebSearchOptions struct {
	DefaultEngine config.SearchEngine
	ExaAPIKey     string
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
		renderToolDescription(webSearchDescriptionTpl),
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
				engine = config.SearchEngineExa
			}

			if engine == config.SearchEngineExa {
				results, err := searchExa(ctx, client, opts.ExaAPIKey, params.Query, maxResults)
				if err == nil {
					slog.Debug("Web search completed", "engine", engine, "query", params.Query, "results", len(results))
					return fantasy.NewTextResponse(formatExaSearchResults(results)), nil
				}
				if ctx.Err() != nil {
					return fantasy.ToolResponse{}, ctx.Err()
				}
				slog.Warn("Exa web search failed; falling back to DuckDuckGo", "error", err)
				maybeDelaySearch()
				results, fallbackErr := searchDuckDuckGo(ctx, client, params.Query, maxResults)
				if fallbackErr != nil {
					return fantasy.NewTextErrorResponse("Failed to search with Exa and DuckDuckGo: " + fallbackErr.Error()), nil
				}
				return fantasy.NewTextResponse("[Exa search failed; fell back to DuckDuckGo.]\n\n" + formatSearchResults(results)), nil
			}

			maybeDelaySearch()
			results, err := searchDuckDuckGo(ctx, client, params.Query, maxResults)
			slog.Debug("Web search completed", "engine", engine, "query", params.Query, "results", len(results), "err", err)
			if err != nil {
				return fantasy.NewTextErrorResponse("Failed to search: " + err.Error()), nil
			}
			return fantasy.NewTextResponse(formatSearchResults(results)), nil
		},
	)
}

func formatExaSearchResults(results []SearchResult) string {
	return truncateBytes(formatSearchResults(results), exaOutputLimit)
}
