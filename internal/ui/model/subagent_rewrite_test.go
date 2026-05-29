package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRewriteSubagentPrompt covers the pure helper rewriteSubagentPrompt which
// detects an `@name rest` prefix pattern and rewrites it into the canonical
// agent-tool dispatch form when name is in the provided active-names set.
func TestRewriteSubagentPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		activeNames map[string]bool
		want        string
	}{
		{
			name:        "no_at_prefix",
			content:     "just a normal message",
			activeNames: map[string]bool{"code-reviewer": true},
			want:        "just a normal message",
		},
		{
			name:        "at_unknown_name",
			content:     "@unknown do something",
			activeNames: map[string]bool{},
			want:        "@unknown do something",
		},
		{
			name:        "at_known_single_word",
			content:     "@code-reviewer review staged",
			activeNames: map[string]bool{"code-reviewer": true},
			want:        `Use the agent tool with subagent_type="code-reviewer" to handle this request: review staged`,
		},
		{
			name:        "at_known_multiword_rest",
			content:     "@tester write tests for the auth module please",
			activeNames: map[string]bool{"tester": true},
			want:        `Use the agent tool with subagent_type="tester" to handle this request: write tests for the auth module please`,
		},
		{
			name:        "at_name_no_space_after",
			content:     "@code-reviewer",
			activeNames: map[string]bool{"code-reviewer": true},
			want:        "@code-reviewer",
		},
		{
			name:        "at_name_only_whitespace_after",
			content:     "@code-reviewer   ",
			activeNames: map[string]bool{"code-reviewer": true},
			want:        "@code-reviewer   ",
		},
		{
			name:        "empty_content",
			content:     "",
			activeNames: map[string]bool{"code-reviewer": true},
			want:        "",
		},
		{
			name:        "at_name_with_leading_space",
			content:     "  @code-reviewer review it",
			activeNames: map[string]bool{"code-reviewer": true},
			want:        "  @code-reviewer review it",
		},
		{
			name:        "nil_active_names_does_not_panic",
			content:     "@code-reviewer review it",
			activeNames: nil,
			want:        "@code-reviewer review it",
		},
		{
			name:        "newline_as_separator_not_supported",
			content:     "@code-reviewer\nreview this",
			activeNames: map[string]bool{"code-reviewer": true},
			want:        "@code-reviewer\nreview this",
		},
		{
			name:        "multiple_at_mentions_only_first_rewritten",
			content:     "@code-reviewer review and @tester test",
			activeNames: map[string]bool{"code-reviewer": true, "tester": true},
			want:        `Use the agent tool with subagent_type="code-reviewer" to handle this request: review and @tester test`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := rewriteSubagentPrompt(tt.content, tt.activeNames)
			require.Equal(t, tt.want, got)
		})
	}
}
