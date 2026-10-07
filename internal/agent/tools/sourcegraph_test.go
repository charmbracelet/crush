package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestFormatSourcegraphResults(t *testing.T) {
	t.Parallel()

	result := map[string]any{
		"data": map[string]any{
			"search": map[string]any{
				"results": map[string]any{
					"matchCount":  float64(2),
					"resultCount": float64(1),
					"limitHit":    true,
					"results": []any{
						map[string]any{
							"__typename": "FileMatch",
							"repository": map[string]any{"name": "owner/repo"},
							"file": map[string]any{
								"path":    "main.go",
								"url":     "https://example.com/owner/repo/main.go",
								"content": "package main\n\nfunc main() {\n\tprintln(\"match\")\n}\n",
							},
							"lineMatches": []any{
								map[string]any{
									"lineNumber": float64(4),
									"preview":    "\tprintln(\"match\")",
								},
							},
						},
					},
				},
			},
		},
	}

	got, err := formatSourcegraphResults(result, 1, 10)
	require.NoError(t, err)
	require.Contains(t, got, "# Sourcegraph Search Results")
	require.Contains(t, got, "Found 2 matches across 1 results")
	require.Contains(t, got, "(Result limit reached, try a more specific query)")
	require.Contains(t, got, "## Result 1: owner/repo/main.go")
	require.Contains(t, got, "URL: https://example.com/owner/repo/main.go")
	require.Contains(t, got, "3| func main() {")
	require.Contains(t, got, "4|  \tprintln(\"match\")")
	require.Contains(t, got, "5| }")
}

func TestFormatSourcegraphResultsRespectsCount(t *testing.T) {
	t.Parallel()

	result := map[string]any{
		"data": map[string]any{
			"search": map[string]any{
				"results": map[string]any{
					"results": []any{
						map[string]any{
							"__typename": "FileMatch",
							"repository": map[string]any{"name": "owner/repo"},
							"file":       map[string]any{"path": "first.go"},
						},
						map[string]any{
							"__typename": "FileMatch",
							"repository": map[string]any{"name": "owner/repo"},
							"file":       map[string]any{"path": "second.go"},
						},
					},
				},
			},
		},
	}

	got, err := formatSourcegraphResults(result, 1, 1)
	require.NoError(t, err)
	require.Contains(t, got, "owner/repo/first.go")
	require.NotContains(t, got, "owner/repo/second.go")
}

func TestFormatSourcegraphResultsErrorsAndNoResults(t *testing.T) {
	t.Parallel()

	errorResult := map[string]any{
		"errors": []any{
			map[string]any{"message": "bad query"},
			map[string]any{"message": "timeout"},
		},
	}
	got, err := formatSourcegraphResults(errorResult, 1, 10)
	require.NoError(t, err)
	require.Equal(t, "## Sourcegraph API Error\n\n- bad query\n- timeout\n", got)

	noResult := map[string]any{
		"data": map[string]any{
			"search": map[string]any{
				"results": map[string]any{
					"matchCount":  float64(0),
					"resultCount": float64(0),
					"results":     []any{},
				},
			},
		},
	}
	got, err = formatSourcegraphResults(noResult, 1, 10)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(got, "No results found. Try a different query.\n"))
}

type erroringRoundTripper struct {
	err error
}

func (rt erroringRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, rt.err
}

func runSourcegraphTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params SourcegraphParams) (fantasy.ToolResponse, error) {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	call := fantasy.ToolCall{
		ID:    "test-call",
		Name:  SourcegraphToolName,
		Input: string(input),
	}

	return tool.Run(ctx, call)
}

func TestSourcegraphToolNetworkError(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: erroringRoundTripper{err: errors.New("dial tcp: i/o timeout")},
	}
	tool := NewSourcegraphTool(client)

	resp, err := runSourcegraphTool(t, tool, context.Background(), SourcegraphParams{
		Query: "context cancellation",
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "Failed to fetch URL")
}

func TestSourcegraphToolContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tool := NewSourcegraphTool(nil)

	resp, err := runSourcegraphTool(t, tool, ctx, SourcegraphParams{
		Query: "context cancellation",
	})
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, resp.IsError)
}
