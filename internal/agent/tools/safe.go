package tools

import (
	"runtime"
	"slices"
	"strings"
)

var safeCommands = []string{
	// Bash builtins and core utils
	"cal",
	"date",
	"df",
	"du",
	"echo",
	"env",
	"free",
	"groups",
	"hostname",
	"id",
	"kill",
	"killall",
	"ls",
	"nice",
	"nohup",
	"printenv",
	"ps",
	"pwd",
	"set",
	"time",
	"timeout",
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

	// GitHub CLI — read-only queries only. `gh` is otherwise entirely absent
	// from the safe list, so even `gh pr view` hits the permission gate and
	// stalls non-interactive sessions waiting for an answer that never comes.
	// Mutating subcommands (pr create, release create, api POST, …) are NOT
	// listed, so they still require explicit approval.
	"gh auth status",
	"gh alias list",
	"gh cache list",
	"gh codespace list",
	"gh extension list",
	"gh gist list",
	"gh issue list",
	"gh issue view",
	"gh label list",
	"gh pr diff",
	"gh pr list",
	"gh pr view",
	"gh repo list",
	"gh repo view",
	"gh run list",
	"gh run view",
	"gh search",
	"gh secret list",
	"gh ssh-key list",
	"gh variable list",
	"gh workflow list",

	// Homebrew — read-only inspections only. `brew install` stays blocked by
	// its ArgumentsBlocker; these merely let an agent look before it asks.
	"brew --version",
	"brew config",
	"brew deps",
	"brew doctor",
	"brew info",
	"brew leaves",
	"brew list",
	"brew outdated",
	"brew search",
	"brew services list",
	"brew tap",
	"brew uses",
}

var chainingMetacharacters = []string{
	";",
	"|",
	"&&",
	"$(",
	"`",
}

// containsCommandChaining reports whether s contains shell metacharacters
// that enable command chaining or substitution.
func containsCommandChaining(s string) bool {
	return slices.ContainsFunc(chainingMetacharacters, func(c string) bool {
		return strings.Contains(s, c)
	})
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
