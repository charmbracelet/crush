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
	"git config --get",
	"git config --list",
	"git describe",
	"git diff",
	"git grep",
	"git log",
	"git ls-files",
	"git ls-remote",
	"git rev-parse",
	"git shortlog",
	"git show",
	"git status",
}

var safeGitQueryCommands = []string{
	"git branch",
	"git branch --show-current",
	"git remote",
	"git remote --verbose",
	"git remote -v",
	"git tag",
}

var safeGitQueryPrefixes = []string{
	"git branch --all",
	"git branch --list",
	"git branch --remotes",
	"git branch -a",
	"git branch -l",
	"git branch -r",
	"git branch -v",
	"git branch -vv",
	"git remote get-url",
	"git remote show",
	"git tag --list",
	"git tag -l",
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

func isSafeReadOnlyCommand(command string) bool {
	if containsCommandChaining(command) {
		return false
	}

	command = strings.ToLower(command)
	for _, safe := range safeCommands {
		if hasCommandPrefix(command, safe) {
			return true
		}
	}

	if slices.Contains(safeGitQueryCommands, command) {
		return true
	}

	for _, safe := range safeGitQueryPrefixes {
		if (command == safe || strings.HasPrefix(command, safe+" ")) && isSafeGitQuery(command) {
			return true
		}
	}

	return false
}

func isSafeGitQuery(command string) bool {
	if strings.Contains(command, "$") {
		return false
	}

	args := strings.Fields(command)
	if len(args) < 2 {
		return false
	}

	for _, arg := range args[2:] {
		switch args[1] {
		case "branch":
			if isMutatingGitBranchOption(arg) {
				return false
			}
		case "tag":
			if isMutatingGitTagOption(arg) {
				return false
			}
		}
	}

	return true
}

func isMutatingGitBranchOption(arg string) bool {
	if strings.HasPrefix(arg, "--") {
		option, _, _ := strings.Cut(arg, "=")
		return slices.Contains([]string{
			"--copy",
			"--create-reflog",
			"--delete",
			"--edit-description",
			"--force",
			"--move",
			"--no-track",
			"--recurse-submodules",
			"--set-upstream-to",
			"--track",
			"--unset-upstream",
		}, option)
	}

	return strings.HasPrefix(arg, "-") && strings.ContainsAny(arg[1:], "cCdDfmM")
}

func isMutatingGitTagOption(arg string) bool {
	if strings.HasPrefix(arg, "--") {
		option, _, _ := strings.Cut(arg, "=")
		return slices.Contains([]string{
			"--annotate",
			"--delete",
			"--edit",
			"--file",
			"--force",
			"--local-user",
			"--message",
			"--sign",
		}, option)
	}

	return strings.HasPrefix(arg, "-") && strings.ContainsAny(arg[1:], "aDefFmsu")
}

func hasCommandPrefix(command, prefix string) bool {
	return strings.HasPrefix(command, prefix) &&
		(len(command) == len(prefix) || command[len(prefix)] == ' ' || command[len(prefix)] == '-')
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
