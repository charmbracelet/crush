package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/env"
	"github.com/stretchr/testify/require"
)

// These tests exercise the full shell-expansion path (no mocks,
// no injected Expander) to catch regressions that only surface when
// internal/shell actually runs the value. Table-level unit tests with
// fake expanders live in resolve_test.go.

// realShellResolver builds a resolver backed by a shell env that
// contains PATH + the caller-supplied overrides. Production callers
// get PATH for free via env.New(); these tests need it so $(cat ...)
// and similar inner commands can resolve.
func realShellResolver(vars map[string]string) VariableResolver {
	m := map[string]string{"PATH": os.Getenv("PATH")}
	maps.Copy(m, vars)
	return NewShellVariableResolver(env.NewFromMap(m))
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "secret")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

// TestResolvedEnv_RealShell_Success covers the shell constructs the
// PLAN calls out: $(cat tempfile) with and without trailing newline,
// ${VAR:-default} for unset vars, literal-space preservation around
// $(...), nested parens, quoted args inside $(echo ...), and a
// glob-like literal round-tripping unchanged.
func TestResolvedEnv_RealShell_Success(t *testing.T) {
	t.Parallel()

	// filepath.ToSlash so Windows temp paths (C:\Users\...) survive
	// being injected into a shell command string — the embedded shell
	// treats backslashes as escapes, forward slashes work on every OS.
	withNL := filepath.ToSlash(writeTempFile(t, "token-with-nl\n"))
	noNL := filepath.ToSlash(writeTempFile(t, "token-no-nl"))

	m := MCPConfig{
		Env: map[string]string{
			// POSIX strips exactly one trailing newline from $(...)
			// output, so both forms land on the same value.
			"TOK_NL": fmt.Sprintf("$(cat %s)", withNL),
			"TOK_NO": fmt.Sprintf("$(cat %s)", noNL),

			// ${VAR:-default} must not error on unset: this is the
			// opt-in escape hatch for "empty is fine".
			"FALLBACK": "${MCP_MISSING:-fallback}",

			// Leading/trailing literal spaces around $(...) must be
			// preserved — single-value contract, no field splitting.
			"PADDED": "  $(echo v)  ",

			// ")" inside a quoted arg to echo is a regression guard
			// for the old hand-rolled paren matcher.
			"PAREN": `$(echo ")")`,

			// Embedded space inside a quoted arg must survive
			// verbatim; no word-splitting side effect.
			"SPACEY": `$(echo "a b")`,

			// Glob-like literals must not expand.
			"GLOB": "*.go",
		},
	}

	got, err := m.ResolvedEnv(realShellResolver(nil))
	require.NoError(t, err)

	// ResolvedEnv returns "KEY=value" sorted by key.
	want := []string{
		"FALLBACK=fallback",
		"GLOB=*.go",
		"PADDED=  v  ",
		"PAREN=)",
		"SPACEY=a b",
		"TOK_NL=token-with-nl",
		"TOK_NO=token-no-nl",
	}
	require.Equal(t, want, got)
}

// TestResolvedEnv_RealShell_DoesNotMutate pins that both success and
// failure paths leave m.Env untouched. Prior behaviour rewrote the
// value in place on error; that was the exact mechanism that shipped
// empty credentials to MCP servers.
func TestResolvedEnv_RealShell_DoesNotMutate(t *testing.T) {
	t.Parallel()

	t.Run("success path leaves Env untouched", func(t *testing.T) {
		t.Parallel()
		m := MCPConfig{Env: map[string]string{"TOKEN": "$(echo shh)"}}
		orig := maps.Clone(m.Env)

		_, err := m.ResolvedEnv(realShellResolver(nil))
		require.NoError(t, err)
		require.Equal(t, orig, m.Env)
	})

	t.Run("failure path leaves Env untouched", func(t *testing.T) {
		t.Parallel()
		m := MCPConfig{Env: map[string]string{"BROKEN": "$(false)"}}
		orig := maps.Clone(m.Env)

		_, err := m.ResolvedEnv(realShellResolver(nil))
		require.Error(t, err)
		require.Equal(t, orig, m.Env, "map must be preserved on error")
	})
}

