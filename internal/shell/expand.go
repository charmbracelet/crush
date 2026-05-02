package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// maxInnerStderrBytes bounds how much stderr from a failing $(...) is
// surfaced in the returned error, to avoid leaking a secret that happened
// to be embedded in a failing inner command.
const maxInnerStderrBytes = 512

// ExpandValue expands shell-style substitutions in a single config value.
//
// Supported constructs match the bash tool:
//
//   - $VAR and ${VAR} (unset is an error; see nounset below).
//   - ${VAR:-default} / ${VAR:+alt} / ${VAR:?msg}.
//   - $(command) with full quoting and nesting.
//   - escaped and quoted strings ("...", '...').
//
// Contract:
//
//   - Returns exactly one string. No field splitting, no globbing, no
//     pathname generation. Multi-word command output is preserved
//     verbatim; it is never split into multiple values.
//   - Nounset is on: unset variables produce an error instead of
//     expanding to the empty string. Use ${VAR:-default} to opt in to
//     an empty fallback.
//   - Embedded whitespace and newlines in the input are preserved
//     verbatim. Command substitution strips trailing newlines only
//     (POSIX), never leading or internal whitespace.
//   - Errors wrap the failing inner command's exit code and a bounded
//     prefix of its stderr. Callers that surface the error to users
//     should additionally scrub it for the original template text.
func ExpandValue(ctx context.Context, value string, env []string) (string, error) {
	// Parse the value as a here-doc style word: no word splitting, no
	// globbing, but full support for $VAR, ${VAR...}, $(...), and
	// quoted/escaped strings.
	word, err := syntax.NewParser().Document(strings.NewReader(value))
	if err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}

	// Build a minimal Shell value purely to reuse its handler chain
	// (builtins, block funcs, optional Go coreutils) inside $(...).
	// We deliberately skip NewShell so the passed-in env is used
	// verbatim, with no CRUSH/AGENT/AI_AGENT injection: callers of
	// ExpandValue control the env, and nounset must treat any name
	// not in env as unset.
	cwd, _ := os.Getwd()
	s := &Shell{
		cwd:    cwd,
		env:    env,
		logger: noopLogger{},
	}

	var stderrBuf bytes.Buffer
	cfg := &expand.Config{
		Env:     expand.ListEnviron(env...),
		NoUnset: true,
		CmdSubst: func(w io.Writer, cs *syntax.CmdSubst) error {
			stderrBuf.Reset()
			runner, rerr := interp.New(
				interp.StdIO(nil, w, &stderrBuf),
				interp.Interactive(false),
				interp.Env(expand.ListEnviron(env...)),
				interp.Dir(s.cwd),
				interp.ExecHandlers(s.execHandlers()...),
				// Match the outer NoUnset: an unset $VAR inside
				// $(...) is also an error, not a silent empty.
				interp.Params("-u"),
			)
			if rerr != nil {
				return rerr
			}
			if rerr := runner.Run(ctx, &syntax.File{Stmts: cs.Stmts}); rerr != nil {
				return wrapCmdSubstErr(rerr, stderrBuf.Bytes())
			}
			return nil
		},
		// ReadDir / ReadDir2 left nil: globbing is disabled.
	}

	return expand.Document(cfg, word)
}

// wrapCmdSubstErr attaches a bounded prefix of the inner command's stderr
// to the original error, if any.
func wrapCmdSubstErr(err error, stderrBytes []byte) error {
	msg := sanitizeStderr(stderrBytes)
	if msg == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, msg)
}

// sanitizeStderr trims, bounds, and scrubs non-printable bytes from the
// stderr of a failing command so the result is safe to include in an
// error message shown to the user.
func sanitizeStderr(b []byte) string {
	b = bytes.TrimRight(b, "\n")
	if len(b) > maxInnerStderrBytes {
		b = b[:maxInnerStderrBytes]
	}
	out := make([]byte, len(b))
	for i, c := range b {
		if c == '\t' || c == '\n' || (c >= 0x20 && c < 0x7f) {
			out[i] = c
		} else {
			out[i] = '?'
		}
	}
	return string(out)
}
