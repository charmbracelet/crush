package tools

import (
	"os/user"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// MakeCommandToken constructs a normalised command: context token from a
// command string. cmd may be a bare command name ("go") or include a
// subcommand ("go test"). Surrounding and internal whitespace is normalised
// so callers constructing tokens from config entries and callers constructing
// tokens from parsed AST nodes always agree on the token string.
func MakeCommandToken(cmd string) string {
	return "command:" + strings.Join(strings.Fields(cmd), " ")
}

// MakePathToken constructs a path: context token from an absolute,
// already-cleaned path.
func MakePathToken(path string) string {
	return "path:" + filepath.Clean(path)
}

// AnalyzeCommand parses a shell command string and returns context tokens
// for use with the permission system. It walks the AST to extract command
// names (including subcommands) and path arguments, resolving relative paths
// against the working directory.
//
// Fail-closed semantics: for unsafe constructs (command substitution,
// backticks, redirection, process substitution, eval, sh -c, function
// declarations, grouping, loops, conditionals, chains), the parser emits a
// single opaque token "command!:<original command>" instead of individual
// command/path tokens. This ensures the request still carries a context and
// is never silently auto-approved.
//
// Safe constructs: simple commands, and pipelines (|), and command chains
// (&&, ||, ;) produce command:path tokens for each constituent command.
func AnalyzeCommand(cmd, workingDir string) []string {
	if cmd == "" || strings.TrimSpace(cmd) == "" {
		return []string{MakeCommandToken(strings.TrimSpace(cmd))}
	}
	parsed, err := syntax.NewParser().Parse(strings.NewReader(cmd), "")
	if err != nil {
		return opaqueTokens(cmd)
	}

	if !hasUnsafeConstructs(parsed) {
		return extractTokens(cmd, parsed, workingDir)
	}
	return opaqueTokens(cmd)
}

// opaqueTokens returns an opaque context token for the entire command.
// This is used when the command contains unsafe constructs that the parser
// cannot model as individual contexts.
func opaqueTokens(cmd string) []string {
	return []string{"command!:" + strings.TrimSpace(cmd)}
}

// hasUnsafeConstructs reports whether the AST contains constructs that the
// parser cannot safely model as individual command/path contexts.
//
// The spec (§3.4, §3.5) explicitly lists these as "fail-closed":
//   - Command substitution: $(…), backticks
//   - Process substitution: <(cmd), >(cmd)
//   - Redirection: >, <, >>, 2>&1
//   - Shell execution via flags: sh -c, bash -c
//   - Eval
//   - Function declarations
//   - Grouping: (cmd), { cmd; }
//   - Loops and conditionals: for, while, if, case
//
// Chains (&&, ||, ;) and pipelines (|) are safe — they produce the same
// effect as separate commands and the spec (§4 §3.4) requires emitting
// command: tokens for each command in a chain.
func hasUnsafeConstructs(n syntax.Node) bool {
	hasUnsafe := false
	syntax.Walk(n, func(cn syntax.Node) bool {
		if hasUnsafe {
			return false
		}
		switch cn.(type) {
		case *syntax.CmdSubst:
			// $() command substitution (backticks are folded here).
			hasUnsafe = true
		case *syntax.Redirect:
			// >, <, >>, 2>&1 etc.
			hasUnsafe = true
		case *syntax.Subshell, *syntax.Block:
			// (cmd) and { cmd; } — grouping, fail closed.
			hasUnsafe = true
		case *syntax.FuncDecl:
			// Function declarations.
			hasUnsafe = true
		case *syntax.IfClause, *syntax.WhileClause, *syntax.ForClause, *syntax.CaseClause:
			// Conditionals and loops — fail closed.
			hasUnsafe = true
		case *syntax.CallExpr:
			// Detect sh -c / bash -c and eval.
			ce := cn.(*syntax.CallExpr)
			if len(ce.Args) > 0 {
				first := cmdText(ce.Args[0])
				if first == "eval" {
					hasUnsafe = true
				} else if first == "sh" || first == "bash" || first == "zsh" {
					// Check for -c flag.
					for _, a := range ce.Args[1:] {
						if isSafeWord(a) && cmdText(a) == "-c" {
							hasUnsafe = true
							break
						}
					}
				}
			}
		}
		// Check for ProcSubst inside Word nodes: <(ls) or >(ls).
		if w, ok := cn.(*syntax.Word); ok {
			for _, part := range w.Parts {
				if _, ok := part.(*syntax.ProcSubst); ok {
					hasUnsafe = true
					return false
				}
			}
		}
		return true
	})
	return hasUnsafe
}

// extractTokens walks the safe AST and extracts command and path tokens.
func extractTokens(cmd string, n syntax.Node, workingDir string) []string {
	var commands []string
	var paths []string

	syntax.Walk(n, func(cn syntax.Node) bool {
		ce, ok := cn.(*syntax.CallExpr)
		if !ok {
			return true
		}

		// Extract command name (first argument).
		if len(ce.Args) > 0 && isSafeWord(ce.Args[0]) {
			cmdName := cmdText(ce.Args[0])
			hasSub := false
			// Check if second word is a subcommand.
			if len(ce.Args) > 1 && isSafeWord(ce.Args[1]) {
				sub := cmdText(ce.Args[1])
				if sub != "" && isSubcommand(sub) {
					commands = append(commands, MakeCommandToken(cmdName+" "+sub))
					hasSub = true
				}
			}
			if cmdName != "" && !hasSub {
				commands = append(commands, MakeCommandToken(cmdName))
			}
		}

		// Extract path arguments (all arguments after the first).
		for _, w := range ce.Args[1:] {
			if !isSafeWord(w) {
				continue
			}
			wordStr := cmdText(w)
			if looksLikePath(wordStr) {
				if p := cleanPath(wordStr, workingDir); p != "" {
					paths = append(paths, MakePathToken(p))
				}
			}
		}
		return true
	})

	if len(commands) == 0 {
		return opaqueTokens(cmd)
	}

	// Deduplicate.
	seen := make(map[string]struct{})
	var tokens []string
	for _, c := range commands {
		if _, ok := seen[c]; !ok {
			seen[c] = struct{}{}
			tokens = append(tokens, c)
		}
	}
	for _, p := range paths {
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			tokens = append(tokens, p)
		}
	}

	if len(tokens) == 0 {
		return opaqueTokens(cmd)
	}
	return tokens
}

