package config

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/env"
	"github.com/stretchr/testify/require"
)

// fakeExpander returns a canned value/error for the last passed value and
// records the context, raw value, and env slice it was called with. It
// lets the config-layer tests assert on delegation behaviour without
// spinning up a real interpreter — real-shell coverage lives in
// internal/shell/expand_test.go and resolve_real_test.go.
type fakeExpander struct {
	expand    func(ctx context.Context, value string, env []string) (string, error)
	lastValue string
	lastEnv   []string
	calls     int
}

func (f *fakeExpander) Expand(ctx context.Context, value string, env []string) (string, error) {
	f.calls++
	f.lastValue = value
	f.lastEnv = env
	if f.expand == nil {
		return value, nil
	}
	return f.expand(ctx, value, env)
}

func TestShellVariableResolver_DelegatesToExpander(t *testing.T) {
	t.Parallel()

	fe := &fakeExpander{
		expand: func(_ context.Context, value string, _ []string) (string, error) {
			if value == "hello $FOO" {
				return "hello bar", nil
			}
			return value, nil
		},
	}

	e := env.NewFromMap(map[string]string{"FOO": "bar"})
	r := NewShellVariableResolver(e, WithExpander(fe.Expand))

	got, err := r.ResolveValue("hello $FOO")
	require.NoError(t, err)
	require.Equal(t, "hello bar", got)
	require.Equal(t, 1, fe.calls)
	require.Equal(t, "hello $FOO", fe.lastValue)
	require.Contains(t, fe.lastEnv, "FOO=bar")
}

func TestShellVariableResolver_LoneDollarIsError(t *testing.T) {
	t.Parallel()

	// Lone "$" must short-circuit before reaching the expander: the
	// underlying shell parser would accept it as a literal, but this
	// resolver has historically rejected it and callers depend on
	// that early-fail behaviour.
	fe := &fakeExpander{}
	r := NewShellVariableResolver(env.NewFromMap(nil), WithExpander(fe.Expand))

	_, err := r.ResolveValue("$")
	require.Error(t, err)
	require.Equal(t, 0, fe.calls, "expander must not be called for lone $")
}

func TestShellVariableResolver_PassesThroughLiterals(t *testing.T) {
	t.Parallel()

	fe := &fakeExpander{
		expand: func(_ context.Context, value string, _ []string) (string, error) {
			return value, nil
		},
	}
	r := NewShellVariableResolver(env.NewFromMap(nil), WithExpander(fe.Expand))

	got, err := r.ResolveValue("plain-string")
	require.NoError(t, err)
	require.Equal(t, "plain-string", got)
}

func TestShellVariableResolver_WrapsErrorsWithTemplate(t *testing.T) {
	t.Parallel()

	inner := errors.New("cat: /run/secrets/x: permission denied")
	fe := &fakeExpander{
		expand: func(_ context.Context, _ string, _ []string) (string, error) {
			return "", inner
		},
	}
	r := NewShellVariableResolver(env.NewFromMap(nil), WithExpander(fe.Expand))

	_, err := r.ResolveValue("$(cat /run/secrets/x)")
	require.Error(t, err)
	require.ErrorIs(t, err, inner)
	require.Contains(t, err.Error(), "$(cat /run/secrets/x)")
	require.Contains(t, err.Error(), "permission denied")
}

