package session

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDerivedTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prompt string
		want   string
	}{
		{
			name:   "terminal controls removed",
			prompt: "Fix\x07 log\x08in\x00 bug",
			want:   "Fix login bug",
		},
		{
			name:   "terminal escapes removed",
			prompt: "\x1b[31mFix\x1b[0m login\x1b]0;untrusted\x07",
			want:   "Fix login",
		},
		{
			name:   "wide characters fit terminal width",
			prompt: strings.Repeat("界", 30),
			want:   strings.Repeat("界", 24) + "…",
		},
		{
			name:   "single line kept as is",
			prompt: "Fix the login bug on staging",
			want:   "Fix the login bug on staging",
		},
		{
			name:   "multiline collapses to one line",
			prompt: "first line\nsecond line\nthird line",
			want:   "first line second line third line",
		},
		{
			name:   "whitespace collapsed",
			prompt: "  fix\t\tthe  bug  ",
			want:   "fix the bug",
		},
		{
			name:   "long prompt truncated with ellipsis",
			prompt: strings.Repeat("x", 80),
			want:   strings.Repeat("x", 49) + "…",
		},
		{
			name:   "exactly at limit not truncated",
			prompt: strings.Repeat("y", 50),
			want:   strings.Repeat("y", 50),
		},
		{
			name:   "empty prompt",
			prompt: "",
			want:   "",
		},
		{
			name:   "whitespace only prompt",
			prompt: "  \n\t ",
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, DerivedTitle(tt.prompt))
		})
	}
}
