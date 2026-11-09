//go:build windows

package shell

import "strings"

// normalizeHomeForPOSIXShell rewrites HOME's backslashes to forward slashes.
//
// mvdan.cc/sh/v3 is a POSIX shell emulator: outside quotes, `\X` is an escape
// sequence that yields the literal `X`, consuming the backslash. On Windows,
// $HOME commonly contains backslashes (e.g. from MSYS/git-bash setting
// HOME=C:\Users\<user> when spawning crush), so any command using `~` -- the
// POSIX shorthand for $HOME -- gets that value mangled: the tokenizer eats
// every backslash, turning C:\Users\<user> into C:Users<user>. Hook commands
// silently fail with exit 127 as a result, with no signal in the TUI.
//
// Forward slashes are accepted by Windows path APIs and have no special
// meaning to the POSIX tokenizer, so rewriting only HOME's value (not
// arbitrary user-authored backslashes elsewhere in a command) fixes `~`
// expansion without touching legitimately-escaped shell syntax.
func normalizeHomeForPOSIXShell(env []string) []string {
	for i, e := range env {
		key, value, ok := strings.Cut(e, "=")
		if !ok || key != "HOME" {
			continue
		}
		result := make([]string, len(env))
		copy(result, env)
		result[i] = key + "=" + strings.ReplaceAll(value, `\`, "/")
		return result
	}
	return env
}
