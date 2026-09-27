package router

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildQuestions(t *testing.T) {
	t.Parallel()

	questions := BuildQuestions(nil)

	require.Len(t, questions, 1)
	require.Contains(t, questions, "reasoning_effort")
	require.Equal(t, "score", questions["reasoning_effort"].Type)
	require.Equal(t, "How much reasoning effort does this request need?", questions["reasoning_effort"].Instructions)
	levels, ok := questions["reasoning_effort"].Criteria.([]string)
	require.True(t, ok)
	require.Equal(t, []string{"low", "medium", "high"}, levels)
}

func TestBuildQuestions_IncludesModelChoiceWhenPoolNonEmpty(t *testing.T) {
	t.Parallel()

	questions := BuildQuestions([]string{"anthropic/claude-opus-4", "anthropic/claude-haiku-4"})
	require.Contains(t, questions, "reasoning_effort")
	require.Contains(t, questions, "model_choice")
	require.Equal(t, "choice", questions["model_choice"].Type)
	criteria, ok := questions["model_choice"].Criteria.(map[string]string)
	require.True(t, ok)
	require.Contains(t, criteria, "anthropic/claude-opus-4")
	require.Contains(t, criteria, "anthropic/claude-haiku-4")
	// Each model description should be more informative than just the ID.
	opusDesc := criteria["anthropic/claude-opus-4"]
	require.NotEqual(t, "anthropic/claude-opus-4", opusDesc)
	require.Contains(t, opusDesc, "Flagship")
	haikuDesc := criteria["anthropic/claude-haiku-4"]
	require.NotEqual(t, "anthropic/claude-haiku-4", haikuDesc)
	require.Contains(t, haikuDesc, "Fast")
}

func TestBuildQuestions_OmitsModelChoiceWhenPoolEmpty(t *testing.T) {
	t.Parallel()

	questions := BuildQuestions(nil)
	require.Contains(t, questions, "reasoning_effort")
	require.NotContains(t, questions, "model_choice")
}

func TestMapDecision_IncludesModelIDWhenAnswered(t *testing.T) {
	t.Parallel()

	answers := map[string]Answer{
		"reasoning_effort": {Type: "score", Score: 2, Confidence: 0.9},
		"model_choice":     {Type: "choice", Choice: "anthropic/claude-opus-4", Confidence: 0.85},
	}
	decision, err := MapDecision(answers, 0.7)
	require.NoError(t, err)
	require.Equal(t, "high", decision.ReasoningEffort)
	require.Equal(t, "anthropic/claude-opus-4", decision.ModelID)
}

func TestMapDecision_EmptyModelIDWhenNotAnswered(t *testing.T) {
	t.Parallel()

	answers := map[string]Answer{
		"reasoning_effort": {Type: "score", Score: 0, Confidence: 0.9},
	}
	decision, err := MapDecision(answers, 0.7)
	require.NoError(t, err)
	require.Empty(t, decision.ModelID)
}

func TestMapDecision_Success(t *testing.T) {
	t.Parallel()

	answers := map[string]Answer{
		"reasoning_effort": {Type: "score", Score: 1.4, Confidence: 0.92},
	}
	decision, err := MapDecision(answers, 0.7)
	require.NoError(t, err)
	require.Equal(t, "medium", decision.ReasoningEffort)
	require.InDelta(t, 0.92, decision.Confidence, 0.0001)
	require.False(t, decision.LowConfidence)
}

func TestMapDecision_LowConfidenceStillApplied(t *testing.T) {
	t.Parallel()

	answers := map[string]Answer{
		"reasoning_effort": {Type: "score", Score: 0, Confidence: 0.4},
	}
	decision, err := MapDecision(answers, 0.7)
	require.NoError(t, err)
	require.Equal(t, "low", decision.ReasoningEffort)
	require.InDelta(t, 0.4, decision.Confidence, 0.0001)
	require.True(t, decision.LowConfidence)
}

func TestMapDecision_ClampsOutOfRangeScore(t *testing.T) {
	t.Parallel()

	answers := map[string]Answer{
		"reasoning_effort": {Type: "score", Score: 99, Confidence: 1},
	}
	decision, err := MapDecision(answers, 0.7)
	require.NoError(t, err)
	require.Equal(t, "high", decision.ReasoningEffort)

	answers["reasoning_effort"] = Answer{Type: "score", Score: -5, Confidence: 1}
	decision, err = MapDecision(answers, 0.7)
	require.NoError(t, err)
	require.Equal(t, "low", decision.ReasoningEffort)
}

func TestMapDecision_MissingAnswer(t *testing.T) {
	t.Parallel()

	_, err := MapDecision(map[string]Answer{}, 0.7)
	require.Error(t, err)

	_, err = MapDecision(map[string]Answer{
		"reasoning_effort": {Type: "choice", Choice: "high", Confidence: 1},
	}, 0.7)
	require.Error(t, err)
}

func TestDescribeModelForRouter_KnownModels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		modelID string
		wants   string
	}{
		// Exact matches.
		{"anthropic/claude-opus", "Flagship Claude model"},
		{"anthropic/claude-sonnet", "Balanced Claude model"},
		{"anthropic/claude-haiku", "Fast Claude model"},
		{"openai/gpt-4o", "GPT model for general reasoning"},
		{"openai/gpt-4o-mini", "Compact GPT model"},
		{"google/gemini-2.0-flash", "Fast Gemini model"},
		{"google/gemini-2.5-pro", "Advanced Gemini model"},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			desc := describeModelForRouter(tt.modelID)
			require.NotEqual(t, tt.modelID, desc, "description should not be the model ID")
			require.Contains(t, desc, tt.wants)
		})
	}
}

func TestDescribeModelForRouter_PrefixMatching(t *testing.T) {
	t.Parallel()

	// Prefix matching: "anthropic/claude-opus-4.5" matches "anthropic/claude-opus".
	desc := describeModelForRouter("anthropic/claude-opus-4.5")
	require.Contains(t, desc, "Flagship")
	require.NotEqual(t, "anthropic/claude-opus-4.5", desc)

	// Another prefix match: "openai/gpt-4o-2024-11-20" matches "openai/gpt-4o".
	desc = describeModelForRouter("openai/gpt-4o-2024-11-20")
	require.Contains(t, desc, "general reasoning")
}

func TestDescribeModelForRouter_LongestPrefixWins(t *testing.T) {
	t.Parallel()

	// "openai/gpt-4o-mini" must match the more specific "openai/gpt-4o-mini"
	// entry, not the shorter "openai/gpt-4o" prefix — Go map iteration order
	// is random, so this pins the longest-prefix-wins tie-break deterministically.
	desc := describeModelForRouter("openai/gpt-4o-mini-2024-07-18")
	require.Contains(t, desc, "Compact GPT model")
	require.NotContains(t, desc, "general reasoning")
}

func TestDescribeModelForRouter_UnknownModelFailsOpen(t *testing.T) {
	t.Parallel()

	// Unknown model returns itself (fail-open).
	unknown := "example-provider/unknown-model-v1"
	desc := describeModelForRouter(unknown)
	require.Equal(t, unknown, desc)
}
