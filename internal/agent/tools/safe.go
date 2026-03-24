package tools

import (
	"bytes"
	"runtime"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

var safeDirectCommands = map[string]struct{}{
	// Bash builtins and core utils.
	"cal":      {},
	"date":     {},
	"df":       {},
	"du":       {},
	"echo":     {},
	"free":     {},
	"groups":   {},
	"hostname": {},
	"id":       {},
	"ls":       {},
	"printenv": {},
	"ps":       {},
	"pwd":      {},
	"top":      {},
	"uname":    {},
	"uptime":   {},
	"whatis":   {},
	"whereis":  {},
	"which":    {},
	"whoami":   {},
}

func init() {
	if runtime.GOOS == "windows" {
		safeDirectCommands["ipconfig"] = struct{}{}
		safeDirectCommands["nslookup"] = struct{}{}
		safeDirectCommands["ping"] = struct{}{}
		safeDirectCommands["systeminfo"] = struct{}{}
		safeDirectCommands["tasklist"] = struct{}{}
		safeDirectCommands["where"] = struct{}{}
	}
}

func isSafeReadOnlyCommand(command string) bool {
	file, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil || len(file.Stmts) != 1 {
		return false
	}

	stmt := file.Stmts[0]
	if stmt.Background || stmt.Coprocess || stmt.Negated || len(stmt.Redirs) > 0 {
		return false
	}

	call, ok := stmt.Cmd.(*syntax.CallExpr)
	if !ok || len(call.Assigns) > 0 || len(call.Args) == 0 {
		return false
	}

	args := make([]string, 0, len(call.Args))
	for _, arg := range call.Args {
		value, ok := literalWord(arg)
		if !ok {
			return false
		}
		args = append(args, value)
	}

	if _, ok := safeDirectCommands[args[0]]; ok {
		return true
	}

	if args[0] == "git" {
		return isSafeGitCommand(args)
	}

	return false
}

func isSafeGitCommand(args []string) bool {
	if len(args) < 2 {
		return false
	}

	subcommand := args[1]
	rest := args[2:]
	switch subcommand {
	case "blame", "describe", "grep", "ls-files", "rev-parse", "show", "status":
		return true
	case "diff":
		return safeGitDiffArgs(rest)
	case "log", "shortlog":
		return safeGitLogArgs(rest)
	case "config":
		return slicesEqual(rest, []string{"--list"}) || (len(rest) >= 2 && rest[0] == "--get")
	case "branch":
		return len(rest) == 0 || safeFlagList(rest, map[string]struct{}{"--list": {}, "--show-current": {}, "-a": {}, "-r": {}})
	case "remote":
		return len(rest) == 0 || (len(rest) == 1 && (rest[0] == "-v" || rest[0] == "show"))
	case "tag":
		return len(rest) == 0 || safeFlagList(rest, map[string]struct{}{"-l": {}, "--list": {}})
	default:
		return false
	}
}

func safeGitDiffArgs(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--output") {
			return false
		}
	}
	return true
}

func safeGitLogArgs(args []string) bool {
	allowedFlags := map[string]struct{}{
		"--oneline":     {},
		"--stat":        {},
		"--graph":       {},
		"--decorate":    {},
		"--patch":       {},
		"-p":            {},
		"--name-only":   {},
		"--name-status": {},
		"--no-merges":   {},
		"--reverse":     {},
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-n" || arg == "--max-count":
			if i+1 >= len(args) || !isPositiveInt(args[i+1]) {
				return false
			}
			i++
		case strings.HasPrefix(arg, "--max-count="):
			if !isPositiveInt(strings.TrimPrefix(arg, "--max-count=")) {
				return false
			}
		case strings.HasPrefix(arg, "-"):
			if _, ok := allowedFlags[arg]; !ok {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func safeFlagList(args []string, allowed map[string]struct{}) bool {
	for _, arg := range args {
		if _, ok := allowed[arg]; !ok {
			return false
		}
	}
	return true
}

func isPositiveInt(value string) bool {
	n, err := strconv.Atoi(value)
	return err == nil && n > 0
}

func literalWord(word *syntax.Word) (string, bool) {
	if len(word.Parts) == 0 {
		return "", false
	}

	var buf bytes.Buffer
	for _, part := range word.Parts {
		lit, ok := part.(*syntax.Lit)
		if !ok {
			return "", false
		}
		buf.WriteString(lit.Value)
	}
	return buf.String(), true
}

func slicesEqual(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
