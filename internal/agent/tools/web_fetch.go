package tools

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"charm.land/fantasy"
)

//go:embed web_fetch.md
var webFetchToolDescription []byte

// NewWebFetchTool creates a simple web fetch tool for sub-agents (no permissions needed).
func NewWebFetchTool(workingDir string, client *http.Client) fantasy.AgentTool {
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
		WebFetchToolName,
		string(webFetchToolDescription),
		func(ctx context.Context, params WebFetchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.URL == "" {
				return fantasy.NewTextErrorResponse("url is required"), nil
			}

			content, err := FetchURLAndConvert(ctx, client, params.URL)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to fetch URL: %s", err)), nil
			}

			hasLargeContent := len(content) > LargeContentThreshold
			var result strings.Builder

			if hasLargeContent {
				tempFile, err := os.CreateTemp(workingDir, "page-*.md")
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to create temporary file: %s", err)), nil
				}
				tempFilePath := tempFile.Name()

				if _, err := tempFile.WriteString(content); err != nil {
					_ = tempFile.Close() // Best effort close
					return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to write content to file: %s", err)), nil
				}
				if err := tempFile.Close(); err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to close temporary file: %s", err)), nil
				}

				result.WriteString(fmt.Sprintf("Fetched content from %s (large page)\n\n", params.URL))
				result.WriteString(fmt.Sprintf("Content saved to: %s\n\n", tempFilePath))
				result.WriteString("Use the view and grep tools to analyze this file.")
			} else {
				result.WriteString(fmt.Sprintf("Fetched content from %s:\n\n", params.URL))
				result.WriteString(content)
			}

			return fantasy.NewTextResponse(result.String()), nil
		})
}
