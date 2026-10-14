package tools

import (
	"runtime"
	"slices"
	"strings"
)

// safeCommands skip the permission prompt. Never list a command that runs
// another command (env, nice, nohup, time, timeout): the prefix match would
// auto-approve whatever it wraps.
var safeCommands = []string{
	// Bash builtins and core utils
	"cal",
	"date",
	"df",
	"du",
	"echo",
	"free",
	"groups",
	"hostname",
	"id",
	"ls",
	"printenv",
	"ps",
	"pwd",
	"set",
	"top",
	"type",
	"uname",
	"unset",
	"uptime",
	"whatis",
	"whereis",
	"which",
	"whoami",

	// Git
	"git blame",
	"git branch",
	"git config --get",
	"git config --list",
	"git describe",
	"git diff",
	"git grep",
	"git log",
	"git ls-files",
	"git ls-remote",
	"git remote",
	"git rev-parse",
	"git shortlog",
	"git show",
	"git status",
	"git tag",
}

var chainingMetacharacters = []string{
	";",
	"|",
	"&", // Also covers && and &>.
	"$(",
	"`",
	">",
	"<",
	"\n",
	"\r",
}

// containsCommandChaining reports whether s contains shell metacharacters
// that enable command chaining or substitution.
func containsCommandChaining(s string) bool {
	return slices.ContainsFunc(chainingMetacharacters, func(c string) bool {
		return strings.Contains(s, c)
	})
}

// isSafeCommand reports whether cmd may skip the permission prompt: no
// chaining, and it starts with a safeCommands entry (whole word or word-flag).
func isSafeCommand(cmd string) bool {
	if containsCommandChaining(cmd) {
		return false
	}
	lower := strings.ToLower(cmd)
	for _, safe := range safeCommands {
		if !strings.HasPrefix(lower, safe) {
			continue
		}
		// "git <verb>-<x>" is an alias or a git-<verb>-<x> program, not the
		// verb: after a git verb only a flag entry (git config --get) may go
		// on with "-".
		gitVerb := strings.HasPrefix(safe, "git ") && !strings.Contains(safe, " -")
		if n := len(safe); n != len(lower) && lower[n] != ' ' && (lower[n] != '-' || gitVerb) {
			continue
		}
		return !strings.HasPrefix(safe, "git ") ||
			isReadOnlyGit(safe, lower) && !slices.ContainsFunc(strings.Fields(cmd), gitArgRunsProgram)
	}
	return false
}

// gitProgramOptions name a program for git to run (pager, upload-pack) or
// let it run a configured diff/textconv driver. Top-level -c, --config-env,
// --exec-path and GIT_* env prefixes need no entry: they come before the
// verb, so the safeCommands prefix never matches them.
var gitProgramOptions = []string{"--open-files-in-pager", "--upload-pack", "--exec", "--ext-diff", "--textconv"}

// gitArgRunsProgram reports whether a git argument can make git run another
// program: grep's -O[<pager>], also bundled (-iO), or a gitProgramOptions
// entry or any prefix of one (git accepts unique abbreviations, e.g.
// --open=true). Fails closed: --text, a prefix of --textconv, prompts too.
func gitArgRunsProgram(arg string) bool {
	if !strings.HasPrefix(arg, "--") {
		return strings.HasPrefix(arg, "-") && strings.Contains(arg, "O")
	}
	name, _, _ := strings.Cut(strings.ToLower(arg), "=")
	return len(name) > 2 && slices.ContainsFunc(gitProgramOptions, func(o string) bool { return strings.HasPrefix(o, name) })
}

// gitReadOnlyArgs: for git verbs that also mutate, the only auto-approved
// argument lists (every arg must be listed; anything else prompts).
var gitReadOnlyArgs = map[string][]string{
	"git branch": {"-a", "-r", "--list", "-v"},
	"git tag":    {"-l", "--list"},
	"git remote": {"-v"},
}

// isReadOnlyGit vets a git command whose verb matched safe. It fails closed:
// "--output" writes a file (git rejects abbreviations of it), and quoting,
// expansion or globs could build that word, so they prompt too.
func isReadOnlyGit(safe, lower string) bool {
	if strings.ContainsAny(lower, `'"\${*?[`) || strings.Contains(lower, "--output") {
		return false
	}
	allowed, restricted := gitReadOnlyArgs[safe]
	if !restricted {
		return true
	}
	fields := strings.Fields(lower)
	if len(fields) < 2 || fields[0]+" "+fields[1] != safe {
		return false
	}
	args := fields[2:]
	if safe == "git remote" && len(args) > 0 && args[0] == "show" {
		return true
	}
	return !slices.ContainsFunc(args, func(a string) bool { return !slices.Contains(allowed, a) })
}

func init() {
	if runtime.GOOS == "windows" {
		safeCommands = append(
			safeCommands,
			// Windows-specific commands
			"ipconfig",
			"nslookup",
			"ping",
			"systeminfo",
			"tasklist",
			"where",
		)
	}
}
