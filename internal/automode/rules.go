// Package automode implements a native auto mode for Crush: a safety
// classifier that runs in-process before permission prompts, deciding
// allow/deny via static rules and an optional LLM classifier. The rules
// and pipeline mirror the standalone automatoer tool so both share the
// same semantics.
package automode

import "strings"

// Rules holds the static classification lists.
type Rules struct {
	readOnly       map[string]bool
	write          map[string]bool
	lowRisk        map[string]bool
	dangerous      []string
	banned         map[string]bool
	safe           map[string]bool
	protectedPaths []string
}

// DefaultRules returns the built-in classification rules, aligned with
// Crush's tool categories and automatoer's lists.
func DefaultRules() *Rules {
	return &Rules{
		readOnly: toSet([]string{
			"view", "ls", "glob", "grep",
			"lsp_diagnostics", "lsp_references", "lsp_symbols",
			"lsp_definition", "lsp_call_hierarchy",
			"sourcegraph", "crush_info", "crush_logs",
			"job_output", "todos", "question",
			"list_mcp_resources", "read_mcp_resource",
			"fetch", "agentic_fetch",
			"read", "search", "web_search",
		}),
		write: toSet([]string{
			"edit", "multiedit", "write",
			"lsp_rename", "lsp_replace_symbol",
		}),
		lowRisk: toSet([]string{
			"job_kill", "lsp_restart",
		}),
		dangerous:      defaultDangerousPatterns(),
		banned:         toSet(defaultBannedCommands()),
		safe:           toSet(defaultSafeCommands()),
		protectedPaths: defaultProtectedPaths(),
	}
}

// IsReadOnly reports whether the tool is read-only.
func (r *Rules) IsReadOnly(toolName string) bool {
	return r.readOnly[toolName]
}

// IsWriteTool reports whether the tool modifies files.
func (r *Rules) IsWriteTool(toolName string) bool {
	return r.write[toolName]
}

// IsLowRisk reports whether the tool is low-risk.
func (r *Rules) IsLowRisk(toolName string) bool {
	return r.lowRisk[toolName]
}

