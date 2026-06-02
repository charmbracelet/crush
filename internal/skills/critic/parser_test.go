package critic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFeedback_DirectJSON(t *testing.T) {
	t.Parallel()

	raw := `{"verdict":"approve","confidence":0.95,"concerns":[],"summary":"Looks good."}`
	fb, err := ParseFeedback(raw)
	require.NoError(t, err)
	require.Equal(t, "approve", fb.Verdict)
	require.InDelta(t, 0.95, fb.Confidence, 0.001)
	require.Empty(t, fb.Concerns)
}

func TestParseFeedback_MarkdownFence(t *testing.T) {
	t.Parallel()

	raw := "```json\n{\"verdict\":\"revise\",\"confidence\":0.72,\"concerns\":[{\"severity\":\"major\",\"dimension\":\"correctness\",\"summary\":\"Bug\",\"suggestion\":\"Fix it\"}],\"summary\":\"Needs work.\"}\n```"
	fb, err := ParseFeedback(raw)
	require.NoError(t, err)
	require.Equal(t, "revise", fb.Verdict)
	require.Len(t, fb.Concerns, 1)
	require.Equal(t, "major", fb.Concerns[0].Severity)
}

func TestParseFeedback_TrailingCommaRepair(t *testing.T) {
	t.Parallel()

	raw := `{"verdict":"halt","confidence":1.0,"concerns":[{"severity":"critical","dimension":"safety","summary":"SQL injection","suggestion":"Use params",},],"summary":"Dangerous."}`
	fb, err := ParseFeedback(raw)
	require.NoError(t, err)
	require.Equal(t, "halt", fb.Verdict)
	require.Len(t, fb.Concerns, 1)
}

func TestParseFeedback_Empty(t *testing.T) {
	t.Parallel()

	_, err := ParseFeedback("")
	require.Error(t, err)
}

func TestParseFeedback_Invalid(t *testing.T) {
	t.Parallel()

	_, err := ParseFeedback("this is not json at all")
	require.Error(t, err)
}

func TestParseFeedback_LargeInputTruncatedInError(t *testing.T) {
	t.Parallel()

	large := make([]byte, 2000)
	for i := range large {
		large[i] = 'x'
	}
	_, err := ParseFeedback(string(large))
	require.Error(t, err)
	require.Contains(t, err.Error(), "... [truncated]")
	require.Less(t, len(err.Error()), 800)
}

func TestParseFeedback_NestedFence(t *testing.T) {
	t.Parallel()

	// The outer fence should be captured, inner content ignored by JSON parser.
	raw := "Some text before\n```json\n{\"verdict\":\"approve\",\"confidence\":0.99,\"concerns\":[],\"summary\":\"OK\"}\n```\nSome text after"
	fb, err := ParseFeedback(raw)
	require.NoError(t, err)
	require.Equal(t, "approve", fb.Verdict)
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	require.Equal(t, "hello", truncate("hello", 10))
	long := make([]byte, 600)
	for i := range long {
		long[i] = 'a'
	}
	truncated := truncate(string(long), 500)
	require.Len(t, truncated, 515) // 500 + len("... [truncated]")
	require.Contains(t, truncated, "... [truncated]")
}
