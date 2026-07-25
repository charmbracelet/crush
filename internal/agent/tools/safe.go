package tools

import (
	"runtime"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// safeCommand describes one command form that is read-only enough to run
// without a permission prompt.
//
// argv is matched as an exact leading token sequence, never as a string
// prefix: an entry for {"git", "status"} matches the command `git status`
// and not `git status-something`, and — because the tokens come from the
// parsed AST — not `git -c core.pager=... status` either, since that argv
// begins with different tokens.
type safeCommand struct {
	// argv is the exact leading token sequence this entry matches.
	argv []string
	// restrictFlags limits the accepted flags to allowFlags. When false,
	// any flag not in denyFlags is accepted. It exists so that a command
	// whose every flag is harmless (`ls`, `df`) does not have to enumerate
	// them, while one that can mutate under a flag (`git branch`) does.
	restrictFlags bool
	// allowFlags is the set of accepted flag names when restrictFlags is
	// set. Compared against the flag name only, so `--format=x` is matched
	// by an entry of "--format".
	allowFlags []string
	// denyFlags is the set of rejected flag names when restrictFlags is
	// not set.
	denyFlags []string
	// requireFlag demands that at least one flag be present. It exists
	// for `git config`, where the read-only forms are distinguished from
	// the write form by a flag rather than by a subcommand token:
	// `git config --get x` reads, `git config x y` writes.
	requireFlag bool
	// allowOperands reports whether non-flag arguments may follow argv.
	// It is false for commands that mutate when handed an operand, such
	// as `git branch <name>` (creates) or `git remote add` (writes).
	allowOperands bool
}

// gitCodeExecFlags are flags that make an otherwise read-only git
// subcommand run a program or write a file: external diff drivers,
// pager handoff, and explicit output redirection. They are rejected
// everywhere they are accepted as flags.
var gitCodeExecFlags = []string{
	"--ext-diff",
	"--open-files-in-pager",
	"-O",
	"--output",
	"--output-indicator-new",
	"--upload-pack",
	"--receive-pack",
	"--exec",
}

// safeCommands are the command forms that skip the permission prompt.
//
// The bar for membership is that the command cannot write to the
// filesystem, mutate repository or system state, execute another program,
// or reach the network. Anything that fails that bar — including a
// read-only command that becomes destructive under a flag — either gets a
// restricted flag set here or stays off the list entirely.
//
// Deliberately absent:
//
//   - env, nice, nohup, timeout: these execute an arbitrary command given
//     to them. They are handled by peelWrapper instead, which unwraps
//     them and re-checks whatever is inside. (`time` needs no entry: it
//     is a shell keyword, handled in safeStmt.)
//   - kill, killall: signal arbitrary processes.
//   - set, unset: mutate shell state.
//   - git branch/tag/remote with operands, and any git subcommand that
//     writes: see the restricted entries below.
//   - git ls-remote, and nslookup/ping on Windows: these reach the
//     network, which sits badly beside a block list that bans curl/wget.
var safeCommands = []safeCommand{
	// Bash builtins and core utils. Every flag these accept is read-only,
	// so they carry no flag restrictions.
	{argv: []string{"cal"}, allowOperands: true},
	// `date -s` sets the system clock.
	{argv: []string{"date"}, denyFlags: []string{"-s", "--set"}, allowOperands: true},
	{argv: []string{"df"}, allowOperands: true},
	{argv: []string{"du"}, allowOperands: true},
	// echo is safe only because redirections are rejected outright; with
	// a redirect it becomes an arbitrary file write. Do not add redirect
	// support without revisiting this entry.
	{argv: []string{"echo"}, allowOperands: true},
	{argv: []string{"free"}, allowOperands: true},
	{argv: []string{"groups"}, allowOperands: true},
	{argv: []string{"hostname"}, allowOperands: false},
	{argv: []string{"id"}, allowOperands: true},
	{argv: []string{"ls"}, allowOperands: true},
	{argv: []string{"printenv"}, allowOperands: true},
	{argv: []string{"ps"}, allowOperands: true},
	{argv: []string{"pwd"}, allowOperands: false},
	{argv: []string{"top"}, allowOperands: true},
	{argv: []string{"type"}, allowOperands: true},
	{argv: []string{"uname"}, allowOperands: true},
	{argv: []string{"uptime"}, allowOperands: true},
	{argv: []string{"whatis"}, allowOperands: true},
	{argv: []string{"whereis"}, allowOperands: true},
	{argv: []string{"which"}, allowOperands: true},
	{argv: []string{"whoami"}, allowOperands: false},

	// Git — read-only porcelain.
	{argv: []string{"git", "blame"}, denyFlags: gitCodeExecFlags, allowOperands: true},
	{argv: []string{"git", "describe"}, denyFlags: gitCodeExecFlags, allowOperands: true},
	{argv: []string{"git", "diff"}, denyFlags: gitCodeExecFlags, allowOperands: true},
	{argv: []string{"git", "grep"}, denyFlags: gitCodeExecFlags, allowOperands: true},
	{argv: []string{"git", "log"}, denyFlags: gitCodeExecFlags, allowOperands: true},
	{argv: []string{"git", "ls-files"}, denyFlags: gitCodeExecFlags, allowOperands: true},
	{argv: []string{"git", "rev-parse"}, denyFlags: gitCodeExecFlags, allowOperands: true},
	{argv: []string{"git", "shortlog"}, denyFlags: gitCodeExecFlags, allowOperands: true},
	{argv: []string{"git", "show"}, denyFlags: gitCodeExecFlags, allowOperands: true},
	{argv: []string{"git", "status"}, denyFlags: gitCodeExecFlags, allowOperands: true},

	// Git — read-only forms of otherwise-mutating subcommands. These take
	// no operands: `git branch <name>` creates, `git tag <name>` creates,
	// and `git remote add|set-url|remove` all write to the config.
	{
		argv:          []string{"git", "branch"},
		restrictFlags: true,
		allowFlags: []string{
			"-l", "--list", "-a", "--all", "-r", "--remotes",
			"-v", "-vv", "--verbose", "--show-current",
			"--format", "--sort",
		},
		allowOperands: false,
	},
	{
		argv:          []string{"git", "tag"},
		restrictFlags: true,
		allowFlags: []string{
			"-l", "--list", "-n", "--contains", "--no-contains",
			"--merged", "--no-merged", "--points-at",
			"--format", "--sort",
		},
		allowOperands: false,
	},
	{
		argv:          []string{"git", "remote"},
		restrictFlags: true,
		allowFlags:    []string{"-v", "--verbose"},
		allowOperands: false,
	},
	// `git config --get <key>` reads one key; the bare `--get` prefix used
	// to also admit --get-urlmatch and friends, so the flag is matched
	// exactly here.
	{
		argv:          []string{"git", "config"},
		restrictFlags: true,
		allowFlags:    []string{"--get", "--get-all", "--list", "-l"},
		requireFlag:   true,
		allowOperands: true,
	},

	// `env` with no command to run just prints the environment. The
	// wrapper peel below only fires when there is an inner command, so
	// this entry is reached exactly when there is not.
	{argv: []string{"env"}, restrictFlags: true, allowOperands: false},
}

// commandWrapper describes a command that runs another command given to
// it. Auto-approving the wrapper itself would auto-approve anything it
// was handed, so instead the wrapper is peeled off and the command inside
// is checked against [safeCommands] on its own merits.
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
	// allowAssigns permits NAME=VALUE tokens before the inner command,
	// which only `env` accepts.
	allowAssigns bool
}

