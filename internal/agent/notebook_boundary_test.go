package agent

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestEstimateRawMessageTokens_CountsStubNotOriginal(t *testing.T) {
	// An applied superseded mark renders the stub, not the stored
	// original — the estimator must count what the model sees or the
	// raw window underfills by the stubbed bytes.
	big := strings.Repeat("file content line\n", 500) // ~9KB.
	result := message.ToolResult{ToolCallID: "tc-1", Name: "view", Content: big}
	pendingResult := result
	pendingResult.Superseded = &message.SupersededMark{Path: "a.go", ByTool: "edit", Turn: 2}
	pending := []message.Message{{Role: message.Tool, Parts: []message.ContentPart{pendingResult}}}

	appliedResult := result
	appliedResult.Superseded = &message.SupersededMark{Path: "a.go", ByTool: "edit", Turn: 2, Applied: true}
	applied := []message.Message{{Role: message.Tool, Parts: []message.ContentPart{appliedResult}}}

	require.Equal(t, len(big)/4, estimateRawMessageTokens(pending))
	stubLen := len(appliedResult.Superseded.StubText(appliedResult))
	require.Equal(t, stubLen/4, estimateRawMessageTokens(applied))
}

func TestEstimateRawMessageTokens(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello world"}}},
	}
	tokens := estimateRawMessageTokens(msgs)
	require.Equal(t, len("hello world")/4, tokens)
}

func TestEstimateRawMessageTokens_Empty(t *testing.T) {
	tokens := estimateRawMessageTokens(nil)
	require.Equal(t, 0, tokens)
}

func TestCountUserMessages(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "second"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "response"}}},
	}
	require.Equal(t, 2, countUserMessages(msgs))
}

func TestExtractExplicitFilePaths(t *testing.T) {
	// Full paths with separators are extracted.
	refs := extractExplicitFilePaths("fix the bug in internal/middleware/auth.go")
	require.Contains(t, refs, "file:auth.go")
}

func TestExtractExplicitFilePaths_NoBareFilenames(t *testing.T) {
	// Bare filenames without separators should not match.
	refs := extractExplicitFilePaths("fix the bug in auth.go")
	require.NotContains(t, refs, "file:auth.go")
}

func TestExtractExplicitFilePaths_MultiplePaths(t *testing.T) {
	refs := extractExplicitFilePaths("edit internal/middleware/auth.go and internal/config/load.go")
	require.Contains(t, refs, "file:auth.go")
	require.Contains(t, refs, "file:load.go")
}

func TestExtractExplicitFilePaths_Deduplicates(t *testing.T) {
	refs := extractExplicitFilePaths("edit internal/middleware/auth.go and internal/auth.go")
	// Same basename should only appear once.
	count := 0
	for _, r := range refs {
		if r == "file:auth.go" {
			count++
		}
	}
	require.Equal(t, 1, count, "file:auth.go should appear only once")
}

func TestExtractExplicitFilePaths_RelativePath(t *testing.T) {
	refs := extractExplicitFilePaths("fix ./src/main.go")
	require.Contains(t, refs, "file:main.go")
}

func TestExtractExplicitFilePaths_StripsTrailingPunctuation(t *testing.T) {
	// Trailing periods, commas, etc. should be stripped.
	refs := extractExplicitFilePaths("fix internal/middleware/auth.go.")
	require.Contains(t, refs, "file:auth.go")
	require.NotContains(t, refs, "file:auth.go.")

	refs2 := extractExplicitFilePaths("edit internal/config/load.go, then test")
	require.Contains(t, refs2, "file:load.go")
	require.NotContains(t, refs2, "file:load.go,")
}
