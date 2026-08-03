package tools

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/semantic"
)

const SemanticSearchToolName = "semantic_search"

// SemanticSearchParams are the parameters for the semantic_search tool.
type SemanticSearchParams struct {
	Query string `json:"query" jsonschema:"description=Natural language query describing what code or concept to find. Use this for behavioral or conceptual searches like 'where is authentication handled' or 'what processes webhook events'. For exact symbol names or string literals use grep instead."`
	Limit int    `json:"limit,omitempty" jsonschema:"description=Maximum number of results to return (default 10)"`
}

// NewSemanticSearchTool creates a semantic search tool backed by the
// given semantic store. If store is nil the tool returns an error
// indicating the index is not available.
func NewSemanticSearchTool(store *semantic.Store, client *semantic.Client) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		SemanticSearchToolName,
		semanticSearchDescription(),
		func(ctx context.Context, params SemanticSearchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if store == nil || client == nil {
				return fantasy.NewTextErrorResponse("Semantic search index is not configured. Set up an embedding provider in crush.json to enable semantic search."), nil
			}

			if strings.TrimSpace(params.Query) == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}

			limit := params.Limit
			if limit <= 0 {
				limit = 10
			}

			embeddings, err := client.Embed(ctx, []string{params.Query})
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to generate query embedding: %v", err)), nil
			}
			// Embed guarantees one vector per input on success, but a
			// panic here would take down the whole tool call rather than
			// returning an error the agent can react to.
			if len(embeddings) == 0 {
				return fantasy.NewTextErrorResponse("Embedding provider returned no vector for the query."), nil
			}

			results, err := store.Search(ctx, embeddings[0], limit)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Search failed: %v", err)), nil
			}

			if len(results) == 0 {
				count, _ := store.ChunkCount(ctx)
				if count == 0 {
					return fantasy.NewTextResponse("No indexed chunks found. Run the indexer first to build the semantic search index."), nil
				}
				return fantasy.NewTextResponse("No results found for this query."), nil
			}

			var b strings.Builder
			fmt.Fprintf(&b, "Found %d results:\n\n", len(results))
			for i, r := range results {
				fmt.Fprintf(&b, "%d. %s", i+1, r.Path)
				if r.Symbol != "" {
					fmt.Fprintf(&b, " :: %s", r.Symbol)
				}
				fmt.Fprintf(&b, " (lines %d-%d, score %.3f)\n", r.StartLine+1, r.EndLine+1, r.Score)

				snippet := r.Content
				lines := strings.Split(snippet, "\n")
				if len(lines) > 5 {
					snippet = strings.Join(lines[:5], "\n") + "\n..."
				}
				fmt.Fprintf(&b, "   %s\n\n", strings.ReplaceAll(snippet, "\n", "\n   "))
			}

			return fantasy.NewTextResponse(b.String()), nil
		},
	)
}

func semanticSearchDescription() string {
	return `Search code semantically using natural language queries. Returns file locations, symbol names, line ranges, and relevance scores — NOT full file content. Use the view tool to read authoritative content after finding relevant locations.

Use this tool when searching for behavior, concepts, or patterns described in natural language (e.g., "where is authentication handled", "what code processes payments", "error handling for network failures").

Do NOT use this tool when you know the exact symbol name, function name, variable, or string literal to search for — use grep instead, which is faster and more precise for exact matches.

The index may be stale between indexing runs. Always verify results against the actual file content using the view tool.`
}