var commandWrappers = []commandWrapper{
	{name: "env", allowAssigns: true, valueFlags: []string{"-u", "--unset", "-C", "--chdir", "-S", "--split-string"}},
	{name: "nohup"},
	{name: "nice", valueFlags: []string{"-n", "--adjustment"}},
	{name: "timeout", skipOperands: 1, valueFlags: []string{"-s", "--signal", "-k", "--kill-after"}},
}

// maxWrapperDepth bounds how many nested wrappers are unwrapped, so a
// pathological `nohup nohup nohup …` cannot spin.
const maxWrapperDepth = 4

func init() {
	if runtime.GOOS == "windows" {
		safeCommands = append(
			safeCommands,
			// Windows-specific read-only commands. nslookup and ping are
			// deliberately excluded: they reach the network.
			safeCommand{argv: []string{"ipconfig"}, allowOperands: true},
			safeCommand{argv: []string{"systeminfo"}, allowOperands: true},
			safeCommand{argv: []string{"tasklist"}, allowOperands: true},
			safeCommand{argv: []string{"where"}, allowOperands: true},
		)
	}
}

// isSafeReadOnly reports whether command consists entirely of read-only
// commands that may run without a permission prompt.
//
// It parses the command rather than matching against its text. Matching
// on text cannot see the difference between `echo hi` and
// `echo pwned > ~/.bashrc`, treats a newline as ordinary whitespace, and
// has no way to tell `git branch` from `git branch -D main`.
//
// The analysis fails closed at every step. A parse error, an unrecognized
// node type, a redirection, a background or negated statement, a variable
// assignment, a word that is not fully literal, or an argument that does
// not match a [safeCommand] entry all result in false — which costs the
// user a permission prompt and nothing more.
func isSafeReadOnly(command string) bool {
	file, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return false
	}
	if len(file.Stmts) == 0 {
		return false
	}
	for _, stmt := range file.Stmts {
		if !safeStmt(stmt) {
			return false
		}
	}
	return true
}

