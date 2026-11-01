package agent

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestAnalysisRouter(t *testing.T) {
	t.Parallel()

	t.Run("plain text without tags", func(t *testing.T) {
		t.Parallel()
		var content strings.Builder
		var reasoning strings.Builder
		r := newAnalysisRouter(
			func(s string) error { reasoning.WriteString(s); return nil },
			func(s string) error { content.WriteString(s); return nil },
		)
		require.NoError(t, r.write("hello world"))
		require.NoError(t, r.flush())
		require.Equal(t, "hello world", content.String())
		require.Empty(t, reasoning.String())
	})

	t.Run("full analysis block in single delta", func(t *testing.T) {
		t.Parallel()
		var content strings.Builder
		var reasoning strings.Builder
		r := newAnalysisRouter(
			func(s string) error { reasoning.WriteString(s); return nil },
			func(s string) error { content.WriteString(s); return nil },
		)
		require.NoError(t, r.write("before<analysis>thinking</analysis>after"))
		require.NoError(t, r.flush())
		require.Equal(t, "beforeafter", content.String())
		require.Equal(t, "thinking\n", reasoning.String())
	})

	t.Run("analysis tag split across deltas", func(t *testing.T) {
		t.Parallel()
		var content strings.Builder
		var reasoning strings.Builder
		r := newAnalysisRouter(
			func(s string) error { reasoning.WriteString(s); return nil },
			func(s string) error { content.WriteString(s); return nil },
		)
		// Split <analysis> across two deltas.
		require.NoError(t, r.write("before<an"))
		require.NoError(t, r.write("alysis>thinking</analysis>after"))
		require.NoError(t, r.flush())
		require.Equal(t, "beforeafter", content.String())
		require.Equal(t, "thinking\n", reasoning.String())
	})

	t.Run("closing tag split across deltas", func(t *testing.T) {
		t.Parallel()
		var content strings.Builder
		var reasoning strings.Builder
		r := newAnalysisRouter(
			func(s string) error { reasoning.WriteString(s); return nil },
			func(s string) error { content.WriteString(s); return nil },
		)
		require.NoError(t, r.write("<analysis>think"))
		require.NoError(t, r.write("ing</anal"))
		require.NoError(t, r.write("ysis>after"))
		require.NoError(t, r.flush())
		require.Equal(t, "after", content.String())
		require.Equal(t, "thinking\n", reasoning.String())
	})

	t.Run("flush while in analysis", func(t *testing.T) {
		t.Parallel()
		var content strings.Builder
		var reasoning strings.Builder
		r := newAnalysisRouter(
			func(s string) error { reasoning.WriteString(s); return nil },
			func(s string) error { content.WriteString(s); return nil },
		)
		require.NoError(t, r.write("<analysis>unclosed thinking"))
		require.NoError(t, r.flush())
		require.Empty(t, content.String())
		require.Equal(t, "unclosed thinking", reasoning.String())
	})

	t.Run("flush with empty buffer", func(t *testing.T) {
		t.Parallel()
		r := newAnalysisRouter(
			func(s string) error { return nil },
			func(s string) error { return nil },
		)
		require.NoError(t, r.flush())
	})

	t.Run("error propagation from onReasoning", func(t *testing.T) {
		t.Parallel()
		reasoningErr := errors.New("reasoning write failed")
		r := newAnalysisRouter(
			func(s string) error { return reasoningErr },
			func(s string) error { return nil },
		)
		err := r.write("<analysis>thinking</analysis>")
		require.ErrorIs(t, err, reasoningErr)
		// flush should also return the last error.
		require.ErrorIs(t, r.flush(), reasoningErr)
	})

	t.Run("error propagation from onContent", func(t *testing.T) {
		t.Parallel()
		contentErr := errors.New("content write failed")
		r := newAnalysisRouter(
			func(s string) error { return nil },
			func(s string) error { return contentErr },
		)
		// Use text longer than the tag length so it's emitted immediately.
		err := r.write("plain text that is long enough")
		require.ErrorIs(t, err, contentErr)
	})

	t.Run("multiple analysis blocks", func(t *testing.T) {
		t.Parallel()
		var content strings.Builder
		var reasoning strings.Builder
		r := newAnalysisRouter(
			func(s string) error { reasoning.WriteString(s); return nil },
			func(s string) error { content.WriteString(s); return nil },
		)
		require.NoError(t, r.write("a<analysis>t1</analysis>b<analysis>t2</analysis>c"))
		require.NoError(t, r.flush())
		require.Equal(t, "abc", content.String())
		require.Equal(t, "t1\nt2\n", reasoning.String())
	})

	t.Run("partial open tag buffered then completed", func(t *testing.T) {
		t.Parallel()
		var content strings.Builder
		var reasoning strings.Builder
		r := newAnalysisRouter(
			func(s string) error { reasoning.WriteString(s); return nil },
			func(s string) error { content.WriteString(s); return nil },
		)
		// Send just "<anal" — should be buffered, nothing emitted.
		require.NoError(t, r.write("<anal"))
		require.Empty(t, content.String())
		require.Empty(t, reasoning.String())
		// Complete the tag.
		require.NoError(t, r.write("ysis>stuff</analysis>"))
		require.NoError(t, r.flush())
		require.Empty(t, content.String())
		require.Equal(t, "stuff\n", reasoning.String())
	})

	t.Run("partial tag that turns out not to be a tag", func(t *testing.T) {
		t.Parallel()
		var content strings.Builder
		var reasoning strings.Builder
		r := newAnalysisRouter(
			func(s string) error { reasoning.WriteString(s); return nil },
			func(s string) error { content.WriteString(s); return nil },
		)
		// "<anal" is buffered, then "ogy>" makes it "<analogy>" which is not <analysis>.
		require.NoError(t, r.write("<anal"))
		require.NoError(t, r.write("ogy>"))
		require.NoError(t, r.flush())
		require.Equal(t, "<analogy>", content.String())
		require.Empty(t, reasoning.String())
	})
}

