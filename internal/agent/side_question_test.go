package agent

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// stubLanguageModel is a non-nil fantasy.LanguageModel marker for unit
// tests that only need to distinguish present vs absent models.
type stubLanguageModel struct {
	fantasy.LanguageModel
}

func TestSideQuestionAttemptsOrder(t *testing.T) {
	t.Parallel()

	small := Model{
		Model: stubLanguageModel{},
		CatwalkCfg: catwalk.Model{
			ContextWindow: 1000,
		},
	}
	large := Model{
		Model: stubLanguageModel{},
		CatwalkCfg: catwalk.Model{
			ContextWindow: 100_000,
		},
	}

	t.Run("fits small uses small then large", func(t *testing.T) {
		t.Parallel()
		got := sideQuestionAttempts(small, large, 100, 100)
		require.Len(t, got, 2)
		require.Equal(t, small, got[0])
		require.Equal(t, large, got[1])
	})

	t.Run("over 80 percent uses large only", func(t *testing.T) {
		t.Parallel()
		got := sideQuestionAttempts(small, large, 700, 200)
		require.Len(t, got, 1)
		require.Equal(t, large, got[0])
	})

	t.Run("exactly 80 percent still prefers small", func(t *testing.T) {
		t.Parallel()
		got := sideQuestionAttempts(small, large, 700, 100)
		require.Len(t, got, 2)
		require.Equal(t, small, got[0])
	})

	t.Run("nil small falls back to large", func(t *testing.T) {
		t.Parallel()
		got := sideQuestionAttempts(Model{}, large, 0, 0)
		require.Len(t, got, 1)
		require.Equal(t, large, got[0])
	})

	t.Run("both nil returns empty", func(t *testing.T) {
		t.Parallel()
		got := sideQuestionAttempts(Model{}, Model{}, 0, 0)
		require.Empty(t, got)
	})
}

func TestSanitizeSideQuestionAnswer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain answer unchanged",
			input:    "This model conversion is finished.",
			expected: "This model conversion is finished.",
		},
		{
			name:     "strips DSML tool call tags",
			input:    "<|DSML|tool_calls><||DSML||invoke name=\"bash\"><||DSML||parameter name=\"command\" string=\"true\">ls -la</||DSML||parameter></||DSML||invoke></|DSML|tool_calls>Check status.",
			expected: "Check status.",
		},
		{
			name:     "strips orphan DSML tags",
			input:    "<||DSML||invoke name=\"bash\">Some content",
			expected: "Some content",
		},
		{
			name:     "strips think tags",
			input:    "<think>Thinking process...</think>Here is the answer.",
			expected: "Here is the answer.",
		},
		{
			name:     "strips tool_call tags",
			input:    "<tool_call>ls -la</tool_call>Finished",
			expected: "Finished",
		},
		{
			name:     "strips orphan intro before tool call",
			input:    "Let me check actual progress:\n<|DSML|tool_calls><||DSML||invoke name=\"bash\"><||DSML||parameter name=\"command\" string=\"true\">ls</||DSML||parameter></||DSML||invoke></|DSML|tool_calls>",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeSideQuestionAnswer(tt.input)
			require.Equal(t, tt.expected, got)
		})
	}
}
