package agent

import (
	"testing"

	"charm.land/fantasy"
)

// Regression test for a real reported bug (charmbracelet/crush#3284): a
// thinking-capable local model (e.g. a hand-configured llama.cpp server)
// whose config never declared CatwalkCfg.CanReason still reasons by its
// own chat template default, burns the tiny 40-token title budget on
// hidden reasoning, and every attempt falls through to "Untitled Session".
func TestShouldRetryTitleWithLargerBudget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		canReason     bool
		finishReason  fantasy.FinishReason
		reasoningText string
		want          bool
	}{
		{
			name:          "unconfigured reasoning model hit limit while thinking",
			canReason:     false,
			finishReason:  fantasy.FinishReasonLength,
			reasoningText: "let me think about a good title...",
			want:          true,
		},
		{
			name:          "already used the bumped budget (CanReason true) -- no further retry",
			canReason:     true,
			finishReason:  fantasy.FinishReasonLength,
			reasoningText: "let me think about a good title...",
			want:          false,
		},
		{
			name:          "hit limit but no reasoning content -- not a thinking-budget problem",
			canReason:     false,
			finishReason:  fantasy.FinishReasonLength,
			reasoningText: "",
			want:          false,
		},
		{
			name:          "finished normally -- no retry needed",
			canReason:     false,
			finishReason:  fantasy.FinishReasonStop,
			reasoningText: "",
			want:          false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := shouldRetryTitleWithLargerBudget(tc.canReason, tc.finishReason, tc.reasoningText)
			if got != tc.want {
				t.Errorf("shouldRetryTitleWithLargerBudget(%v, %v, %q) = %v, want %v",
					tc.canReason, tc.finishReason, tc.reasoningText, got, tc.want)
			}
		})
	}
}
