package tools

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
)

//go:embed web_search.md.tpl
var webSearchDescriptionTmpl []byte

var webSearchDescriptionTpl = template.Must(
	template.New("webSearchDescription").
		Parse(string(webSearchDescriptionTmpl)),
)

// NewWebSearchTool creates a web search tool.
func NewWebSearchTool(permissions permission.Service, workingDir string, client *http.Client) fantasy.AgentTool {
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

			if permissions != nil {
				sessionID := GetSessionFromContext(ctx)
				if sessionID == "" {
					return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for web search")
				}
				granted, err := permissions.Request(ctx, permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        workingDir,
					Resource:    params.Query,
					ToolCallID:  call.ID,
					ToolName:    WebSearchToolName,
					Action:      "search",
					Description: fmt.Sprintf("Search the web for: %s", params.Query),
					Params:      WebSearchPermissionsParams(params),
				})
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !granted {
					return NewPermissionDeniedResponse(), nil
				}
			}

			maxResults := params.MaxResults
			if maxResults <= 0 {
				maxResults = 10
			}
			if maxResults > 20 {
				maxResults = 20
			}

			maybeDelaySearch()
			results, err := searchDuckDuckGo(ctx, client, params.Query, maxResults)
			slog.Debug("Web search completed", "query", params.Query, "results", len(results), "err", err)
			if err != nil {
				return fantasy.NewTextErrorResponse("Failed to search: " + err.Error()), nil
			}
			recordMCPSearchResults(ctx, results)

			return fantasy.NewTextResponse(formatSearchResults(results)), nil
		},
	)
}