func TestSanitizeResolveError(t *testing.T) {
	t.Parallel()

	t.Run("nil passes through", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, sanitizeResolveError("anything", nil))
	})

	t.Run("includes template and wraps inner", func(t *testing.T) {
		t.Parallel()
		inner := errors.New("cat: /run/secrets/x: permission denied")
		got := sanitizeResolveError("$(cat /run/secrets/x)", inner)
		require.Error(t, got)
		require.ErrorIs(t, got, inner)
		require.Contains(t, got.Error(), "$(cat /run/secrets/x)")
		require.Contains(t, got.Error(), "permission denied")
	})

	t.Run("unwrap preserves original for errors.Is", func(t *testing.T) {
		t.Parallel()
		inner := errors.New("sentinel")
		got := sanitizeResolveError("$FOO", inner)
		require.ErrorIs(t, got, inner)
	})

	t.Run("truncates over-budget inner message", func(t *testing.T) {
		t.Parallel()
		// Inner message holds far more than the budget. After
		// sanitization the rendered inner portion must not exceed
		// maxResolveErrBytes, and the characters beyond the budget
		// (marked by a distinct tail sentinel) must be gone.
		const tailSentinel = "TAIL_SENTINEL_BEYOND_BUDGET"
		body := strings.Repeat("x", maxResolveErrBytes)
		inner := errors.New(body + tailSentinel)

		got := sanitizeResolveError("$TEMPLATE", inner)
		require.Error(t, got)

		prefix := `resolving "$TEMPLATE": `
		rendered := got.Error()
		require.True(
			t,
			strings.HasPrefix(rendered, prefix),
			"rendered error must start with template prefix",
		)
		innerRendered := strings.TrimPrefix(rendered, prefix)
		require.LessOrEqual(
			t,
			len(innerRendered),
			maxResolveErrBytes,
			"inner message must be bounded to maxResolveErrBytes",
		)
		require.NotContains(
			t,
			rendered,
			tailSentinel,
			"content past the budget must not leak",
		)
	})

	t.Run("replaces non-printable bytes", func(t *testing.T) {
		t.Parallel()
		// NUL, BEL, ESC, DEL, and a UTF-8 high byte should all be
		// scrubbed to '?'. Tab and newline are preserved because
		// they show up legitimately in command stderr.
		inner := errors.New("ok\x00bad\x07\x1b\x7f\xffend\ttab\nline")
		got := sanitizeResolveError("$T", inner)
		rendered := got.Error()

		require.NotContains(t, rendered, "\x00")
		require.NotContains(t, rendered, "\x07")
		require.NotContains(t, rendered, "\x1b")
		require.NotContains(t, rendered, "\x7f")
		require.NotContains(t, rendered, "\xff")
		require.Contains(t, rendered, "ok?bad????end\ttab\nline")
	})

	t.Run("scrubbing does not depend on shell.ExpandValue upstream", func(t *testing.T) {
		t.Parallel()
		// A custom Expander can inject arbitrary error text. The
		// config-layer helper is the single chokepoint; it must
		// bound + scrub regardless of the error source.
		nasty := strings.Repeat("A", maxResolveErrBytes+64) + "\x00BEYOND"
		fe := &fakeExpander{
			expand: func(_ context.Context, _ string, _ []string) (string, error) {
				return "", errors.New(nasty)
			},
		}
		r := NewShellVariableResolver(env.NewFromMap(nil), WithExpander(fe.Expand))

		_, err := r.ResolveValue("$T")
		require.Error(t, err)
		require.NotContains(t, err.Error(), "BEYOND", "over-budget tail must not leak")
		require.NotContains(t, err.Error(), "\x00", "non-printables must be scrubbed")
	})
}

func TestScrubErrorMessage(t *testing.T) {
	t.Parallel()

	t.Run("bounds output to maxResolveErrBytes", func(t *testing.T) {
		t.Parallel()
		got := scrubErrorMessage(strings.Repeat("a", maxResolveErrBytes*3))
		require.Len(t, got, maxResolveErrBytes)
	})

	t.Run("preserves printable ASCII tab and newline", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "a\tb\nc d!", scrubErrorMessage("a\tb\nc d!"))
	})

	t.Run("replaces control and non-ASCII bytes", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "a?b??c", scrubErrorMessage("a\x01b\x1b\xe2c"))
	})
}

func TestEnvironmentVariableResolver_ResolveValue(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		envVars     map[string]string
		expected    string
		expectError bool
	}{
		{
			name:     "non-variable string returns as-is",
			value:    "plain-string",
			expected: "plain-string",
		},
		{
			name:     "environment variable resolution",
			value:    "$HOME",
			envVars:  map[string]string{"HOME": "/home/user"},
			expected: "/home/user",
		},
		{
			name:     "environment variable with complex value",
			value:    "$PATH",
			envVars:  map[string]string{"PATH": "/usr/bin:/bin:/usr/local/bin"},
			expected: "/usr/bin:/bin:/usr/local/bin",
		},
		{
			name:        "missing environment variable returns error",
			value:       "$MISSING_VAR",
			envVars:     map[string]string{},
			expectError: true,
		},
		{
			name:        "empty environment variable returns error",
			value:       "$EMPTY_VAR",
			envVars:     map[string]string{"EMPTY_VAR": ""},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testEnv := env.NewFromMap(tt.envVars)
			resolver := NewEnvironmentVariableResolver(testEnv)

			result, err := resolver.ResolveValue(tt.value)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestNewShellVariableResolver(t *testing.T) {
	testEnv := env.NewFromMap(map[string]string{"TEST": "value"})
	resolver := NewShellVariableResolver(testEnv)

	require.NotNil(t, resolver)
	require.Implements(t, (*VariableResolver)(nil), resolver)
}

func TestNewEnvironmentVariableResolver(t *testing.T) {
	testEnv := env.NewFromMap(map[string]string{"TEST": "value"})
	resolver := NewEnvironmentVariableResolver(testEnv)

	require.NotNil(t, resolver)
	require.Implements(t, (*VariableResolver)(nil), resolver)
}
