package tools

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/shell"
	"mvdan.cc/sh/v3/syntax"
)

type BashToolOptions struct {
	RestrictedToGitReadOnly bool
	DisableBackground       bool
	DescriptionOverride     string
}

var restrictedGitGlobalFlags = map[string]bool{
	"-C":                  true,
	"--git-dir":           true,
	"--no-optional-locks": false,
	"--no-pager":          false,
	"--work-tree":         true,
}

var restrictedGitSubcommands = map[string]struct{}{
	"blame":      {},
	"describe":   {},
	"diff":       {},
	"grep":       {},
	"log":        {},
	"ls-files":   {},
	"merge-base": {},
	"rev-parse":  {},
	"show":       {},
	"status":     {},
}

var restrictedGitBlockedFlags = map[string]struct{}{
	"--output": {},
}

func restrictedGitBashDescription() string {
	return `Executes a restricted shell for local git inspection only.

Allowed usage:
- Exactly one direct git command per call
- Read-only subcommands only: git blame, git describe, git diff, git grep, git log, git ls-files, git merge-base, git rev-parse, git show, git status
- Optional global git flags: -C, --git-dir, --work-tree, --no-pager, --no-optional-locks

Blocked usage:
- Any non-git command or wrapper shell such as bash -lc, sh -c, cmd /c, or powershell -c
- Any mutating git command such as checkout, switch, restore, reset, clean, stash, commit, merge, rebase, cherry-pick, revert, apply, push, pull, or fetch
- Multiple commands, pipes, redirects, shell assignments, substitutions, or background execution

Use this tool only for inspecting repository state and history.`
}

func RestrictedGitBashDescription() string {
	return restrictedGitBashDescription()
}

func restrictedGitBlockFunc() shell.BlockFunc {
	return func(args []string) bool {
		return validateRestrictedGitArgs(args) != nil
	}
}

func validateRestrictedGitCommand(command string) error {
	file, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return fmt.Errorf("restricted git bash requires a valid shell command: %w", err)
	}

	if len(file.Stmts) != 1 {
		return fmt.Errorf("restricted git bash only allows one git command per call")
	}

	stmt := file.Stmts[0]
	if stmt == nil {
		return fmt.Errorf("restricted git bash requires a git command")
	}
	if stmt.Negated || stmt.Background || stmt.Coprocess || stmt.Disown {
		return fmt.Errorf("restricted git bash does not allow shell control operators")
	}
	if len(stmt.Redirs) > 0 {
		return fmt.Errorf("restricted git bash does not allow redirection")
	}

	call, ok := stmt.Cmd.(*syntax.CallExpr)
	if !ok {
		return fmt.Errorf("restricted git bash only allows a direct git command")
	}
	if len(call.Assigns) > 0 {
		return fmt.Errorf("restricted git bash does not allow shell assignments")
	}

	args := make([]string, 0, len(call.Args))
	for _, word := range call.Args {
		arg, ok := literalWord(word)
		if !ok {
			return fmt.Errorf("restricted git bash does not allow shell expansions or substitutions")
		}
		args = append(args, arg)
	}

	return validateRestrictedGitArgs(args)
}

func validateRestrictedGitArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("restricted git bash requires a git command")
	}
	if args[0] != "git" {
		return fmt.Errorf("restricted git bash only allows direct git commands")
	}

	i := 1
	for i < len(args) {
		arg := args[i]
		if arg == "" || arg == "-" || !strings.HasPrefix(arg, "-") {
			break
		}

		flag, _, hasInlineValue := strings.Cut(arg, "=")
		takesValue, ok := restrictedGitGlobalFlags[flag]
		if !ok {
			break
		}

		if takesValue && !hasInlineValue {
			i++
			if i >= len(args) {
				return fmt.Errorf("restricted git bash requires a value after %s", flag)
			}
		}
		i++
	}

	if i >= len(args) {
		return fmt.Errorf("restricted git bash requires a git subcommand")
	}

	subcommand := args[i]
	if strings.HasPrefix(subcommand, "-") {
		return fmt.Errorf("restricted git bash does not allow git option %q", subcommand)
	}
	if _, ok := restrictedGitSubcommands[subcommand]; !ok {
		return fmt.Errorf("restricted git bash does not allow git %s", subcommand)
	}

	for _, arg := range args[i+1:] {
		if arg == "--" {
			break
		}
		if arg == "" || arg == "-" || !strings.HasPrefix(arg, "-") {
			continue
		}

		flag, _, _ := strings.Cut(arg, "=")
		for blockedFlag := range restrictedGitBlockedFlags {
			if strings.HasPrefix(blockedFlag, flag) {
				return fmt.Errorf("restricted git bash does not allow git option %q", flag)
			}
		}
	}

	return nil
}

func literalWord(word *syntax.Word) (string, bool) {
	if word == nil {
		return "", false
	}
	return literalWordParts(word.Parts)
}

func literalWordParts(parts []syntax.WordPart) (string, bool) {
	var b strings.Builder
	for _, part := range parts {
		switch x := part.(type) {
		case *syntax.Lit:
			b.WriteString(x.Value)
		case *syntax.SglQuoted:
			b.WriteString(x.Value)
		case *syntax.DblQuoted:
			value, ok := literalWordParts(x.Parts)
			if !ok {
				return "", false
			}
			b.WriteString(value)
		default:
			return "", false
		}
	}
	return b.String(), true
}