// safeStmt reports whether a single statement is read-only.
//
// Only a bare call expression qualifies. Pipelines, lists, subshells,
// conditionals, loops and function definitions are all rejected: each
// either composes commands in ways this analysis does not model, or (in
// the case of a pipeline into a non-listed command) has no benefit worth
// the added surface.
func safeStmt(stmt *syntax.Stmt) bool {
	if stmt == nil || stmt.Cmd == nil {
		return false
	}
	// A redirection turns a read-only command into a file write, and
	// backgrounding detaches it from the run we are about to observe.
	if len(stmt.Redirs) > 0 || stmt.Background || stmt.Coprocess || stmt.Negated || stmt.Disown {
		return false
	}
	// `time` is a shell keyword rather than a command, so it arrives as
	// its own node. It only measures what it wraps, so defer to that.
	if clause, ok := stmt.Cmd.(*syntax.TimeClause); ok {
		return clause.Stmt != nil && safeStmt(clause.Stmt)
	}
	call, ok := stmt.Cmd.(*syntax.CallExpr)
	if !ok {
		return false
	}
	// `FOO=bar cmd` can steer the command through its environment
	// (PATH, LD_PRELOAD, GIT_*), so it never auto-approves.
	if len(call.Assigns) > 0 || len(call.Args) == 0 {
		return false
	}
	argv, ok := literalArgs(call.Args)
	if !ok {
		return false
	}
	return safeArgv(argv, 0)
}

// safeArgv reports whether a fully-literal argv is a safe command,
// unwrapping command wrappers up to maxWrapperDepth.
func safeArgv(argv []string, depth int) bool {
	if len(argv) == 0 {
		return false
	}
	if inner, ok := peelWrapper(argv); ok {
		if depth >= maxWrapperDepth {
			return false
		}
		return safeArgv(inner, depth+1)
	}
	return slices.ContainsFunc(safeCommands, func(sc safeCommand) bool {
		return sc.matches(argv)
	})
}

// peelWrapper strips a leading command wrapper and returns the command it
// would run. The second result is false when argv is not a wrapper, or is
// a wrapper with no inner command — `env` alone just prints the
// environment, so it falls through to ordinary matching.
func peelWrapper(argv []string) ([]string, bool) {
	idx := slices.IndexFunc(commandWrappers, func(w commandWrapper) bool {
		return w.name == argv[0]
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
		case w.allowAssigns && isAssignment(tok):
			rest = rest[1:]
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

// matches reports whether argv is an instance of this safe command form.
func (sc safeCommand) matches(argv []string) bool {
	if len(argv) < len(sc.argv) || !slices.Equal(argv[:len(sc.argv)], sc.argv) {
		return false
	}
	rest := argv[len(sc.argv):]
	operandsOnly := false
	sawFlag := false
	for _, arg := range rest {
		if arg == "--" {
			operandsOnly = true
			continue
		}
		if !operandsOnly && isFlag(arg) {
			name := flagName(arg)
			if sc.restrictFlags {
				if !slices.Contains(sc.allowFlags, name) {
					return false
				}
			} else if slices.Contains(sc.denyFlags, name) {
				return false
			}
			sawFlag = true
			continue
		}
		if !sc.allowOperands {
			return false
		}
	}
	return sawFlag || !sc.requireFlag
}

// isFlag reports whether a token is a flag rather than an operand. A lone
// "-" is conventionally stdin, and a lone "--" is a separator; neither is
// a flag.
func isFlag(tok string) bool {
	return len(tok) > 1 && strings.HasPrefix(tok, "-") && tok != "--"
}

// flagName strips any =value suffix so `--format=%H` is matched by an
// entry of "--format".
func flagName(tok string) string {
	name, _, _ := strings.Cut(tok, "=")
	return name
}

// isAssignment reports whether a token is a NAME=VALUE environment
// assignment of the form `env` accepts before its command.
func isAssignment(tok string) bool {
	name, _, ok := strings.Cut(tok, "=")
	if !ok || name == "" {
		return false
	}
	for i, r := range name {
		isAlpha := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if !isAlpha && !(isDigit && i > 0) {
			return false
		}
	}
	return true
}

// literalArgs converts parsed words to plain strings, reporting false if
// any word is not entirely literal.
func literalArgs(words []*syntax.Word) ([]string, bool) {
	out := make([]string, 0, len(words))
	for _, word := range words {
		lit, ok := literalWord(word)
		if !ok {
			return nil, false
		}
		out = append(out, lit)
	}
	return out, true
}

// literalWord flattens a word to its literal text, reporting false if any
// part of it is resolved at runtime.
//
// Command substitution, parameter expansion, arithmetic expansion and
// process substitution are all rejected rather than evaluated: their
// value is not knowable here, and `ls $(rm -rf /)` must never be treated
// as an `ls`. Glob characters are left alone — they are expanded by the
// shell against the filesystem and cannot introduce a new command.
func literalWord(word *syntax.Word) (string, bool) {
	var sb strings.Builder
	for _, part := range word.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			sb.WriteString(p.Value)
		case *syntax.SglQuoted:
			// $'…' applies escape sequences, so Value is not the final
			// text; only plain '…' is taken at face value.
			if p.Dollar {
				return "", false
			}
			sb.WriteString(p.Value)
		case *syntax.DblQuoted:
			if p.Dollar {
				return "", false
			}
			for _, inner := range p.Parts {
				lit, ok := inner.(*syntax.Lit)
				if !ok {
					return "", false
				}
				sb.WriteString(lit.Value)
			}
		default:
			return "", false
		}
	}
	return sb.String(), true
}