// TestResolvedEnv_RealShell_Idempotent pins the pure-function contract:
// two calls on the same config return deeply-equal slices.
func TestResolvedEnv_RealShell_Idempotent(t *testing.T) {
	t.Parallel()

	m := MCPConfig{
		Env: map[string]string{
			"A": "$(echo one)",
			"B": "$(echo two)",
			"C": "literal",
		},
	}
	r := realShellResolver(nil)

	first, err := m.ResolvedEnv(r)
	require.NoError(t, err)
	second, err := m.ResolvedEnv(r)
	require.NoError(t, err)
	require.Equal(t, first, second)
}

// TestResolvedEnv_RealShell_Deterministic guards against Go's
// randomized map iteration leaking into the returned slice order.
func TestResolvedEnv_RealShell_Deterministic(t *testing.T) {
	t.Parallel()

	m := MCPConfig{Env: map[string]string{
		"Z": "z",
		"A": "a",
		"M": "m",
	}}

	got, err := m.ResolvedEnv(realShellResolver(nil))
	require.NoError(t, err)
	require.True(t, slices.IsSorted(got), "env slice must be sorted; got %v", got)
}

// TestResolvedEnv_RealShell_UnsetExpandsEmpty pins Phase 2's lenient
// default: an unset bare $VAR expands to the empty string, matching
// bash. The silent-empty-credential class of bug that motivated Phase
// 1's nounset-on default is already prevented by the pure-function
// error-returning contract of ResolvedEnv, so we no longer rely on
// nounset to catch typo'd variable names. Users who want strict
// behaviour for a required credential opt in per-reference with
// ${VAR:?msg}; see TestResolvedEnv_RealShell_ColonQuestionIsStrict.
func TestResolvedEnv_RealShell_UnsetExpandsEmpty(t *testing.T) {
	t.Parallel()

	m := MCPConfig{Env: map[string]string{
		// Intentional typo: user meant $FORGEJO_TOKEN. Under Phase 2
		// defaults this expands to "" rather than erroring, matching
		// bash's behaviour on bare $VAR.
		"FORGEJO_ACCESS_TOKEN": "$FORGJO_TOKEN",
	}}
	got, err := m.ResolvedEnv(realShellResolver(nil))
	require.NoError(t, err, "unset var must expand to empty, not error")
	require.Equal(t, []string{"FORGEJO_ACCESS_TOKEN="}, got)
}

// TestResolvedEnv_RealShell_ColonQuestionIsStrict pins the opt-in
// strictness contract: ${VAR:?msg} must hard-error when VAR is unset,
// regardless of the global NoUnset toggle. This is the recommended
// mechanism for required credentials under the lenient default, so a
// future refactor that accidentally swallows ${VAR:?...} errors would
// silently ship empty tokens to MCP servers again.
func TestResolvedEnv_RealShell_ColonQuestionIsStrict(t *testing.T) {
	t.Parallel()

	m := MCPConfig{Env: map[string]string{
		"FORGEJO_ACCESS_TOKEN": "${FORGJO_TOKEN:?set FORGJO_TOKEN}",
	}}
	got, err := m.ResolvedEnv(realShellResolver(nil))
	require.Error(t, err, "${VAR:?msg} must error when VAR is unset")
	require.Nil(t, got)
	// The resolver wraps with the env key and the user-written
	// template; the inner shell error carries the :? message so
	// users learn which credential is missing and why.
	msg := err.Error()
	require.Contains(t, msg, "FORGEJO_ACCESS_TOKEN")
	require.Contains(t, msg, "${FORGJO_TOKEN:?set FORGJO_TOKEN}")
	require.Contains(t, msg, "set FORGJO_TOKEN")
}

