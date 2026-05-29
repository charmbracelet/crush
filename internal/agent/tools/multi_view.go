package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/taigrr/fantasy"
)

const (
	MultiViewToolName    = "multi_view"
	maxMultiViewFiles    = 20
	multiViewConcurrency = 8
)

//go:embed multi_view.md
var multiViewDescription string

// MultiViewParams takes only paths. Range/limit selection isn't exposed
// here on purpose — that level of control belongs in the single-file
// `view` tool. The agent uses this for "load these N files at once" and
// switches to `view` when it needs a windowed read.
type MultiViewParams struct {
	FilePaths []string `json:"file_paths" description:"Absolute or workspace-relative paths to read"`
}

// MultiViewFileResult is one entry in the response: the path the agent
// asked for, and either content or an error.
type MultiViewFileResult struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

// MultiViewResponse is the parallel envelope returned to the agent.
type MultiViewResponse struct {
	Results []MultiViewFileResult `json:"results"`
}

// NewMultiViewTool returns the multi_view tool. It delegates to the
// supplied single-file view tool, fanning out reads concurrently and
// preserving input order in the response. Failures on individual paths
// are surfaced per-entry rather than aborting the batch — the agent can
// then act on the files that did read successfully.
func NewMultiViewTool(viewTool fantasy.AgentTool) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		MultiViewToolName,
		multiViewDescription,
		func(ctx context.Context, params MultiViewParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if len(params.FilePaths) == 0 {
				return fantasy.NewTextErrorResponse("file_paths is required"), nil
			}
			if len(params.FilePaths) > maxMultiViewFiles {
				return fantasy.NewTextErrorResponse(fmt.Sprintf(
					"too many files: requested %d, limit is %d. Read in batches.",
					len(params.FilePaths), maxMultiViewFiles)), nil
			}

			results := make([]MultiViewFileResult, len(params.FilePaths))
			sem := make(chan struct{}, multiViewConcurrency)
			var wg sync.WaitGroup
			for i, p := range params.FilePaths {
				wg.Add(1)
				sem <- struct{}{}
				go func(i int, p string) {
					defer wg.Done()
					defer func() { <-sem }()
					results[i] = readOne(ctx, viewTool, p)
				}(i, p)
			}
			wg.Wait()

			return fantasy.NewTextResponse(formatMultiView(results)), nil
		},
	)
}

// readOne invokes the underlying view tool for one path. We marshal the
// path into a ViewParams JSON blob so we don't depend on the tool's
// internal representation beyond its public schema.
func readOne(ctx context.Context, viewTool fantasy.AgentTool, path string) MultiViewFileResult {
	input, err := json.Marshal(ViewParams{FilePath: path})
	if err != nil {
		return MultiViewFileResult{Path: path, Error: fmt.Sprintf("failed to marshal input: %s", err)}
	}
	resp, err := viewTool.Run(ctx, fantasy.ToolCall{
		Name:  ViewToolName,
		Input: string(input),
	})
	if err != nil {
		return MultiViewFileResult{Path: path, Error: err.Error()}
	}
	if resp.IsError {
		return MultiViewFileResult{Path: path, Error: resp.Content}
	}
	return MultiViewFileResult{Path: path, Content: resp.Content}
}

// formatMultiView renders the per-file results with a delimiter the
// agent can scan reliably. Errors are clearly prefixed so they don't
// look like content.
func formatMultiView(results []MultiViewFileResult) string {
	var b strings.Builder
	for i, r := range results {
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "===== %s =====\n", r.Path)
		if r.Error != "" {
			fmt.Fprintf(&b, "ERROR: %s\n", r.Error)
			continue
		}
		b.WriteString(r.Content)
		if !strings.HasSuffix(r.Content, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}
