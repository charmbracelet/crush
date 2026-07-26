package format

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrintToolCall_FinishedOnly(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	// Unfinished tool call should not print.
	p.PrintToolCall("bash", "tc1", `{"command":"ls"}`, false)
	require.Empty(t, buf.String())

	// Finished tool call should print.
	p.PrintToolCall("bash", "tc1", `{"command":"ls -la"}`, true)
	require.Equal(t, "-> bash: ls -la\n", buf.String())
}

func TestPrintToolCall_Deduplicates(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	p.PrintToolCall("bash", "tc1", `{"command":"ls"}`, true)
	p.PrintToolCall("bash", "tc1", `{"command":"ls"}`, true)
	require.Equal(t, "-> bash: ls\n", buf.String())
}

func TestPrintToolCall_EachTool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		toolName string
		input    string
		want     string
	}{
		{"bash", "bash", `{"command":"echo hi"}`, "-> bash: echo hi\n"},
		{"edit", "edit", `{"file_path":"/tmp/main.go"}`, "-> edit: /tmp/main.go\n"},
		{"view", "view", `{"file_path":"/src/foo.go"}`, "-> view: /src/foo.go\n"},
		{"glob", "glob", `{"pattern":"*.go"}`, "-> glob: *.go\n"},
		{"grep", "grep", `{"pattern":"TODO"}`, "-> grep: TODO\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			buf := &bytes.Buffer{}
			p := NewEventPrinter(buf)
			p.PrintToolCall(tc.toolName, "id1", tc.input, true)
			require.Equal(t, tc.want, buf.String())
		})
	}
}

func TestPrintToolCall_Truncates(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	longCmd := string(bytes.Repeat([]byte("a"), 100))
	p.PrintToolCall("bash", "tc1", `{"command":"`+longCmd+`"}`, true)
	out := buf.String()
	require.Contains(t, out, "-> bash: ")
	// "-> bash: " (9 chars) + truncated input (60 chars) + "\n" = 70
	require.Len(t, out, 70)
}

func TestPrintToolCall_InvalidJSON(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)
	p.PrintToolCall("bash", "tc1", "not json", true)
	require.Contains(t, buf.String(), "-> bash: not json")
}

func TestPrintToolCall_EmptyInput(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)
	p.PrintToolCall("bash", "tc1", "", true)
	require.Equal(t, "-> bash: \n", buf.String())
}

func TestPrintToolResult_Success(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	p.PrintToolResult("bash", "3 files found", false)
	require.Equal(t, "ok bash: 3 files found\n", buf.String())
}

func TestPrintToolResult_SuccessEmpty(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	p.PrintToolResult("view", "", false)
	require.Equal(t, "ok view\n", buf.String())
}

func TestPrintToolResult_Error(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	p.PrintToolResult("bash", "command not found", true)
	require.Equal(t, "x  bash: command not found\n", buf.String())
}

func TestPrintToolResult_LongContentTruncated(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	long := string(bytes.Repeat([]byte("x"), 200))
	p.PrintToolResult("bash", long, false)
	out := buf.String()
	require.Contains(t, out, "ok bash: ")
	require.Less(t, len(out), 80)
}

func TestPrintAssistantText(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	p.PrintAssistantText("Let me check that file.")
	require.Equal(t, ">> Let me check that file.\n", buf.String())
}

func TestPrintAssistantText_FirstLineOnly(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	p.PrintAssistantText("Line one.\nLine two.\nLine three.")
	require.Equal(t, ">> Line one.\n", buf.String())
}

func TestPrintAssistantText_Truncates(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	long := string(bytes.Repeat([]byte("a"), 200))
	p.PrintAssistantText(long)
	out := buf.String()
	require.Less(t, len(out), 85)
}

func TestPrintAssistantText_Empty(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	p.PrintAssistantText("")
	p.PrintAssistantText("   \n  ")
	require.Empty(t, buf.String())
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	require.Equal(t, "short", truncate("short", 10))
	require.Equal(t, "abcdef", truncate("abcdef", 6))  // exactly fits
	require.Equal(t, "abc...", truncate("abcdefg", 6)) // needs truncation
	require.Equal(t, "ab", truncate("abcdef", 2))
}

func TestFirstLine(t *testing.T) {
	t.Parallel()

	require.Equal(t, "hello", firstLine("hello\nworld", 80))
	require.Equal(t, "", firstLine("  \n  ", 80))
	require.Equal(t, "abc...", firstLine("abcdefghijk", 6))
}