// TestResolvedEnv_RealShell_FailureDetail pins that a failing inner
// command surfaces enough detail (exit code + stderr on POSIX, the
// underlying OS error on Windows where coreutils runs in-process) to
// diagnose without forcing the user to re-run the command by hand.
// Also verifies the template is included so they know which Env
// entry blew up.
func TestResolvedEnv_RealShell_FailureDetail(t *testing.T) {
	t.Parallel()

	// Forward slashes so the path survives shell-string injection on
	// Windows; see TestResolvedEnv_RealShell_Success for the same note.
	missing := filepath.ToSlash(filepath.Join(t.TempDir(), "definitely-not-here"))
	m := MCPConfig{Env: map[string]string{
		"FORGEJO_ACCESS_TOKEN": fmt.Sprintf("$(cat %s)", missing),
	}}

	_, err := m.ResolvedEnv(realShellResolver(nil))
	require.Error(t, err)
	msg := err.Error()
	require.Contains(t, msg, "FORGEJO_ACCESS_TOKEN", "must identify the failing env var")
	require.Contains(t, msg, missing, "must include the template so users see what failed")

	// Inner diagnostic detail must survive. POSIX surfaces "exit
	// status N" + stderr; Windows' in-process coreutils surfaces the
	// Go OS error instead. Accept either shape so the test is
	// portable without weakening the intent.
	lower := strings.ToLower(msg)
	hasDetail := strings.Contains(lower, "exit status") ||
		strings.Contains(lower, "no such file") ||
		strings.Contains(lower, "cannot find")
	require.True(t, hasDetail, "must surface inner error detail: %s", msg)
}

// TestResolvedHeaders_RealShell_FailurePreservesOriginal pins two
// invariants simultaneously: on failure the returned map is nil (not
// a partially-populated map) and the receiver's Headers map is
// unchanged. A test that only asserted on the returned value could
// hide an in-place mutation regression.
func TestResolvedHeaders_RealShell_FailurePreservesOriginal(t *testing.T) {
	t.Parallel()

	m := MCPConfig{Headers: map[string]string{
		"Authorization": "Bearer $(false)",
		"X-Static":      "kept",
	}}
	orig := maps.Clone(m.Headers)

	got, err := m.ResolvedHeaders(realShellResolver(nil))
	require.Error(t, err)
	require.Nil(t, got, "headers map must be nil on failure")
	require.Contains(t, err.Error(), "header Authorization")
	require.Equal(t, orig, m.Headers, "receiver Headers must be preserved")
}

// TestResolvedHeaders_RealShell_DropEmpty pins Phase 2 design
// decision #18 on the MCP side: a header whose value resolves to the
// empty string is omitted from the returned map. Covers the three
// ways a value can legitimately land on empty — unset bare $VAR
// under lenient nounset, a literal "", and a non-failing command
// whose stdout is empty — and also pins that a failing $(cmd) still
// errors rather than silently dropping.
func TestResolvedHeaders_RealShell_DropEmpty(t *testing.T) {
	t.Parallel()

	t.Run("unset $VAR is absent", func(t *testing.T) {
		t.Parallel()
		m := MCPConfig{Headers: map[string]string{
			"X-Missing": "$MCP_HEADER_NEVER_SET",
			"X-Kept":    "present",
		}}
		got, err := m.ResolvedHeaders(realShellResolver(nil))
		require.NoError(t, err)
		_, present := got["X-Missing"]
		require.False(t, present, "unset bare $VAR → empty → header dropped")
		require.Equal(t, "present", got["X-Kept"])
	})

	t.Run("literal empty string is absent", func(t *testing.T) {
		t.Parallel()
		m := MCPConfig{Headers: map[string]string{
			"X-Custom": "",
			"X-Kept":   "present",
		}}
		got, err := m.ResolvedHeaders(realShellResolver(nil))
		require.NoError(t, err)
		_, present := got["X-Custom"]
		require.False(t, present, "literal empty-string header must be dropped")
		require.Equal(t, "present", got["X-Kept"])
	})

	t.Run("$(echo) is absent", func(t *testing.T) {
		t.Parallel()
		m := MCPConfig{Headers: map[string]string{
			"X-Empty": "$(echo)",
			"X-Kept":  "present",
		}}
		got, err := m.ResolvedHeaders(realShellResolver(nil))
		require.NoError(t, err)
		_, present := got["X-Empty"]
		require.False(t, present, "$(echo) → empty → header dropped")
		require.Equal(t, "present", got["X-Kept"])
	})

	t.Run("$(false) errors and does not mutate", func(t *testing.T) {
		t.Parallel()
		m := MCPConfig{Headers: map[string]string{
			"X-Broken": "$(false)",
			"X-Kept":   "present",
		}}
		orig := maps.Clone(m.Headers)

		got, err := m.ResolvedHeaders(realShellResolver(nil))
		require.Error(t, err)
		require.Empty(t, got, "map must be nil/empty on failure, not a partial")
		require.Contains(t, err.Error(), "header X-Broken")
		require.Equal(t, orig, m.Headers, "receiver Headers must be preserved")
	})
}

