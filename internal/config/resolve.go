package config

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/shell"
)

// resolveTimeout bounds how long a single ResolveValue call may spend
// inside shell expansion (including any command substitution).
const resolveTimeout = 5 * time.Minute

type VariableResolver interface {
	ResolveValue(value string) (string, error)
}

// argumentResolver is implemented by resolvers that expand argv items
// differently from other values. The shell resolver keeps backslash
// pairs in arguments; see shell.ExpandArgument.
type argumentResolver interface {
	ResolveArgument(value string) (string, error)
}

// resolveArgument expands one MCP or LSP argv item. It uses
// ResolveArgument when r has it and falls back to ResolveValue.
func resolveArgument(r VariableResolver, value string) (string, error) {
	if ar, ok := r.(argumentResolver); ok {
		return ar.ResolveArgument(value)
	}
	return r.ResolveValue(value)
}

// identityResolver is a no-op resolver that returns values unchanged.
// Used in client mode where variable resolution is handled server-side.
type identityResolver struct{}

func (identityResolver) ResolveValue(value string) (string, error) {
	return value, nil
}

// IdentityResolver returns a VariableResolver that passes values through
// unchanged.
func IdentityResolver() VariableResolver {
	return identityResolver{}
}

// Expander is the single-value shell expansion seam used by
// shellVariableResolver. Production wires it to shell.ExpandValue; tests
// can inject a fake via WithExpander.
type Expander func(ctx context.Context, value string, env []string) (string, error)

// ShellResolverOption customizes shell variable resolver construction.
type ShellResolverOption func(*shellVariableResolver)

// WithExpander overrides the expansion function used by the resolver,
// for argv items too. Primarily intended for tests; production callers
// should not need this.
func WithExpander(e Expander) ShellResolverOption {
	return func(r *shellVariableResolver) {
		if e != nil {
			r.expand = e
			r.expandArg = e
		}
	}
}

type shellVariableResolver struct {
	env       env.Env
	expand    Expander
	expandArg Expander
}

// NewShellVariableResolver returns a VariableResolver that delegates to
// the embedded shell (the same interpreter used by the bash tool and
// hooks). Supported constructs match shell.ExpandValue: $VAR, ${VAR},
// ${VAR:-default}, $(command), quoting, and escapes. Unset variables
// expand to the empty string by default, matching bash; use
// ${VAR:?message} to require a value and fail loudly when it is missing.
// The stricter "unset is always an error" mode is gated globally by
// shell.NoUnset.
func NewShellVariableResolver(e env.Env, opts ...ShellResolverOption) VariableResolver {
	r := &shellVariableResolver{
		env:       e,
		expand:    shell.ExpandValue,
		expandArg: shell.ExpandArgument,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// ResolveValue resolves shell-style substitution anywhere in the string:
//
//   - $(command) for command substitution, with full quoting and nesting.
//   - $VAR and ${VAR} for environment variables.
//   - ${VAR:-default} / ${VAR:+alt} / ${VAR:?msg} for defaulting.
//
// Unset variables expand to the empty string by default, matching bash.
// Command-substitution failures are always a hard error. Required
// credentials should use ${VAR:?message} so a missing variable fails
// loudly at load time instead of quietly resolving to empty. Global
// strict mode is available via shell.NoUnset for callers that want the
// old nounset-on behaviour back.
func (r *shellVariableResolver) ResolveValue(value string) (string, error) {
	return r.resolve(value, r.expand)
}

// ResolveArgument is ResolveValue for one argv item. It keeps backslash
// pairs, so Windows named pipes and UNC paths in MCP and LSP args reach
// the child process unchanged. See shell.ExpandArgument.
func (r *shellVariableResolver) ResolveArgument(value string) (string, error) {
	return r.resolve(value, r.expandArg)
}

func (r *shellVariableResolver) resolve(value string, expand Expander) (string, error) {
	// Preserve the historical backward-compat contract: a lone "$" is a
	// malformed config value, not a legal literal. The underlying shell
	// parser would accept it as a literal; we reject it here so existing
	// configs that relied on this validation still fail early.
	if value == "$" {
		return "", fmt.Errorf("invalid value format: %s", value)
	}

	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()

	out, err := expand(ctx, value, r.env.Env())
	if err != nil {
		return "", sanitizeResolveError(value, err)
	}
	return out, nil
}

// maxResolveErrBytes bounds the size of the inner error message surfaced
// from a resolution failure. Defense-in-depth on top of shell.ExpandValue's
// own stderr budget: a custom Expander injected via WithExpander, or any
// future non-shell error path, must still produce a user-safe message.
const maxResolveErrBytes = 512

// sanitizeResolveError wraps an expansion error with the user-written
// template (the pre-expansion string — it is what they typed, safe to
// surface) and a bounded, scrubbed rendering of the inner error message.
// Contract:
//
//   - Never includes the resolved (post-expansion) value. This helper
//     only receives the template and err, so a successful expansion
//     result cannot reach it.
//   - May include the template verbatim.
//   - Truncates the inner error's message to maxResolveErrBytes and
//     replaces embedded NULs and other non-printables (except tab and
//     newline) with '?'.
//
// The returned error still unwraps to the original for errors.Is/As so
// callers can inspect typed sentinels; only the rendered message is
// scrubbed.
func sanitizeResolveError(template string, err error) error {
	if err == nil {
		return nil
	}
	return &resolveError{
		template: template,
		msg:      scrubErrorMessage(err.Error()),
		inner:    err,
	}
}

// resolveError is the concrete type returned by sanitizeResolveError.
// Its Error() method returns the template + scrubbed inner message;
// Unwrap exposes the original error so errors.Is/As continue to work.
type resolveError struct {
	template string
	msg      string
	inner    error
}

func (e *resolveError) Error() string {
	return fmt.Sprintf("resolving %q: %s", e.template, e.msg)
}

func (e *resolveError) Unwrap() error { return e.inner }

// scrubErrorMessage bounds the message to maxResolveErrBytes bytes and
// replaces non-printable bytes (anything outside ASCII printable, tab, or
// newline) with '?'. Mirrors shell.sanitizeStderr but operates on a
// string rather than raw command stderr and runs at the config layer,
// so arbitrary Expander error text is also sanitized.
func scrubErrorMessage(s string) string {
	if len(s) > maxResolveErrBytes {
		s = s[:maxResolveErrBytes]
	}
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\t' || c == '\n' || (c >= 0x20 && c < 0x7f) {
			out[i] = c
			continue
		}
		out[i] = '?'
	}
	return string(out)
}
