package shell

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpandValue_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		env   []string
		want  string
	}{
		{
			name:  "plain string round trip",
			value: "hello world",
			want:  "hello world",
		},
		{
			name:  "plain var from env",
			value: "$FOO",
			env:   []string{"FOO=bar"},
			want:  "bar",
		},
		{
			name:  "braced var from env",
			value: "pre-${FOO}-post",
			env:   []string{"FOO=bar"},
			want:  "pre-bar-post",
		},
		{
			name:  "default syntax on unset",
			value: "${MISSING:-fallback}",
			want:  "fallback",
		},
		{
			name:  "default syntax on set preserves value",
			value: "${SET:-fallback}",
			env:   []string{"SET=used"},
			want:  "used",
		},
		{
			name:  "command substitution",
			value: "$(echo hi)",
			want:  "hi",
		},
		{
			name:  "command substitution preserves internal spaces",
			value: `$(echo "a b")`,
			want:  "a b",
		},
		{
			name:  "command substitution strips only trailing newline",
			value: "$(printf 'a\\nb\\n')",
			want:  "a\nb",
		},
		{
			name:  "literal spaces around cmdsubst are preserved",
			value: "  $(echo v)  ",
			want:  "  v  ",
		},
		{
			name:  "paren inside quoted arg to echo",
			value: `$(echo ")")`,
			want:  ")",
		},
		{
			name:  "nested command substitution",
			value: "$(echo $(echo hi))",
			want:  "hi",
		},
		{
			name:  "glob-like input round trips unchanged",
			value: "*.go",
			want:  "*.go",
		},
		{
			name:  "tilde round trips unchanged",
			value: "~",
			want:  "~",
		},
		{
			name:  "env var inside cmdsubst",
			value: `$(printf '%s' "$FOO")`,
			env:   []string{"FOO=bar"},
			want:  "bar",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ExpandValue(t.Context(), tc.value, tc.env)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestExpandValue_Errors(t *testing.T) {
	t.Parallel()

	t.Run("unset var is an error, not empty", func(t *testing.T) {
		t.Parallel()
		_, err := ExpandValue(t.Context(), "$MISSING", nil)
		require.Error(t, err)
	})

	t.Run("unset var inside braces is an error", func(t *testing.T) {
		t.Parallel()
		_, err := ExpandValue(t.Context(), "${MISSING}", nil)
		require.Error(t, err)
	})

	t.Run("unset var inside cmdsubst is an error", func(t *testing.T) {
		t.Parallel()
		_, err := ExpandValue(t.Context(), `$(printf '%s' "$MISSING")`, nil)
		require.Error(t, err)
	})

	t.Run("bad syntax returns error", func(t *testing.T) {
		t.Parallel()
		_, err := ExpandValue(t.Context(), "$(", nil)
		require.Error(t, err)
	})

	t.Run("inner non-zero exit returns error with exit code", func(t *testing.T) {
		t.Parallel()
		_, err := ExpandValue(t.Context(), "$(false)", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "exit status 1")
	})

	t.Run("inner explicit exit code is surfaced", func(t *testing.T) {
		t.Parallel()
		_, err := ExpandValue(t.Context(), "$(exit 7)", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "exit status 7")
	})

	t.Run("inner stderr is surfaced", func(t *testing.T) {
		t.Parallel()
		_, err := ExpandValue(
			t.Context(),
			`$(printf 'boom\n' 1>&2; exit 1)`,
			nil,
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), "boom")
	})

	t.Run("inner stderr is truncated to byte budget", func(t *testing.T) {
		t.Parallel()
		// Emit more than maxInnerStderrBytes bytes of 'X' on stderr.
		long := strings.Repeat("X", maxInnerStderrBytes*2)
		_, err := ExpandValue(
			t.Context(),
			`$(printf '`+long+`' 1>&2; exit 1)`,
			nil,
		)
		require.Error(t, err)
		require.NotContains(
			t,
			err.Error(),
			strings.Repeat("X", maxInnerStderrBytes+1),
			"stderr should be bounded",
		)
	})
}

func TestSanitizeStderr(t *testing.T) {
	t.Parallel()

	t.Run("trims trailing newlines", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "hi", sanitizeStderr([]byte("hi\n\n")))
	})

	t.Run("preserves tabs and embedded newlines", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "a\tb\nc", sanitizeStderr([]byte("a\tb\nc")))
	})

	t.Run("replaces control characters", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "a?b", sanitizeStderr([]byte{'a', 0x01, 'b'}))
	})

	t.Run("bounds output", func(t *testing.T) {
		t.Parallel()
		got := sanitizeStderr([]byte(strings.Repeat("x", maxInnerStderrBytes*2)))
		require.Len(t, got, maxInnerStderrBytes)
	})
}