// isSubcommand reports whether w looks like a subcommand argument rather
// than a flag, path, or other option value. We restrict to lowercase alpha
// to avoid treating flags like "czf", build dirs like "build", or numeric
// versions like "1.2.3" as subcommands. Known subcommands like test, diff,
// run, get, show, status all match this pattern.
func isSubcommand(w string) bool {
	// Not a flag.
	if strings.HasPrefix(w, "-") {
		return false
	}
	// Not a path.
	if looksLikePath(w) {
		return false
	}
	// Pure lowercase alpha only. This matches subcommand names like
	// "test", "diff", "run", "get", "status" while rejecting
	// concatenated flags like "czf", directories like "build", etc.
	for _, r := range w {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// isSafeWord reports whether w is a plain literal word without any shell
// expansions or special constructs.
func isSafeWord(w *syntax.Word) bool {
	if w == nil {
		return false
	}
	for _, part := range w.Parts {
		switch part.(type) {
		case *syntax.SglQuoted, *syntax.Lit, *syntax.DblQuoted:
			// Safe.
		default:
			return false
		}
	}
	return true
}

// cmdText extracts the raw text of a word node for command/subcommand
// extraction.
func cmdText(w *syntax.Word) string {
	var parts []string
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			parts = append(parts, p.Value)
		case *syntax.SglQuoted:
			parts = append(parts, p.Value)
		case *syntax.DblQuoted:
			for _, dp := range p.Parts {
				if l, ok := dp.(*syntax.Lit); ok {
					parts = append(parts, l.Value)
				}
			}
		}
	}
	return strings.Join(parts, "")
}

// looksLikePath reports whether s looks like a filesystem path argument.
func looksLikePath(s string) bool {
	if strings.Contains(s, "/") {
		return true
	}
	if s == "." || s == ".." {
		return true
	}
	if strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") {
		return true
	}
	if strings.HasPrefix(s, "~/") {
		return true
	}
	if strings.HasPrefix(s, "~") {
		return true
	}
	return false
}

// cleanPath resolves and cleans a path string against the working directory.
func cleanPath(pathStr, workingDir string) string {
	// cd - refers to the previous working directory in bash — we can't
	// model this, so we skip emitting a path token for it.
	if pathStr == "-" {
		return ""
	}
	switch {
	case pathStr == ".":
		return workingDir
	case pathStr == "..":
		return filepath.Clean(workingDir + "/..")
	case strings.HasPrefix(pathStr, "~") && len(pathStr) > 1:
		usr, err := user.Current()
		if err != nil {
			return pathStr
		}
		if pathStr[1] == '/' || pathStr[1] == 0 {
			return filepath.Join(usr.HomeDir, pathStr[1:])
		}
		// ~user format — we can't resolve arbitrary users, use as-is.
		return pathStr
	default:
		if filepath.IsAbs(pathStr) {
			return filepath.Clean(pathStr)
		}
		return filepath.Clean(filepath.Join(workingDir, pathStr))
	}
}
