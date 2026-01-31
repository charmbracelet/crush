package tools

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"charm.land/fantasy"
)

type SourcegraphParams struct {
	Query         string `json:"query" description:"The Sourcegraph search query"`
	Count         int    `json:"count,omitempty" description:"Optional number of results to return (default: 10, max: 20)"`
	ContextWindow int    `json:"context_window,omitempty" description:"The context around the match to return (default: 10 lines)"`
	Timeout       int    `json:"timeout,omitempty" description:"Optional timeout in seconds (max 120)"`
}

type SourcegraphResponseMetadata struct {
	NumberOfMatches int  `json:"number_of_matches"`
	Truncated       bool `json:"truncated"`
}

const SourcegraphToolName = "sourcegraph"

//go:embed sourcegraph.md
var sourcegraphDescription []byte

func NewSourcegraphTool(client *http.Client) fantasy.AgentTool {
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}
	return fantasy.NewParallelAgentTool(
		SourcegraphToolName,
		string(sourcegraphDescription),
		func(ctx context.Context, params SourcegraphParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Query == "" {
				return fantasy.NewTextErrorResponse("Query parameter is required"), nil
			}

			if params.Count <= 0 {
				params.Count = 10
			} else if params.Count > 20 {
				params.Count = 20 // Limit to 20 results
			}

			if params.ContextWindow <= 0 {
				params.ContextWindow = 10 // Default context window
			}

			// Handle timeout with context
			requestCtx := ctx
			if params.Timeout > 0 {
				maxTimeout := 120 // 2 minutes
				if params.Timeout > maxTimeout {
					params.Timeout = maxTimeout
				}
				var cancel context.CancelFunc
				requestCtx, cancel = context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Second)
				defer cancel()
			}

			type graphqlRequest struct {
				Query     string `json:"query"`
				Variables struct {
					Query string `json:"query"`
				} `json:"variables"`
			}

			request := graphqlRequest{
				Query: "query Search($query: String!) { search(query: $query, version: V2, patternType: keyword ) { results { matchCount, limitHit, resultCount, approximateResultCount, missing { name }, timedout { name }, indexUnavailable, results { __typename, ... on FileMatch { repository { name }, file { path, url, content }, lineMatches { preview, lineNumber, offsetAndLengths } } } } } }",
			}
			request.Variables.Query = params.Query

			graphqlQueryBytes, err := json.Marshal(request)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to marshal GraphQL request: %w", err)
			}
			graphqlQuery := string(graphqlQueryBytes)

			req, err := http.NewRequestWithContext(
				requestCtx,
				"POST",
				"https://sourcegraph.com/.api/graphql",
				bytes.NewBuffer([]byte(graphqlQuery)),
			)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to create request: %w", err)
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "crush/1.0")

			resp, err := client.Do(req)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to fetch URL: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				if len(body) > 0 {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("Request failed with status code: %d, response: %s", resp.StatusCode, string(body))), nil
				}

				return fantasy.NewTextErrorResponse(fmt.Sprintf("Request failed with status code: %d", resp.StatusCode)), nil
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to read response body: %w", err)
			}

			var result map[string]any
			if err = json.Unmarshal(body, &result); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to unmarshal response: %w", err)
			}

			formattedResults, err := formatSourcegraphResults(result, params.ContextWindow)
			if err != nil {
				return fantasy.NewTextErrorResponse("Failed to format results: " + err.Error()), nil
			}

			return fantasy.NewTextResponse(TruncateOutputCtx(ctx, formattedResults)), nil
		})
}

func formatSourcegraphResults(result map[string]any, contextWindow int) (string, error) {
	var buffer strings.Builder

	if errors, ok := result["errors"].([]any); ok && len(errors) > 0 {
		buffer.WriteString("## Sourcegraph API Error\n\n")
		for _, err := range errors {
			if errMap, ok := err.(map[string]any); ok {
				if message, ok := errMap["message"].(string); ok {
					buffer.WriteString(fmt.Sprintf("- %s\n", message))
				}
			}
		}
		return buffer.String(), nil
	}

	data, ok := result["data"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid response format: missing data field")
	}

	search, ok := data["search"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid response format: missing search field")
	}

	searchResults, ok := search["results"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid response format: missing results field")
	}

	matchCount, _ := searchResults["matchCount"].(float64)
	resultCount, _ := searchResults["resultCount"].(float64)
	limitHit, _ := searchResults["limitHit"].(bool)

	buffer.WriteString("# Sourcegraph Search Results\n\n")
	buffer.WriteString(fmt.Sprintf("Found %d matches across %d results\n", int(matchCount), int(resultCount)))

	if limitHit {
		buffer.WriteString("(Result limit reached, try a more specific query)\n")
	}

	buffer.WriteString("\n")

	results, ok := searchResults["results"].([]any)
	if !ok || len(results) == 0 {
		buffer.WriteString("No results found. Try a different query.\n")
		return buffer.String(), nil
	}

	maxResults := 10
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	for i, res := range results {
		fileMatch, ok := res.(map[string]any)
		if !ok {
			continue
		}

		typeName, _ := fileMatch["__typename"].(string)
		if typeName != "FileMatch" {
			continue
		}

		repo, _ := fileMatch["repository"].(map[string]any)
		file, _ := fileMatch["file"].(map[string]any)
		lineMatches, _ := fileMatch["lineMatches"].([]any)

		if repo == nil || file == nil {
			continue
		}

		repoName, _ := repo["name"].(string)
		filePath, _ := file["path"].(string)
		fileURL, _ := file["url"].(string)
		fileContent, _ := file["content"].(string)

		buffer.WriteString(fmt.Sprintf("## Result %d: %s/%s\n\n", i+1, repoName, filePath))

		if fileURL != "" {
			buffer.WriteString(fmt.Sprintf("URL: %s\n\n", fileURL))
		}

		if len(lineMatches) > 0 {
			for _, lm := range lineMatches {
				lineMatch, ok := lm.(map[string]any)
				if !ok {
					continue
				}

				lineNumber, _ := lineMatch["lineNumber"].(float64)
				preview, _ := lineMatch["preview"].(string)

				if fileContent != "" {
					lines := strings.Split(fileContent, "\n")

					buffer.WriteString("```\n")

					startLine := max(1, int(lineNumber)-contextWindow)

					for j := startLine - 1; j < int(lineNumber)-1 && j < len(lines); j++ {
						if j >= 0 {
							buffer.WriteString(fmt.Sprintf("%d| %s\n", j+1, lines[j]))
						}
					}

					buffer.WriteString(fmt.Sprintf("%d|  %s\n", int(lineNumber), preview))

					endLine := int(lineNumber) + contextWindow

					for j := int(lineNumber); j < endLine && j < len(lines); j++ {
						if j < len(lines) {
							buffer.WriteString(fmt.Sprintf("%d| %s\n", j+1, lines[j]))
						}
					}

					buffer.WriteString("```\n\n")
				} else {
					buffer.WriteString("```\n")
					buffer.WriteString(fmt.Sprintf("%d| %s\n", int(lineNumber), preview))
					buffer.WriteString("```\n\n")
				}
			}
		}
	}

	return buffer.String(), nil
}