// IsDangerous reports whether a command matches a dangerous pattern.
func (r *Rules) IsDangerous(cmd string) bool {
	lower := strings.ToLower(strings.TrimSpace(cmd))
	for _, pattern := range r.dangerous {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// IsBanned reports whether a command contains a banned program name as a
// distinct token (not a substring of another word).
func (r *Rules) IsBanned(cmd string) bool {
	for _, tok := range tokenize(cmd) {
		if r.banned[strings.ToLower(tok)] {
			return true
		}
	}
	return false
}

// IsSafeCommand reports whether every segment of a chained command
// (split on ; | && ||) is a known safe read-only prefix with no shell
// chaining of its own. This prevents `ls && rm -rf /` from being marked
// safe because of the leading `ls`.
func (r *Rules) IsSafeCommand(cmd string) bool {
	segments := splitCommandSegments(cmd)
	if len(segments) == 0 {
		return false
	}
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if !r.isSingleSafeSegment(seg) {
			return false
		}
	}
	return true
}

func (r *Rules) isSingleSafeSegment(seg string) bool {
	chainingChars := []string{";", "|", "&&", "$(", "`"}
	for _, c := range chainingChars {
		if strings.Contains(seg, c) {
			return false
		}
	}
	lower := strings.ToLower(seg)
	for safe := range r.safe {
		if strings.HasPrefix(lower, safe) {
			if len(lower) == len(safe) || lower[len(safe)] == ' ' || lower[len(safe)] == '-' {
				return true
			}
		}
	}
	return false
}

// IsProtectedPath reports whether a file path matches one of the
// protected patterns that should always be classified (never auto-allowed
// by in-project write trust or read-only exceptions).
func (r *Rules) IsProtectedPath(path string) bool {
	if path == "" {
		return false
	}
	lower := strings.ToLower(path)
	for _, p := range r.protectedPaths {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func toSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}

// tokenize splits a shell command into tokens respecting single and
// double quotes. It is intentionally simple, but sufficient to
// distinguish `curl` from `nocurl` and `apt-get` from `capture`.
func tokenize(cmd string) []string {
	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	for _, r := range cmd {
		switch {
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case (r == ' ' || r == '\t') && !inSingle && !inDouble:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// splitCommandSegments splits a shell command on ; | && || so each
// segment can be evaluated independently. Quoted delimiters are not split.
func splitCommandSegments(cmd string) []string {
	var segments []string
	var current strings.Builder
	runes := []rune(cmd)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\'' || r == '"' {
			current.WriteRune(r)
			quote := r
			i++
			for i < len(runes) && runes[i] != quote {
				current.WriteRune(runes[i])
				i++
			}
			if i < len(runes) {
				current.WriteRune(runes[i])
			}
			continue
		}
		if r == '&' && i+1 < len(runes) && runes[i+1] == '&' {
			segments = append(segments, current.String())
			current.Reset()
			i++ // skip second &
			continue
		}
		if r == '|' && i+1 < len(runes) && runes[i+1] == '|' {
			segments = append(segments, current.String())
			current.Reset()
			i++ // skip second |
			continue
		}
		if r == ';' || r == '|' {
			segments = append(segments, current.String())
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		segments = append(segments, current.String())
	}
	return segments
}

func defaultDangerousPatterns() []string {
	return []string{
		"rm -rf", "rm -fr",
		"--force", "--no-verify",
		"push --force", "push -f",
		"rebase --onto", "reset --hard",
		"clean -fd", "clean -dfx",
		"chmod 777", "chmod -r",
		"chown ", "mkfs", "dd if=",
		"> /dev/", "truncate ", "shred ",
		":(){ :|:",
		"pip install", "pip3 install",
		"npm install --global", "npm install -g",
		"pnpm add --global", "pnpm add -g",
		"yarn global add",
		"cargo install", "gem install",
		"go install", "brew install",
	}
}

func defaultBannedCommands() []string {
	return []string{
		"alias", "aria2c", "axel", "chrome", "curl", "curlie",
		"firefox", "http-prompt", "httpie", "links", "lynx", "nc",
		"safari", "scp", "ssh", "telnet", "w3m", "wget", "xh",
		"doas", "su", "sudo",
		"apk", "apt", "apt-cache", "apt-get", "dnf", "dpkg",
		"emerge", "home-manager", "makepkg", "opkg", "pacman",
		"paru", "pkg", "pkg_add", "pkg_delete", "portage", "rpm",
		"yay", "yum", "zypper",
		"at", "batch", "chkconfig", "crontab", "fdisk", "mkfs",
		"mount", "parted", "service", "systemctl", "umount",
		"firewall-cmd", "ifconfig", "ip", "iptables", "netstat",
		"pfctl", "route", "ufw",
	}
}

func defaultSafeCommands() []string {
	return []string{
		"cal", "date", "df", "du", "echo", "env", "free",
		"groups", "hostname", "id", "kill", "killall", "ls",
		"nice", "nohup", "printenv", "ps", "pwd", "set",
		"time", "timeout", "top", "type", "uname", "unset",
		"uptime", "whatis", "whereis", "which", "whoami",
		"git blame", "git branch", "git config --get",
		"git config --list", "git describe", "git diff",
		"git grep", "git log", "git ls-files", "git ls-remote",
		"git remote", "git rev-parse", "git shortlog",
		"git show", "git status", "git tag",
	}
}

func defaultProtectedPaths() []string {
	return []string{
		"/.git/", "/.git\\",
		"/.claude/", "/.crush/",
		".bashrc", ".zshrc", ".profile", ".bash_profile",
		".zprofile", ".login", ".logout",
		"/etc/profile", "/etc/bash.bashrc",
		".ssh/authorized_keys", ".ssh/config",
		"automatoer.json", ".pi/auto-mode",
	}
}