// TestResolvedArgs_RealShell exercises both success and failure for
// m.Args symmetrically with Env. Args are ordered so error messages
// must identify a positional index, not a key.
func TestResolvedArgs_RealShell(t *testing.T) {
	t.Parallel()

	t.Run("success expands each element", func(t *testing.T) {
		t.Parallel()
		m := MCPConfig{Args: []string{"--token", "$(echo shh)", "--host", "example.com"}}
		got, err := m.ResolvedArgs(realShellResolver(nil))
		require.NoError(t, err)
		require.Equal(t, []string{"--token", "shh", "--host", "example.com"}, got)
	})

	t.Run("failure identifies offending index", func(t *testing.T) {
		t.Parallel()
		m := MCPConfig{Args: []string{"--token", "$(false)"}}
		orig := slices.Clone(m.Args)

		got, err := m.ResolvedArgs(realShellResolver(nil))
		require.Error(t, err)
		require.Nil(t, got)
		require.Contains(t, err.Error(), "arg 1")
		require.Equal(t, orig, m.Args, "receiver Args must be preserved")
	})

	t.Run("nil args returns nil, no error", func(t *testing.T) {
		t.Parallel()
		m := MCPConfig{}
		got, err := m.ResolvedArgs(realShellResolver(nil))
		require.NoError(t, err)
		require.Nil(t, got)
	})
}

// TestMCPConfig_IdentityResolver pins the client-mode contract: every
// Resolved* method round-trips the template verbatim and never errors
// on unset variables. Local expansion would double-expand when the
// server does its own — this has to stay a pure pass-through.
func TestMCPConfig_IdentityResolver(t *testing.T) {
	t.Parallel()

	m := MCPConfig{
		Command: "$CMD",
		Args:    []string{"--token", "$MCP_MISSING_TOKEN", "$(vault read -f secret)"},
		Env: map[string]string{
			"TOKEN": "$(cat /run/secrets/x)",
			"HOST":  "$MCP_MISSING_HOST",
		},
		Headers: map[string]string{
			"Authorization": "Bearer $(vault read -f token)",
		},
		URL: "https://$MCP_HOST/$(vault read -f path)",
	}
	r := IdentityResolver()

	args, err := m.ResolvedArgs(r)
	require.NoError(t, err)
	require.Equal(t, m.Args, args)

	envs, err := m.ResolvedEnv(r)
	require.NoError(t, err)
	// Sorted "KEY=value".
	require.Equal(t, []string{
		"HOST=$MCP_MISSING_HOST",
		"TOKEN=$(cat /run/secrets/x)",
	}, envs)

	headers, err := m.ResolvedHeaders(r)
	require.NoError(t, err)
	require.Equal(t, m.Headers, headers)

	u, err := m.ResolvedURL(r)
	require.NoError(t, err)
	require.Equal(t, m.URL, u)
}
