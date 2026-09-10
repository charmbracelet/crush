package format

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// stripANSI removes ANSI escape sequences so tests can check visible text.
func stripANSI(s string) string {
	var out strings.Builder
	out.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			// Skip until we reach a letter (end of CSI sequence).
			i += 2
			for i < len(s) && !isLetter(s[i]) {
				i++
			}
			continue
		}
		out.WriteByte(s[i])
	}
	return out.String()
}

func isLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func TestPrintToolCall_FinishedOnly(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	// Unfinished tool call should not print.
	p.PrintToolCall("bash", "tc1", `{"command":"ls"}`, false)
	require.Empty(t, buf.String())

	// Finished tool call should print.
	p.PrintToolCall("bash", "tc1", `{"command":"ls -la"}`, true)
	out := stripANSI(buf.String())
	require.Contains(t, out, "●")
	require.Contains(t, out, "BASH")
	require.Contains(t, out, "ls -la")
}

func TestPrintToolCall_Deduplicates(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	p.PrintToolCall("bash", "tc1", `{"command":"ls"}`, true)
	p.PrintToolCall("bash", "tc1", `{"command":"ls"}`, true)
	out := stripANSI(buf.String())
	count := strings.Count(out, "BASH")
	require.Equal(t, 1, count)
}

func TestPrintToolCall_EachTool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		toolName string
		input    string
		want     string
	}{
		{"bash", "bash", `{"command":"echo hi"}`, "echo hi"},
		{"edit", "edit", `{"file_path":"/tmp/main.go"}`, "/tmp/main.go"},
		{"view", "view", `{"file_path":"/src/foo.go"}`, "/src/foo.go"},
		{"glob", "glob", `{"pattern":"*.go"}`, "*.go"},
		{"grep", "grep", `{"pattern":"TODO"}`, "TODO"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			buf := &bytes.Buffer{}
			p := NewEventPrinter(buf)
			p.PrintToolCall(tc.toolName, "id1", tc.input, true)
			out := stripANSI(buf.String())
			require.Contains(t, out, tc.want)
			require.Contains(t, out, strings.ToUpper(tc.toolName))
		})
	}
}

func TestPrintToolCall_Truncates(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	longCmd := string(bytes.Repeat([]byte("a"), 100))
	p.PrintToolCall("bash", "tc1", `{"command":"`+longCmd+`"}`, true)
	out := stripANSI(buf.String())
	require.Contains(t, out, "...")
	// Truncated to 60 chars + "..." = 63
	require.Less(t, len(strings.TrimSpace(out)), 80)
}

func TestPrintToolCall_InvalidJSON(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)
	p.PrintToolCall("bash", "tc1", "not json", true)
	out := stripANSI(buf.String())
	require.Contains(t, out, "not json")
}

func TestPrintToolCall_EmptyInput(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)
	p.PrintToolCall("bash", "tc1", "", true)
	out := stripANSI(buf.String())
	require.Contains(t, out, "BASH")
	require.NotContains(t, out, "=false")
}

func TestPrintToolResult_Success(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	p.PrintToolResult("bash", "3 files found", false)
	out := stripANSI(buf.String())
	require.Contains(t, out, "✓")
	require.Contains(t, out, "BASH")
	require.Contains(t, out, "3 files found")
}

func TestPrintToolResult_SuccessEmpty(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	p.PrintToolResult("view", "", false)
	out := stripANSI(buf.String())
	require.Contains(t, out, "✓")
	require.Contains(t, out, "VIEW")
}

func TestPrintToolResult_Error(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	p.PrintToolResult("bash", "command not found", true)
	out := stripANSI(buf.String())
	require.Contains(t, out, "×")
	require.Contains(t, out, "BASH")
	require.Contains(t, out, "command not found")
}

func TestPrintToolResult_LongContentTruncated(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	long := string(bytes.Repeat([]byte("x"), 200))
	p.PrintToolResult("bash", long, false)
	out := stripANSI(buf.String())
	require.Contains(t, out, "...")
}

func TestPrintAssistantText(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	p.PrintAssistantText("Let me check that file.")
	out := stripANSI(buf.String())
	require.Contains(t, out, "Let me check that file.")
}

func TestPrintAssistantText_FirstLineOnly(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	p.PrintAssistantText("Line one.\nLine two.\nLine three.")
	out := stripANSI(buf.String())
	require.Contains(t, out, "Line one.")
	require.NotContains(t, out, "Line two.")
}

func TestPrintAssistantText_Truncates(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := NewEventPrinter(buf)

	long := string(bytes.Repeat([]byte("a"), 200))
	p.PrintAssistantText(long)
	out := stripANSI(buf.String())
	require.Contains(t, out, "...")
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
