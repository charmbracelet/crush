package shell

import (
	"slices"
	"strings"
)

// commandWrapper describes a command whose job is to run another command.
//
// Deciding anything about `nice curl` by looking at `nice` answers the wrong
// question: the command that runs is `curl`. Both the list of commands that
// may skip a prompt and the list of commands that must be refused need the
// same answer, so the peeling lives here, beside the matching, rather than
// in either list.
type commandWrapper struct {
	// name is the wrapper's command name.
	name string
	// valueFlags are flags that consume the following token as their
	// value, which must be skipped when hunting for the inner command
	// (`nice -n 10 ls` → the "10" is not the command).
	valueFlags []string
	// skipOperands is how many non-flag operands belong to the wrapper
	// itself before the inner command begins (`timeout 5 ls` → 1).
	skipOperands int
}

var commandWrappers = []commandWrapper{
	// `env` takes NAME=VALUE assignments before its command, and they are
	// deliberately not skipped here: an assignment is environment steering
	// of the same kind as a `FOO=bar cmd` prefix, so it has to fail the
	// same way. The assignment is left in place as the inner argv's first
	// token, which matches nothing and so resolves to itself.
	{name: "env", valueFlags: []string{"-u", "--unset", "-C", "--chdir", "-S", "--split-string"}},
	{name: "nohup"},
	{name: "nice", valueFlags: []string{"-n", "--adjustment"}},
	{name: "timeout", skipOperands: 1, valueFlags: []string{"-s", "--signal", "-k", "--kill-after"}},
}

// maxWrapperDepth bounds how many nested wrappers are unwrapped, so a
// pathological `nohup nohup nohup …` cannot spin.
const maxWrapperDepth = 4

// ResolveArgv reduces an argv to the command it would actually run, peeling
// off any command wrappers in front of it. `nice -n 10 timeout 5 curl x`
// resolves to `curl x`.
//
// An argv that is not wrapped, or a wrapper with nothing after it, resolves
// to itself. Peeling stops at [maxWrapperDepth].
func ResolveArgv(argv []string) []string {
	for depth := 0; depth < maxWrapperDepth; depth++ {
		inner, ok := peelWrapper(argv)
		if !ok {
			return argv
		}
		argv = inner
	}
	return argv
}

// peelWrapper strips a leading command wrapper and returns the command it
// would run. The second result is false when argv is not a wrapper, or is a
// wrapper with no inner command — `env` alone just prints the environment.
func peelWrapper(argv []string) ([]string, bool) {
	if len(argv) == 0 {
		return nil, false
	}
	idx := slices.IndexFunc(commandWrappers, func(w commandWrapper) bool {
		return w.name == normalizeCommand(argv[0])
	})
	if idx < 0 {
		return nil, false
	}
	w := commandWrappers[idx]

	rest := argv[1:]
	operandsSkipped := 0
	for len(rest) > 0 {
		tok := rest[0]
		switch {
		case tok == "--":
			rest = rest[1:]
			// Everything after -- is the inner command.
			if len(rest) == 0 {
				return nil, false
			}
			return rest, true
		case isFlag(tok):
			// A flag that takes a separate value consumes the next token
			// too, unless it was given as --flag=value.
			consumesValue := slices.Contains(w.valueFlags, flagName(tok)) &&
				!strings.Contains(tok, "=")
			rest = rest[1:]
			if consumesValue {
				if len(rest) == 0 {
					return nil, false
				}
				rest = rest[1:]
			}
		case operandsSkipped < w.skipOperands:
			operandsSkipped++
			rest = rest[1:]
		default:
			// First token that is not part of the wrapper's own
			// arguments: this is the command being wrapped.
			return rest, true
		}
	}
	// Wrapper with no inner command.
	return nil, false
}

// isFlag reports whether tok is a command-line flag rather than an operand.
func isFlag(tok string) bool {
	return len(tok) > 1 && strings.HasPrefix(tok, "-") && tok != "--"
}

// flagName strips any =value suffix from a flag token.
func flagName(tok string) string {
	name, _, _ := strings.Cut(tok, "=")
	return name
}
