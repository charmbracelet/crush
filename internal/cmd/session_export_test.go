package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestOutputSessionMarkdown_BasicConversation(t *testing.T) {
	t.Parallel()

	sess := session.Session{
		ID:               "test-uuid-1234",
		Title:            "Fix the widget parser",
		CreatedAt:        1712000000,
		UpdatedAt:        1712001000,
		Cost:             0.0123,
		PromptTokens:     500,
		CompletionTokens: 200,
	}

	msgs := []*message.Message{
		{
			ID:        "msg-1",
			Role:      message.User,
			SessionID: sess.ID,
			CreatedAt: 1712000000,
			Parts: []message.ContentPart{
				message.TextContent{Text: "How do I fix the widget parser?"},
			},
		},
		{
			ID:        "msg-2",
			Role:      message.Assistant,
			SessionID: sess.ID,
			Model:     "claude-opus-4-20250514",
			Provider:  "anthropic",
			CreatedAt: 1712000010,
			Parts: []message.ContentPart{
				message.TextContent{Text: "You need to update the parsing logic in `parser.go`."},
			},
		},
	}

	var buf bytes.Buffer
	err := outputSessionMarkdown(&buf, sess, msgs)
	require.NoError(t, err)

	out := buf.String()

	// Frontmatter.
	require.True(t, strings.HasPrefix(out, "---\n"), "should start with YAML frontmatter")
	require.Contains(t, out, `title: "Fix the widget parser"`)
	require.Contains(t, out, "cost: $0.0123")
	require.Contains(t, out, "tokens: 700")

	// Title.
	require.Contains(t, out, "# Fix the widget parser")

	// User message.
	require.Contains(t, out, "## User")
	require.Contains(t, out, "How do I fix the widget parser?")

	// Assistant message.
	require.Contains(t, out, "## Assistant")
	require.Contains(t, out, "*Model: claude-opus-4-20250514 (anthropic)*")
	require.Contains(t, out, "You need to update the parsing logic")
}

func TestOutputSessionMarkdown_ToolCalls(t *testing.T) {
	t.Parallel()

	sess := session.Session{
		ID:        "test-uuid-5678",
		Title:     "Search the codebase",
		CreatedAt: 1712000000,
		UpdatedAt: 1712001000,
	}

	msgs := []*message.Message{
		{
			ID:        "msg-1",
			Role:      message.User,
			SessionID: sess.ID,
			CreatedAt: 1712000000,
			Parts: []message.ContentPart{
				message.TextContent{Text: "Find all TODO comments"},
			},
		},
		{
			ID:        "msg-2",
			Role:      message.Assistant,
			SessionID: sess.ID,
			Model:     "gpt-5.2",
			CreatedAt: 1712000010,
			Parts: []message.ContentPart{
				message.TextContent{Text: "Let me search for TODO comments."},
				message.ToolCall{
					ID:    "tc-1",
					Name:  "grep",
					Input: `{"pattern": "TODO", "path": "."}`,
				},
			},
		},
		{
			ID:        "msg-3",
			Role:      message.Tool,
			SessionID: sess.ID,
			CreatedAt: 1712000020,
			Parts: []message.ContentPart{
				message.ToolResult{
					ToolCallID: "tc-1",
					Name:       "grep",
					Content:    "main.go:42: // TODO: fix this\nparser.go:10: // TODO: refactor",
				},
			},
		},
	}

	var buf bytes.Buffer
	err := outputSessionMarkdown(&buf, sess, msgs)
	require.NoError(t, err)

	out := buf.String()

	// Tool call.
	require.Contains(t, out, "### Tool Call: `grep`")
	require.Contains(t, out, `"pattern": "TODO"`)

	// Tool result.
	require.Contains(t, out, "### Tool Result: `grep`")
	require.Contains(t, out, "main.go:42")
	require.Contains(t, out, "<details>")
	require.Contains(t, out, "<summary>Output (")
}

