package tools

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
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

//go:embed sourcegraph.md.tpl
var sourcegraphDescriptionTmpl []byte

var sourcegraphDescriptionTpl = template.Must(
	template.New("sourcegraphDescription").
		Parse(string(sourcegraphDescriptionTmpl)),
)

type sourcegraphDescriptionData struct {
	MaxResults int
}

func sourcegraphDescription() string {
	return renderTemplate(sourcegraphDescriptionTpl, sourcegraphDescriptionData{
		MaxResults: 20,
	})
}

func NewSourcegraphTool(client *http.Client) fantasy.AgentTool {
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
		SourcegraphToolName,
		sourcegraphDescription(),
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

			formattedResults, err := formatSourcegraphResults(result, params.ContextWindow, params.Count)
			if err != nil {
				return fantasy.NewTextErrorResponse("Failed to format results: " + err.Error()), nil
			}

			return fantasy.NewTextResponse(formattedResults), nil
		},
	)
}

func formatSourcegraphResults(result map[string]any, contextWindow, maxResults int) (string, error) {
	var buffer strings.Builder

	if writeSourcegraphErrors(&buffer, result) {
		return buffer.String(), nil
	}

	searchResults, err := sourcegraphSearchResults(result)
	if err != nil {
		return "", err
	}

	writeSourcegraphHeader(&buffer, searchResults)

	results, ok := searchResults["results"].([]any)
	if !ok || len(results) == 0 {
		buffer.WriteString("No results found. Try a different query.\n")
		return buffer.String(), nil
	}

	if len(results) > maxResults {
		results = results[:maxResults]
	}

	for i, res := range results {
		formatSourcegraphResult(&buffer, i, res, contextWindow)
	}

	return buffer.String(), nil
}

func writeSourcegraphErrors(buffer *strings.Builder, result map[string]any) bool {
	errors, ok := result["errors"].([]any)
	if !ok || len(errors) == 0 {
		return false
	}

	buffer.WriteString("## Sourcegraph API Error\n\n")
	for _, err := range errors {
		errMap, ok := err.(map[string]any)
		if !ok {
			continue
		}
		message, ok := errMap["message"].(string)
		if ok {
			fmt.Fprintf(buffer, "- %s\n", message)
		}
	}
	return true
}

func sourcegraphSearchResults(result map[string]any) (map[string]any, error) {
	data, ok := result["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing data field")
	}

	search, ok := data["search"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing search field")
	}

	searchResults, ok := search["results"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing results field")
	}
	return searchResults, nil
}

func writeSourcegraphHeader(buffer *strings.Builder, searchResults map[string]any) {
	matchCount, _ := searchResults["matchCount"].(float64)
	resultCount, _ := searchResults["resultCount"].(float64)
	limitHit, _ := searchResults["limitHit"].(bool)

	buffer.WriteString("# Sourcegraph Search Results\n\n")
	fmt.Fprintf(buffer, "Found %d matches across %d results\n", int(matchCount), int(resultCount))

	if limitHit {
		buffer.WriteString("(Result limit reached, try a more specific query)\n")
	}

	buffer.WriteString("\n")
}

func formatSourcegraphResult(buffer *strings.Builder, index int, res any, contextWindow int) {
	fileMatch, ok := res.(map[string]any)
	if !ok {
		return
	}

	typeName, _ := fileMatch["__typename"].(string)
	if typeName != "FileMatch" {
		return
	}

	repo, _ := fileMatch["repository"].(map[string]any)
	file, _ := fileMatch["file"].(map[string]any)
	lineMatches, _ := fileMatch["lineMatches"].([]any)

	if repo == nil || file == nil {
		return
	}

	repoName, _ := repo["name"].(string)
	filePath, _ := file["path"].(string)
	fileURL, _ := file["url"].(string)
	fileContent, _ := file["content"].(string)

	fmt.Fprintf(buffer, "## Result %d: %s/%s\n\n", index+1, repoName, filePath)

	if fileURL != "" {
		fmt.Fprintf(buffer, "URL: %s\n\n", fileURL)
	}

	formatSourcegraphLineMatches(buffer, lineMatches, fileContent, contextWindow)
}

func formatSourcegraphLineMatches(buffer *strings.Builder, lineMatches []any, fileContent string, contextWindow int) {
	for _, lm := range lineMatches {
		lineMatch, ok := lm.(map[string]any)
		if !ok {
			continue
		}
		formatSourcegraphLineMatch(buffer, lineMatch, fileContent, contextWindow)
	}
}

func formatSourcegraphLineMatch(buffer *strings.Builder, lineMatch map[string]any, fileContent string, contextWindow int) {
	lineNumber, _ := lineMatch["lineNumber"].(float64)
	preview, _ := lineMatch["preview"].(string)
	line := int(lineNumber)

	buffer.WriteString("```\n")
	if fileContent == "" {
		fmt.Fprintf(buffer, "%d| %s\n", line, preview)
		buffer.WriteString("```\n\n")
		return
	}

	lines := strings.Split(fileContent, "\n")
	startLine := max(1, line-contextWindow)
	for j := startLine - 1; j < line-1 && j < len(lines); j++ {
		if j >= 0 {
			fmt.Fprintf(buffer, "%d| %s\n", j+1, lines[j])
		}
	}

	fmt.Fprintf(buffer, "%d|  %s\n", line, preview)

	endLine := line + contextWindow
	for j := line; j < endLine && j < len(lines); j++ {
		fmt.Fprintf(buffer, "%d| %s\n", j+1, lines[j])
	}
	buffer.WriteString("```\n\n")
}
