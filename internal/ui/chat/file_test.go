package chat

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestViewToolRenderContextRendersSanitizedResultAsPlainText(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles()
	renderer := &ViewToolRenderContext{}

	viewMeta, err := json.Marshal(tools.ViewResponseMetadata{
		FilePath: "/tmp/test.go",
		Content:  "<file>\n     1|package main\n</file>",
	})
	require.NoError(t, err)

	result := (&message.ToolResult{
		Name:     tools.ViewToolName,
		Content:  "<file>\n     1|package main\n</file>",
		Metadata: string(viewMeta),
	}).WithAutoReview(message.ToolResultAutoReview{
		Suspicious: true,
		Sanitized:  true,
		Reason:     "Tool output matched local prompt-injection heuristics (assistant message).",
	})

	out := renderer.RenderTool(&sty, 120, &ToolRenderOpts{
		ToolCall: message.ToolCall{
			Name:  tools.ViewToolName,
			Input: `{"file_path":"/tmp/test.go","offset":0,"limit":20}`,
		},
		Result:     &result,
		Status:     ToolStatusSuccess,
		IsSpinning: false,
	})

	require.Contains(t, out, "Tool output was withheld from the model")
	require.Contains(t, out, "Reason: Tool output matched local prompt-injection heuristics")
	require.NotContains(t, out, "<file>")
	require.NotContains(t, out, "|package main")
}

func TestMultiEditToolRenderContextParsesMetadataWithoutAutoReviewCollision(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles()
	renderer := &MultiEditToolRenderContext{}

	metaJSON, err := json.Marshal(tools.MultiEditResponseMetadata{
		OldContent:   "line1\n",
		NewContent:   "line1\nline2\n",
		Additions:    1,
		Removals:     0,
		EditsApplied: 1,
	})
	require.NoError(t, err)

	result := message.ToolResult{
		Name:     tools.MultiEditToolName,
		Content:  "Applied 1 edits to file: /tmp/test.go",
		Metadata: string(metaJSON),
	}

	out := renderer.RenderTool(&sty, 120, &ToolRenderOpts{
		ToolCall: message.ToolCall{
			Name:  tools.MultiEditToolName,
			Input: `{"file_path":"/tmp/test.go","edits":[{"old_string":"line1","new_string":"line1\nline2"}]}`,
		},
		Result:     &result,
		Status:     ToolStatusSuccess,
		IsSpinning: false,
	})

	require.NotContains(t, out, "Applied 1 edits to file")
	require.Contains(t, out, "line2")
}

func TestViewToolRenderContextRendersSanitizedResultWithWindowsPath(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles()
	renderer := &ViewToolRenderContext{}

	viewMeta, err := json.Marshal(tools.ViewResponseMetadata{
		FilePath: `C:\\Users\\dev\\project\\test.go`,
		Content:  "<file>\n     1|package main\n</file>",
	})
	require.NoError(t, err)

	result := (&message.ToolResult{
		Name:     tools.ViewToolName,
		Content:  "<file>\n     1|package main\n</file>",
		Metadata: string(viewMeta),
	}).WithAutoReview(message.ToolResultAutoReview{
		Suspicious: true,
		Sanitized:  true,
		Reason:     "windows path case",
	})

	out := renderer.RenderTool(&sty, 120, &ToolRenderOpts{
		ToolCall: message.ToolCall{
			Name:  tools.ViewToolName,
			Input: `{"file_path":"C:\\Users\\dev\\project\\test.go","offset":0,"limit":20}`,
		},
		Result:     &result,
		Status:     ToolStatusSuccess,
		IsSpinning: false,
	})

	require.Contains(t, out, "Tool output was withheld from the model")
	require.Contains(t, out, "Reason: windows path case")
	require.Contains(t, out, `C:\Users\dev\project\test.go`)
	require.NotContains(t, out, "|package main")
}

func TestHashlineEditToolRenderContextRendersDiffFromMetadata(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles()
	renderer := &HashlineEditToolRenderContext{}

	metaJSON, err := json.Marshal(tools.HashlineEditResponseMetadata{
		OldContent: "line1\n",
		NewContent: "line1\nline2\n",
		Additions:  1,
		Removals:   0,
	})
	require.NoError(t, err)

	result := message.ToolResult{
		Name:     tools.HashlineEditToolName,
		Content:  "Applied hashline edit to file: /tmp/test.go",
		Metadata: string(metaJSON),
	}

	out := renderer.RenderTool(&sty, 120, &ToolRenderOpts{
		ToolCall: message.ToolCall{
			Name:  tools.HashlineEditToolName,
			Input: `{"file_path":"/tmp/test.go"}`,
		},
		Result:     &result,
		Status:     ToolStatusSuccess,
		IsSpinning: false,
	})

	require.NotContains(t, out, "Applied hashline edit to file")
	require.Contains(t, out, "line2")
}