func TestBuildSummaryPrompt(t *testing.T) {
	t.Parallel()

	t.Run("with thinking enabled", func(t *testing.T) {
		t.Parallel()
		prompt := buildSummaryPrompt("", nil, true)
		require.Contains(t, prompt, "reasoning/thinking block")
		require.NotContains(t, prompt, "<analysis>")
	})

	t.Run("with thinking disabled", func(t *testing.T) {
		t.Parallel()
		prompt := buildSummaryPrompt("", nil, false)
		require.Contains(t, prompt, "<analysis>")
		require.NotContains(t, prompt, "reasoning/thinking block")
	})

	t.Run("includes todos", func(t *testing.T) {
		t.Parallel()
		require.NotContains(t, buildSummaryPrompt("", nil, false), "Current Todo List")
		todos := []session.Todo{{Content: "fix login", Status: session.TodoStatusInProgress}}
		require.Contains(t, buildSummaryPrompt("", todos, false), "fix login")
	})

	t.Run("summarizes an extraction when given one", func(t *testing.T) {
		t.Parallel()
		prompt := buildSummaryPrompt("## Goals\n- fix login", nil, true)
		require.Contains(t, prompt, "## Extracted Context")
		require.Contains(t, prompt, "- fix login")
		require.NotContains(t, prompt, "conversation above")
	})
}

// The summary model reads the extraction instead of the conversation when
// algorithmic compaction is on, and the conversation otherwise.
func TestSummaryInput(t *testing.T) {
	env := testEnv(t)
	a := testSessionAgent(env, fastModel{}, fastModel{}, "system").(*sessionAgent)
	msgs := []message.Message{
		{ID: "u1", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "fix the login bug in auth.go"}}},
		{ID: "a1", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "On it."}, message.Finish{Reason: message.FinishReasonEndTurn}}},
	}

	t.Setenv("CRUSH_ALGORITHMIC_COMPACT", "")
	prompt, history := a.summaryInput(session.Session{}, msgs, a.models().Large, false)
	require.Contains(t, prompt, "conversation above")
	require.NotEmpty(t, history)

	t.Setenv("CRUSH_ALGORITHMIC_COMPACT", "1")
	prompt, history = a.summaryInput(session.Session{}, msgs, a.models().Large, false)
	require.Contains(t, prompt, "## Extracted Context")
	require.Empty(t, history, "the conversation itself is not sent")
}
