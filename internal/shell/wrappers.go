package shell

import (
	"path/filepath"
	"strings"
)

// baseName returns the final path element of a command name, so that an
// absolute or relative invocation (/usr/bin/curl, ./curl) is matched against
// the same name as a bare invocation (curl). It also normalizes Windows-style
// separators, since the bash tool runs POSIX emulation on all platforms and a
// command may be written with either separator.
func baseName(cmd string) string {
	if cmd == "" {
		return cmd
	}
	// Normalize backslashes so a Windows-style path is reduced too; on
	// POSIX, filepath.Base treats backslash as an ordinary character.
	cmd = strings.ReplaceAll(cmd, "\\", "/")
	return filepath.Base(cmd)
}

// commandWrapper describes a leaf binary that, after consuming its own
// options, execs another command given by its trailing arguments — nohup,
// env, timeout, nice, xargs, and similar. The block list only inspects the
// command it is handed, so a banned command placed behind such a wrapper
// ("nohup curl ...", "env wget ...", "timeout 5 nc ...") slips past it: the
// wrapper itself is not banned and it execs the real command as a child
// process, out of the interpreter's reach. unwrapCommand peels one such
// wrapper so the block list can be re-applied to the command that actually
// runs (see blockHandler, which loops to handle nesting).
//
// This is intentionally a recognition table for wrappers, not a denylist of
// dangerous commands. The set is bounded to leaf binaries whose documented
// purpose is to exec a command argument; the bannedCommands list it protects
// is unchanged. Shells/interpreters that take a command as a string
// (sh -c, bash -c, python3 -c, ...) are NOT wrappers in this sense — they
// take their command as opaque data, cannot be peeled, and remain out of
// scope of any command-name block list (see the package docs and PR notes).
type commandWrapper struct {
	// optsWithValue are the wrapper's own option flags that consume a
	// separate following token (e.g. nice -n 10). Long forms spelled
	// --flag=value are self-contained and need not be listed.
	optsWithValue map[string]struct{}
	// firstPositionalIsValue marks wrappers whose first non-option token is
	// an argument to the wrapper rather than the wrapped command (timeout's
	// DURATION, flock's lockfile, taskset's CPU mask, chrt's priority).
	firstPositionalIsValue bool
	// allowsAssignments marks wrappers that accept leading NAME=value
	// assignments before the command (env).
	allowsAssignments bool
	// splitStringFlags names the options (env's -S / --split-string) whose
	// value is itself a whitespace-separated command line that must be
	// re-tokenized rather than treated as a single opaque token.
	splitStringFlags map[string]struct{}
}

func opts(names ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(names))
	for _, n := range names {
		m[n] = struct{}{}
	}
	return m
}

// commandWrappers is the set of recognized exec wrappers. It is matched on
// filepath.Base of argv[0] (see unwrapCommand) so an absolute or relative
// path to the wrapper (/usr/bin/env, ./timeout) is recognized too.
var commandWrappers = map[string]commandWrapper{
	"nohup":   {},
	"setsid":  {},
	"nice":    {optsWithValue: opts("-n", "--adjustment")},
	"ionice":  {optsWithValue: opts("-c", "-n", "-p", "-P", "--class", "--classdata", "--pid", "--pgid")},
	"stdbuf":  {optsWithValue: opts("-i", "-o", "-e", "--input", "--output", "--error")},
	"chrt":    {optsWithValue: opts("-T", "-P"), firstPositionalIsValue: true},
	"taskset": {optsWithValue: opts("-c", "--cpu-list", "-p", "--pid"), firstPositionalIsValue: true},
	"runuser": {optsWithValue: opts("-u", "--user", "-g", "--group", "-G", "--supp-group", "-c", "--command")},
	"setpriv": {optsWithValue: opts("--reuid", "--regid", "--groups", "--inh-caps", "--ambient-caps", "--bounding-set", "--securebits", "--pdeathsig", "--selinux-label", "--apparmor-profile")},
	"flock":   {optsWithValue: opts("-w", "--timeout", "-E", "--conflict-exit-code"), firstPositionalIsValue: true},
	"timeout": {optsWithValue: opts("-s", "--signal", "-k", "--kill-after"), firstPositionalIsValue: true},
	"env":     {optsWithValue: opts("-u", "--unset", "-C", "--chdir"), allowsAssignments: true, splitStringFlags: opts("-S", "--split-string")},
	"xargs": {optsWithValue: opts(
		"-I", "-i", "-n", "-L", "-l", "-P", "-s", "-d", "-E", "-a",
		"--replace", "--max-args", "--max-procs", "--max-lines", "--delimiter",
		"--eof", "--arg-file", "--process-slot-var",
	)},
}

// wrapperFor reports the wrapper descriptor for argv[0], matching on the path
// basename so that /usr/bin/timeout and ./env are recognized as wrappers.
func wrapperFor(arg string) (commandWrapper, bool) {
	w, ok := commandWrappers[baseName(arg)]
	return w, ok
}

// unwrapCommand returns the inner command argv when args[0] is a recognized
// exec wrapper, and reports whether a wrapper was peeled. It strips the
// wrapper token, the wrapper's own option flags (and their values), any
// leading NAME=value assignments (env), and a leading value positional
// (timeout's DURATION). env's split-string form (`env -S 'curl ...'`) is
// re-tokenized on whitespace so the smuggled command is exposed. It peels
// exactly one layer; callers loop to handle nesting such as
// `nohup env timeout 5 curl`.
//
// When the wrapped command cannot be located (the wrapper is used with no
// trailing command, e.g. a bare `env` that just prints the environment) it
// returns ok=false so the caller leaves the original argv untouched.
func unwrapCommand(args []string) (inner []string, ok bool) {
	if len(args) == 0 {
		return args, false
	}
	w, isWrapper := wrapperFor(args[0])
	if !isWrapper {
		return args, false
	}
	i := 1
	for i < len(args) {
		tok := args[i]
		if w.allowsAssignments && !strings.HasPrefix(tok, "-") && strings.Contains(tok, "=") {
			i++
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			// "--" ends option processing; the command follows.
			if tok == "--" {
				i++
				break
			}
			name := tok
			value := ""
			hasInlineValue := false
			if before, after, found := strings.Cut(tok, "="); found {
				// --flag=value / -S=cmd: self-contained value.
				name = before
				value = after
				hasInlineValue = true
			}
			// env -S / --split-string: the value is a command line that
			// must be re-tokenized and dispatched, not consumed as opaque.
			if _, isSplit := w.splitStringFlags[name]; isSplit {
				if hasInlineValue {
					if fields := strings.Fields(value); len(fields) > 0 {
						return fields, true
					}
					return args, false
				}
				if i+1 < len(args) {
					if fields := strings.Fields(args[i+1]); len(fields) > 0 {
						return fields, true
					}
				}
				return args, false
			}
			if hasInlineValue {
				i++ // self-contained --flag=value
				continue
			}
			if _, consumesValue := w.optsWithValue[name]; consumesValue {
				i += 2 // flag plus its separate value
			} else {
				i++ // standalone flag, or a combined short flag like -n10
			}
			continue
		}
		// First bare positional.
		if w.firstPositionalIsValue {
			i++ // consume the wrapper's own value (timeout DURATION)
		}
		break
	}
	if i >= len(args) {
		return args, false
	}
	return args[i:], true
}
