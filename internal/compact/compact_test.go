package compact

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func user(text string) message.Message {
	return message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: text}}}
}

func assistant(text string, calls ...message.ToolCall) message.Message {
	parts := []message.ContentPart{message.TextContent{Text: text}}
	for _, c := range calls {
		parts = append(parts, c)
	}
	return message.Message{Role: message.Assistant, Parts: append(parts, message.Finish{Reason: message.FinishReasonToolUse})}
}

func results(rs ...message.ToolResult) message.Message {
	parts := make([]message.ContentPart, len(rs))
	for i, r := range rs {
		parts[i] = r
	}
	return message.Message{Role: message.Tool, Parts: parts}
}

func call(id, name, input string) message.ToolCall {
	return message.ToolCall{ID: id, Name: name, Input: input, Finished: true}
}

func result(id, name, content string, isError bool) message.ToolResult {
	return message.ToolResult{ToolCallID: id, Name: name, Content: content, IsError: isError}
}

func summary(text string) message.Message {
	return message.Message{Role: message.User, IsSummaryMessage: true, Parts: []message.ContentPart{message.TextContent{Text: text}}}
}

func bash(id, command string) message.ToolCall {
	return call(id, "bash", fmt.Sprintf(`{"command":%q,"description":"run"}`, command))
}

func TestCompactEmpty(t *testing.T) {
	t.Parallel()

	require.Empty(t, Compact(nil))
	require.Empty(t, Compact([]message.Message{
		assistant("Hello."),
		{Role: message.User, Parts: []message.ContentPart{message.ShellCommand{Command: "ls", Output: "a\nb"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "continue", Hidden: true}}},
	}))
}

func TestCompactPreviousSummaryVerbatim(t *testing.T) {
	t.Parallel()

	prev := "### Goal\nShip the login fix.\n\n### Completed\n1. Patched `internal/auth/login.go`."
	out := Compact([]message.Message{summary(prev)})
	require.Equal(t, "[Previous summary]\n"+prev, out)

	out = Compact([]message.Message{summary(prev), user("now add tests for the login fix")})
	require.True(t, strings.HasPrefix(out, "[Previous summary]\n"+prev+"\n\n[Session goal]\n- now add tests"), out)
}

func TestCompactGoals(t *testing.T) {
	t.Parallel()

	first := "Rebuild the compaction package so it extracts a deterministic brief from Crush sessions. " +
		"It must pair tool calls with results by ID, cap every section, keep output under a byte budget, " +
		"and pass the previous summary through verbatim. Keep the diff focused."
	require.Greater(t, len(first), 200)

	out := Compact([]message.Message{
		user(first),
		assistant("Starting."),
		user("ok"),
		user("actually, switch to fixing the flaky TestSummaryInput first"),
		assistant("Switching."),
		user("yes"),
	})
	require.Contains(t, out, "[Session goal]\n- "+first+"\n")
	require.Contains(t, out, "[Latest request]\n- actually, switch to fixing the flaky TestSummaryInput first\n")
	require.Contains(t, out, "- User: yes")
	require.NotContains(t, out, "[Latest request]\n- yes")
}

func TestCompactParallelToolCalls(t *testing.T) {
	t.Parallel()

	view := "<file>\n     1|package auth\n     2|\n     3|func Login() error {\n</file>\n"
	testOut := "--- FAIL: TestLogin (0.00s)\n    login_test.go:12: want nil, got err\nFAIL\nFAIL\tgithub.com/x/auth\t0.01s\nExit code 1\n\n<cwd>/repo</cwd>"
	out := Compact([]message.Message{
		user("fix the login bug"),
		assistant("Looking.",
			call("c1", "view", `{"file_path":"/repo/auth/login.go"}`),
			bash("c2", "go test ./auth/..."),
			call("c3", "edit", `{"file_path":"/repo/auth/session.go","old_string":"a","new_string":"b"}`),
			call("c4", "edit", `{"file_path":"/repo/auth/token.go","old_string":"x","new_string":"y"}`),
		),
		// Results arrive in a different order than the calls.
		results(
			result("c4", "edit", "old_string not found in file. Make sure it matches exactly", true),
			result("c3", "edit", "<result>\nContent replaced in file: /repo/auth/session.go\n</result>", false),
			result("c2", "bash", testOut, false),
			result("c1", "view", view, false),
		),
	})
	require.Contains(t, out, "[Files modified]\n- /repo/auth/session.go\n")
	require.Contains(t, out, "[Files read]\n- /repo/auth/login.go\n")
	require.Contains(t, out, "- bash `go test ./auth/...`: --- FAIL: TestLogin (0.00s)\n")
	require.Contains(t, out, "- edit `/repo/auth/token.go`: old_string not found in file.")
	require.NotContains(t, out, "token.go\n")
	require.NotContains(t, out, "session.go`")
}

