package tools

import (
	"runtime"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/shell"
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
//
// --textconv sits here for the same reason as --ext-diff: both run a
// program named by the repository's own config, so a repository the user
// cloned but does not control chooses what executes.
var gitCodeExecFlags = []string{
	"--ext-diff",
	"--textconv",
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
//     to them. They are resolved by shell.ResolveArgv instead, which peels
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
	// hostname's flags are enumerated rather than denied, because the ones
	// that write take their value attached to the flag: `hostname -F/etc/x`
	// sets the hostname from a file in a single token, so there is no
	// operand for allowOperands to reject. An allow list fails closed on
	// the whole shape, including clustered forms.
	{
		argv:          []string{"hostname"},
		restrictFlags: true,
		allowFlags: []string{
			"-a", "--alias", "-A", "--all-fqdns", "-d", "--domain",
			"-f", "--fqdn", "--long", "-i", "--ip-address",
			"-I", "--all-ip-addresses", "-s", "--short",
			"-y", "--yp", "--nis",
		},
		allowOperands: false,
	},
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
	// The filter flags above take a commit-ish operand. Requiring one of
	// them to be present is what keeps `git branch <name>` (creates) and
	// `git tag <name>` (creates) out, since those carry an operand and no
	// flag.
	{
		argv:          []string{"git", "branch"},
		restrictFlags: true,
		allowFlags:    []string{"--contains", "--no-contains", "--merged", "--no-merged", "--points-at"},
		requireFlag:   true,
		allowOperands: true,
	},
	{
		argv:          []string{"git", "tag"},
		restrictFlags: true,
		allowFlags:    []string{"--contains", "--no-contains", "--merged", "--no-merged", "--points-at"},
		requireFlag:   true,
		allowOperands: true,
	},
	{
		argv:          []string{"git", "remote"},
		restrictFlags: true,
		allowFlags:    []string{"-v", "--verbose"},
		allowOperands: false,
	},
	// Read-only remote subcommands, spelled out so the mutating siblings
	// (add, remove, rename, set-url, prune) stay off the list.
	//
	// `git remote show` queries the remote over the network unless -n is
	// given, so -n is required here — otherwise this entry would admit
	// exactly the network access that keeps `git ls-remote` off the list.
	{
		argv:          []string{"git", "remote", "show"},
		restrictFlags: true,
		allowFlags:    []string{"-n"},
		requireFlag:   true,
		allowOperands: true,
	},
	{argv: []string{"git", "remote", "get-url"}, allowOperands: true},
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
func isSafeReadOnly(command string, extra []safeCommand) bool {
	file, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return false
	}
	if len(file.Stmts) == 0 {
		return false
	}
	return safeStmts(file.Stmts, extra)
}

// safeStmts reports whether every statement in a list is safe. An empty list
// is not: a command that resolves to nothing is not something to wave
// through, and it is the shape a parse surprise tends to take.
func safeStmts(stmts []*syntax.Stmt, extra []safeCommand) bool {
	if len(stmts) == 0 {
		return false
	}
	for _, stmt := range stmts {
		if !safeStmt(stmt, extra) {
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
func safeStmt(stmt *syntax.Stmt, extra []safeCommand) bool {
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
		return clause.Stmt != nil && safeStmt(clause.Stmt, extra)
	}
	// Composing read-only commands leaves them read-only, so a chain is
	// judged by its parts: `&&`, `||`, and `|` are safe exactly when both
	// sides are. Each part keeps its own fixed argv, which is what makes
	// this sound — unlike command substitution, where one command's output
	// becomes another's arguments and could supply a flag that turns a
	// read-only command into a writing one. Substitution is still refused
	// by literalArgs below.
	if bin, ok := stmt.Cmd.(*syntax.BinaryCmd); ok {
		switch bin.Op {
		case syntax.AndStmt, syntax.OrStmt, syntax.Pipe, syntax.PipeAll:
			return safeStmt(bin.X, extra) && safeStmt(bin.Y, extra)
		default:
			return false
		}
	}
	// A subshell changes where the commands run, not what they may do.
	if sub, ok := stmt.Cmd.(*syntax.Subshell); ok {
		return safeStmts(sub.Stmts, extra)
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
	return safeArgv(argv, extra)
}

// safeArgv reports whether a fully-literal argv is a safe command. The argv
// is first resolved through any command wrapper, with the same peeling the
// deny list applies, so the two lists cannot disagree about which command is
// being run.
func safeArgv(argv []string, extra []safeCommand) bool {
	argv = shell.ResolveArgv(argv)
	if len(argv) == 0 {
		return false
	}
	match := func(sc safeCommand) bool { return sc.matches(argv) }
	return slices.ContainsFunc(safeCommands, match) || slices.ContainsFunc(extra, match)
}

// userSafeCommands turns configured safe command lines into entries.
//
// Each entry matches only the exact tokens it was given: no operands and no
// flags beyond them, so "go build" covers `go build` and not `go build -o
// /usr/local/bin/thing`. That is deliberately stricter than the built-in
// entries, which carry a hand-checked flag policy per command. A setting that
// widened itself to cover flags nobody reviewed would be a way to hand over
// far more than the reader thought they were granting.
func userSafeCommands(perms *config.Permissions) []safeCommand {
	if perms == nil {
		return nil
	}
	out := make([]safeCommand, 0, len(perms.SafeCommands))
	for _, entry := range perms.SafeCommands {
		argv := strings.Fields(entry)
		if len(argv) == 0 {
			continue
		}
		// A wrapper in front would let the entry stand in for whatever the
		// wrapper was handed, so resolve it the way every other list does
		// and record what actually runs.
		argv = shell.ResolveArgv(argv)
		argv[0] = shell.NormalizeCommandName(argv[0])
		out = append(out, safeCommand{
			argv:          argv,
			restrictFlags: true, // allowFlags is empty, so any flag fails.
		})
	}
	return out
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
			if sc.restrictFlags {
				// Matched exactly: an unrecognized spelling, including a
				// cluster of otherwise-allowed short flags, fails closed.
				if !slices.Contains(sc.allowFlags, flagName(arg)) {
					return false
				}
			} else if flagDenied(arg, sc.denyFlags) {
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

// flagName strips any =value suffix from a flag token.
func flagName(tok string) string {
	name, _, _ := strings.Cut(tok, "=")
	return name
}

// flagDenied reports whether tok is a denied flag, or carries one.
//
// A long flag matches by name, so `--output=x` is caught by an entry of
// "--output". A short flag needs more than that: getopt accepts its value
// attached (`date -s2020-01-01`) and accepts clusters (`date -us`), so
// neither spelling is equal to "-s". Every character of a single-dash
// token is therefore checked against the deny set.
func flagDenied(tok string, deny []string) bool {
	if slices.Contains(deny, flagName(tok)) {
		return true
	}
	if strings.HasPrefix(tok, "--") {
		return false
	}
	for _, r := range tok[1:] {
		if slices.Contains(deny, "-"+string(r)) {
			return true
		}
	}
	return false
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
//
// A literal containing a backslash is also rejected. syntax.Lit.Value keeps
// unquoted escape sequences verbatim, so the text here is not the argv the
// shell will pass: `--ext\-diff` reads as the denied flag `--ext-diff` only
// after the shell strips the backslash, which would let a protected flag
// slip past denyFlags. Failing closed on any backslash is cheaper than
// re-implementing the shell's escape rules.
func literalWord(word *syntax.Word) (string, bool) {
	var sb strings.Builder
	for _, part := range word.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			if strings.Contains(p.Value, `\`) {
				return "", false
			}
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
				// Double-quoted literals keep backslash escapes verbatim
				// too; reject them for the same reason as bare literals.
				if strings.Contains(lit.Value, `\`) {
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
