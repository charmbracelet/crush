package agent

import (
	"errors"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func makeStep(calls []fantasy.ToolCallContent, results []fantasy.ToolResultContent) fantasy.StepResult {
	var content fantasy.ResponseContent
	for _, call := range calls {
		content = append(content, call)
	}
	for _, result := range results {
		content = append(content, result)
	}
	return fantasy.StepResult{Response: fantasy.Response{Content: content}}
}

func TestFailureClass(t *testing.T) {
	t.Parallel()

	require.Equal(t, "package-not-found", failureClass("npm error code E404"))
	require.Equal(t, "resource-not-found", failureClass("GET https://api.example.test/repos/guessed: 404 Not Found"))
	require.Equal(t, "executable-not-found", failureClass(`"head": executable file not found in $PATH`))
	require.Equal(t, "invalid-json", failureClass("SyntaxError: invalid JSON"))
	require.Equal(t, "unsupported-schema", failureClass("unsupported MCP type"))
	require.Equal(t, "timeout", failureClass("context deadline exceeded"))
	require.Empty(t, failureClass("operation completed"))
}

func TestToolResultOutputString(t *testing.T) {
	t.Parallel()

	require.Equal(t, "text", toolResultOutputString(fantasy.ToolResultOutputContentText{Text: "text"}))
	require.Equal(t, "failed", toolResultOutputString(fantasy.ToolResultOutputContentError{Error: errors.New("failed")}))
	require.Equal(t, "data", toolResultOutputString(fantasy.ToolResultOutputContentMedia{Data: "data"}))
	require.Empty(t, toolResultOutputString(nil))
}