func TestCompactBashFailures(t *testing.T) {
	t.Parallel()

	out := Compact([]message.Message{
		user("build it"),
		assistant("",
			bash("b1", "go build ./..."),
			bash("b2", "go test ./..."),
			bash("b3", "cat build.log"),
			bash("b4", "go vet ./..."),
			bash("b5", "make lint"),
		),
		results(
			result("b1", "bash", "# github.com/x/y\n./y.go:3:2: undefined: Foo\nExit code 1\n\n<cwd>/repo</cwd>", false),
			result("b2", "bash", "ok  \tgithub.com/x/y\t0.2s\n10 passed, 0 failed\n\n<cwd>/repo</cwd>", false),
			result("b3", "bash", "Exit code 1\nretrying\n\n<cwd>/repo</cwd>", false),
			result("b4", "bash", "[Job 3] Error executing command: signal: killed", true),
			result("b5", "bash", "\nExit code 2\n\n<cwd>/repo</cwd>", false),
		),
		assistant("", bash("b6", "go build ./...")),
		results(result("b6", "bash", "no output", false)),
	})
	require.Contains(t, out, "- bash `go build ./...`: ./y.go:3:2: undefined: Foo (later succeeded)\n")
	require.Contains(t, out, "- bash `go vet ./...`: [Job 3] Error executing command: signal: killed\n")
	require.Contains(t, out, "- bash `make lint`: Exit code 2\n")
	require.NotContains(t, out, "go test")
	require.NotContains(t, out, "cat build.log")
}

func TestCompactEmptySearches(t *testing.T) {
	t.Parallel()

	out := Compact([]message.Message{
		user("where is the retry logic?"),
		assistant("",
			call("g1", "grep", `{"pattern":"retryPolicy","path":"/repo/internal"}`),
			call("g2", "glob", `{"pattern":"**/*retry*.go"}`),
			call("g3", "grep", `{"pattern":"Backoff"}`),
		),
		results(
			result("g1", "grep", "No files found", false),
			result("g2", "glob", "No files found", false),
			result("g3", "grep", "Found 1 matches\n/repo/backoff.go:\n  Line 3, Char 6: type Backoff struct", false),
		),
	})
	require.Contains(t, out, "[Searches with no results]\n- grep `retryPolicy` in /repo/internal\n- glob `**/*retry*.go`")
	require.NotContains(t, out, "Backoff")
	require.NotContains(t, out, "[Failed tool calls]")
}

func TestCompactSkipsHiddenAndReasoning(t *testing.T) {
	t.Parallel()

	out := Compact([]message.Message{
		user("fix the parser"),
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.ReasoningContent{Thinking: "secret reasoning"},
			message.TextContent{Text: "Fixed."},
		}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hidden continuation", Hidden: true}}},
	})
	require.Contains(t, out, "- Assistant: Fixed.")
	require.NotContains(t, out, "secret reasoning")
	require.NotContains(t, out, "hidden continuation")
}

func TestClipMultibyte(t *testing.T) {
	t.Parallel()

	s := strings.Repeat("日本語🙂é", 50)
	for n := 1; n < len(s)+2; n++ {
		got := clip(s, n)
		require.True(t, utf8.ValidString(got), "n=%d", n)
		require.LessOrEqual(t, len(got), max(n, len("…")), "n=%d", n)
	}

	out := Compact([]message.Message{user(strings.Repeat("修正してください🙂", 400))})
	require.True(t, utf8.ValidString(out))
	require.Contains(t, out, "…")
}

func TestCompactDeterministicAndBounded(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{user("long running session " + strings.Repeat("with a detailed spec ", 200))}
	for i := range 1250 {
		v, b, e, g := fmt.Sprintf("v%d", i), fmt.Sprintf("b%d", i), fmt.Sprintf("e%d", i), fmt.Sprintf("g%d", i)
		msgs = append(msgs,
			assistant(strings.Repeat(fmt.Sprintf("step %d ", i), 100),
				call(v, "view", fmt.Sprintf(`{"file_path":"/repo/pkg%d/file.go"}`, i%300)),
				bash(b, fmt.Sprintf("git commit -am 'change %d'", i)),
				call(e, "edit", fmt.Sprintf(`{"file_path":"/repo/pkg%d/edit.go"}`, i%200)),
				call(g, "grep", fmt.Sprintf(`{"pattern":"sym%d"}`, i)),
			),
			results(
				result(g, "grep", "No files found", false),
				result(e, "edit", "failed: "+strings.Repeat("x", 500), i%3 == 0),
				result(b, "bash", fmt.Sprintf("[main %07x] change %d\n", i+0xa000000, i), false),
				result(v, "view", "<file>\n     1|package x\n</file>", false),
			),
			assistant(fmt.Sprintf("Done with step %d.", i)),
			user(fmt.Sprintf("next step %d please ", i)+strings.Repeat("detail ", 100)),
		)
	}
	require.Equal(t, 5001, len(msgs))

	withSummary := append([]message.Message{summary(strings.Repeat("### Goal\nprevious ", 1000))}, msgs...)
	out := Compact(withSummary)
	require.LessOrEqual(t, len(out), maxBytes)
	require.True(t, utf8.ValidString(out))
	require.Contains(t, out, "[Previous summary]")
	require.Contains(t, out, "[Session goal]\n- long running session")
	require.Contains(t, out, "[Latest request]\n- next step 1249 please")
	require.Contains(t, out, "more)")
	require.Equal(t, out, Compact(withSummary), "output must be deterministic")

	out = Compact(msgs)
	require.LessOrEqual(t, len(out), maxBytes)
	require.Contains(t, out, "[Recent transcript]")
}