func TestOutputSessionMarkdown_Reasoning(t *testing.T) {
	t.Parallel()

	sess := session.Session{
		ID:        "test-uuid-9012",
		Title:     "Thinking session",
		CreatedAt: 1712000000,
		UpdatedAt: 1712001000,
	}

	msgs := []*message.Message{
		{
			ID:        "msg-1",
			Role:      message.User,
			SessionID: sess.ID,
			CreatedAt: 1712000000,
			Parts: []message.ContentPart{
				message.TextContent{Text: "Think hard about this"},
			},
		},
		{
			ID:        "msg-2",
			Role:      message.Assistant,
			SessionID: sess.ID,
			Model:     "claude-opus-4-20250514",
			Provider:  "anthropic",
			CreatedAt: 1712000010,
			Parts: []message.ContentPart{
				message.ReasoningContent{
					Thinking:  "Let me reason about this carefully...",
					StartedAt: 1712000010,
				},
				message.TextContent{Text: "Here is my answer."},
			},
		},
	}

	var buf bytes.Buffer
	err := outputSessionMarkdown(&buf, sess, msgs)
	require.NoError(t, err)

	out := buf.String()

	// Reasoning in collapsible.
	require.Contains(t, out, "<details>")
	require.Contains(t, out, "<summary>Thinking</summary>")
	require.Contains(t, out, "Let me reason about this carefully...")
	require.Contains(t, out, "</details>")
	require.Contains(t, out, "Here is my answer.")
}

func TestOutputSessionMarkdown_SkipsSystemMessages(t *testing.T) {
	t.Parallel()

	sess := session.Session{
		ID:        "test-uuid-skip",
		Title:     "System skip test",
		CreatedAt: 1712000000,
		UpdatedAt: 1712001000,
	}

	msgs := []*message.Message{
		{
			ID:        "msg-sys",
			Role:      message.System,
			SessionID: sess.ID,
			CreatedAt: 1712000000,
			Parts: []message.ContentPart{
				message.TextContent{Text: "You are a helpful assistant."},
			},
		},
		{
			ID:        "msg-1",
			Role:      message.User,
			SessionID: sess.ID,
			CreatedAt: 1712000001,
			Parts: []message.ContentPart{
				message.TextContent{Text: "Hello"},
			},
		},
	}

	var buf bytes.Buffer
	err := outputSessionMarkdown(&buf, sess, msgs)
	require.NoError(t, err)

	out := buf.String()

	require.NotContains(t, out, "You are a helpful assistant")
	require.Contains(t, out, "Hello")
}

func TestOutputSessionMarkdown_ErrorToolResult(t *testing.T) {
	t.Parallel()

	sess := session.Session{
		ID:        "test-uuid-err",
		Title:     "Error test",
		CreatedAt: 1712000000,
		UpdatedAt: 1712001000,
	}

	msgs := []*message.Message{
		{
			ID:        "msg-1",
			Role:      message.Tool,
			SessionID: sess.ID,
			CreatedAt: 1712000000,
			Parts: []message.ContentPart{
				message.ToolResult{
					ToolCallID: "tc-1",
					Name:       "bash",
					Content:    "command not found: foobar",
					IsError:    true,
				},
			},
		},
	}

	var buf bytes.Buffer
	err := outputSessionMarkdown(&buf, sess, msgs)
	require.NoError(t, err)

	out := buf.String()

	require.Contains(t, out, "**Error:**")
	require.Contains(t, out, "command not found: foobar")
}

func TestOutputSessionMarkdown_NoCostOrTokens(t *testing.T) {
	t.Parallel()

	sess := session.Session{
		ID:        "test-uuid-nocost",
		Title:     "No cost session",
		CreatedAt: 1712000000,
		UpdatedAt: 1712001000,
	}

	var buf bytes.Buffer
	err := outputSessionMarkdown(&buf, sess, nil)
	require.NoError(t, err)

	out := buf.String()

	// Should not contain cost or tokens lines when zero.
	require.NotContains(t, out, "cost:")
	require.NotContains(t, out, "tokens:")
}

func TestSessionOutputFormat(t *testing.T) {
	t.Parallel()

	// Explicit --json flag wins over everything.
	require.Equal(t, "json", sessionOutputFormat(true, false, ""))
	require.Equal(t, "json", sessionOutputFormat(true, true, "out.md"))

	// Explicit --markdown flag.
	require.Equal(t, "markdown", sessionOutputFormat(false, true, ""))
	require.Equal(t, "markdown", sessionOutputFormat(false, true, "out.json"))

	// File extension detection (no explicit flags).
	require.Equal(t, "markdown", sessionOutputFormat(false, false, "session.md"))
	require.Equal(t, "markdown", sessionOutputFormat(false, false, "path/to/out.markdown"))
	require.Equal(t, "json", sessionOutputFormat(false, false, "session.json"))

	// No flags, no output file, stdout IS a TTY → human.
	// (In test context os.Stdout is not a terminal, so this falls through to "markdown"
	// for the non-TTY branch. We verify the non-TTY default here.)
	require.Equal(t, "markdown", sessionOutputFormat(false, false, ""))
}
