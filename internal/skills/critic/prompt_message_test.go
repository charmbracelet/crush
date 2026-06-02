package critic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildCriticPrompt_MessageCheckpoint(t *testing.T) {
	t.Parallel()

	cp := Checkpoint{
		Type:           CheckpointMessage,
		UserPrompt:     "Explain Go interfaces",
		MessageContent: "Go interfaces are implicit contracts...",
		Iteration:      0,
	}

	prompt, err := BuildCriticPrompt(cp, "")
	require.NoError(t, err)

	// Should include response review dimensions.
	require.Contains(t, prompt, "Response Review Dimensions")
	require.Contains(t, prompt, "Accuracy")
	require.Contains(t, prompt, "Clarity")
	require.Contains(t, prompt, "Completeness")
	require.Contains(t, prompt, "Actionability")
	require.Contains(t, prompt, "Tone")

	// Should include message delimiters.
	require.Contains(t, prompt, "<<<MESSAGE_BEGIN>>>")
	require.Contains(t, prompt, "<<<MESSAGE_END>>>")
	require.Contains(t, prompt, "Go interfaces are implicit contracts...")

	// Should include message-specific output format.
	require.Contains(t, prompt, "\"dimension\": \"accuracy | clarity | completeness | safety | actionability | tone\"")

	// Should NOT include code review dimensions.
	require.NotContains(t, prompt, "Code Review Dimensions")
	require.NotContains(t, prompt, "<<<DIFF_BEGIN>>>")
}

func TestBuildCriticPrompt_EditCheckpoint(t *testing.T) {
	t.Parallel()

	cp := Checkpoint{
		Type:        CheckpointEdit,
		UserPrompt:  "Fix the bug",
		PrimaryDiff: "--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-foo\n+bar\n",
		Iteration:   0,
	}

	prompt, err := BuildCriticPrompt(cp, "")
	require.NoError(t, err)

	// Should include code review dimensions.
	require.Contains(t, prompt, "Code Review Dimensions")
	require.Contains(t, prompt, "Correctness")
	require.Contains(t, prompt, "Idiomatics")

	// Should include diff delimiters.
	require.Contains(t, prompt, "<<<DIFF_BEGIN>>>")
	require.Contains(t, prompt, "<<<DIFF_END>>>")

	// Should include code-specific output format.
	require.Contains(t, prompt, "\"dimension\": \"correctness | safety | idiomatics | efficiency | testing | minimalism\"")

	// Should NOT include response review dimensions.
	require.NotContains(t, prompt, "Response Review Dimensions")
	require.NotContains(t, prompt, "<<<MESSAGE_BEGIN>>>")
}

func TestBuildCriticPrompt_EmptyCheckpoint(t *testing.T) {
	t.Parallel()

	// A checkpoint with neither diff nor message content should still produce a valid prompt.
	cp := Checkpoint{
		Type:       CheckpointMessage,
		UserPrompt: "Hello",
		Iteration:  0,
	}

	prompt, err := BuildCriticPrompt(cp, "")
	require.NoError(t, err)
	require.NotEmpty(t, prompt)
	// Should not include any dimensions since there's nothing to review.
	require.False(t, strings.Contains(prompt, "Code Review Dimensions") && strings.Contains(prompt, "Response Review Dimensions"))
}
