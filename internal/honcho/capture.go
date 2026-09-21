package honcho

import (
	"path/filepath"
	"slices"
	"strings"
)

// maxSummaryLen bounds a tool summary. These are one-line notes, and
// an unbounded one would drown the conversation it annotates.
const maxSummaryLen = 300

// redactedArgs replaces a command's arguments when any of them look
// like a credential.
const redactedArgs = " (arguments redacted)"

// trivialCommands produce no memory worth keeping. Recording that
// Crush ran `ls` teaches nothing about the work.
var trivialCommands = map[string]bool{
	"ls": true, "pwd": true, "cd": true, "echo": true, "cat": true,
	"head": true, "tail": true, "wc": true, "which": true, "whoami": true,
	"date": true, "true": true, "false": true, "clear": true, "env": true,
	"printf": true, "dirname": true, "basename": true, "sleep": true,
}

// readOnlyTools are already visible in the transcript and carry no
// durable signal about intent.
var readOnlyTools = map[string]bool{
	"view": true, "read": true, "glob": true, "grep": true, "ls": true,
	"diagnostics": true, "lsp_definition": true, "lsp_references": true,
	"lsp_symbols": true, "lsp_call_hierarchy": true, "crush_info": true,
	"crush_logs": true, "job_output": true, "list_mcp_resources": true,
	"read_mcp_resource": true, "sourcegraph": true, "fetch": true,
}

// credentialTokens mark an argument as sensitive. Matching is on a
// lowercased substring, so `--api-key`, `API_KEY=`, and
// `Authorization:` all trip it.
var credentialTokens = []string{
	"api-key", "api_key", "apikey",
	"secret", "token", "password", "passwd",
	"authorization", "bearer", "cookie",
	"credential", "private-key", "private_key",
	"access-key", "access_key", "session-key",
}

// shortCredentialFlags are flags that conventionally precede a
// credential, making the NEXT argument sensitive. `-H` is included
// because an auth header is the most common way a secret reaches
// curl, and the cost of redacting a benign `-H "Accept: json"` is one
// uninteresting memory line.
var shortCredentialFlags = map[string]bool{
	"-p": true, "-u": true, "-P": true, "-H": true,
	"--header": true, "--data": true, "-d": true,
}

// secretPrefixes identify a token by shape alone, so a credential
// pasted with no surrounding keyword is still caught.
var secretPrefixes = []string{
	"sk-", "sk_", "pk_live", "rk_live",
	"ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_",
	"xoxb-", "xoxp-", "xoxa-", "xoxs-",
	"akia", "asia",
	"aiza", "ya29.",
	"hch-", "glpat-", "dop_v1_", "shpat_", "npm_",
	"eyj", // A JWT header always base64-encodes to this.
}

// SummarizeTool renders a tool call as a single memory-worthy line,
// or returns the empty string when the call is not worth remembering.
//
// The summary is what Honcho reasons over, so it must describe intent
// ("edited internal/foo.go") rather than mechanism, and must never
// carry a credential that appeared in an argument.
func SummarizeTool(tool string, params map[string]any) string {
	tool = strings.ToLower(strings.TrimSpace(tool))
	if tool == "" {
		return ""
	}
	// Memory tools describing memory operations is noise at best and
	// a feedback loop at worst.
	if strings.HasPrefix(tool, "honcho_") || readOnlyTools[tool] {
		return ""
	}

	switch tool {
	case "bash", "shell", "exec", "run":
		cmd := stringParam(params, "command", "cmd", "script")
		return clampSummary(summarizeShell(cmd))

	case "edit", "multiedit":
		if p := pathParam(params); p != "" {
			return clampSummary("Edited " + p)
		}
		return "Edited a file"

	case "write":
		if p := pathParam(params); p != "" {
			return clampSummary("Wrote " + p)
		}
		return "Wrote a file"

	case "download":
		if p := pathParam(params); p != "" {
			return clampSummary("Downloaded to " + p)
		}
		return "Downloaded a file"

	case "lsp_rename":
		from := stringParam(params, "symbol")
		to := stringParam(params, "new_name")
		if from != "" && to != "" {
			return clampSummary("Renamed " + from + " to " + to)
		}
		return "Renamed a symbol"

	case "lsp_replace_symbol":
		if sym := stringParam(params, "symbol"); sym != "" {
			return clampSummary("Replaced " + sym)
		}
		return "Replaced a symbol"

	case "agent", "task":
		if d := stringParam(params, "prompt", "description", "label"); d != "" {
			return clampSummary("Delegated: " + firstLine(d))
		}
		return "Delegated a task"

	default:
		return clampSummary("Used " + tool)
	}
}

// summarizeShell renders a shell command, keeping the most meaningful
// segment of a compound command and redacting credentials.
func summarizeShell(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}
	segments := splitCommand(cmd)

	// Report the last non-trivial segment: in `cd foo && go test`,
	// the test run is the part worth remembering.
	for _, seg := range slices.Backward(segments) {
		seg = strings.TrimSpace(seg)
		if seg == "" || isTrivialCommand(seg) {
			continue
		}
		return "Ran: " + RedactCommand(seg)
	}
	return ""
}

// splitCommand breaks a compound shell command into its segments,
// respecting quotes so a separator inside a string literal does not
// split the command.
func splitCommand(cmd string) []string {
	var (
		segments []string
		cur      strings.Builder
		quote    rune
		escaped  bool
	)
	runes := []rune(cmd)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			cur.WriteRune(r)
			escaped = true
			continue
		}
		if quote != 0 {
			cur.WriteRune(r)
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			cur.WriteRune(r)
			continue
		}

		// Unquoted separator.
		switch r {
		case ';', '\n', '|':
			// `||` is one separator, not two.
			if r == '|' && i+1 < len(runes) && runes[i+1] == '|' {
				i++
			}
			segments = append(segments, cur.String())
			cur.Reset()
			continue
		case '&':
			if i+1 < len(runes) && runes[i+1] == '&' {
				i++
				segments = append(segments, cur.String())
				cur.Reset()
				continue
			}
		}
		cur.WriteRune(r)
	}
	segments = append(segments, cur.String())
	return segments
}

// isTrivialCommand reports whether a segment's executable is one whose
// invocation carries no durable meaning.
func isTrivialCommand(seg string) bool {
	fields := strings.Fields(seg)
	if len(fields) == 0 {
		return true
	}
	// Skip leading VAR=value assignments to find the executable.
	exe := ""
	for _, f := range fields {
		if strings.Contains(f, "=") && !strings.HasPrefix(f, "-") {
			continue
		}
		exe = f
		break
	}
	if exe == "" {
		return true
	}
	return trivialCommands[filepath.Base(exe)]
}

// RedactCommand returns a command safe to store, dropping every
// argument when any of them looks like a credential.
//
// The all-or-nothing rule is deliberate. Redacting only the matching
// argument leaks position and shape, and a credential can arrive in
// an argument that does not itself match any pattern (`--header
// "X-Key: hunter2"`). Keeping only the executable name is the only
// version that is obviously correct.
func RedactCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}

	exe := filepath.Base(fields[0])
	for i, f := range fields {
		if !looksSensitive(f) {
			// A bare `-p` or `-u` makes the NEXT argument the secret.
			if i > 0 && shortCredentialFlags[fields[i-1]] {
				return exe + redactedArgs
			}
			continue
		}
		return exe + redactedArgs
	}
	// A URL carrying inline credentials (user:pass@host) is sensitive
	// even when no keyword appears.
	if hasInlineCredentials(cmd) {
		return exe + redactedArgs
	}
	return cmd
}

// looksSensitive reports whether a single argument suggests a secret.
func looksSensitive(arg string) bool {
	lower := strings.ToLower(arg)
	for _, tok := range credentialTokens {
		if strings.Contains(lower, tok) {
			return true
		}
	}
	// Long flags that name a user are usually paired with a password.
	if strings.HasPrefix(lower, "--user") || strings.HasPrefix(lower, "--pass") {
		return true
	}
	// A token recognizable by its issuer prefix, wherever it appears
	// in the argument. Checking the whole argument rather than just
	// its start catches `Authorization:Bearer ghp_...` and
	// `KEY=sk-...` without needing to parse either.
	for _, prefix := range secretPrefixes {
		if strings.Contains(lower, prefix) {
			return true
		}
	}
	return looksHighEntropy(arg)
}

// looksHighEntropy reports whether an argument has the shape of a
// random credential: long, and mixing character classes the way
// generated tokens do and ordinary words and paths do not.
//
// This is the backstop for a secret with no recognizable prefix and
// no surrounding keyword. It errs toward redaction, since the cost of
// a false positive is one vague memory line and the cost of a false
// negative is a leaked credential.
func looksHighEntropy(arg string) bool {
	// Strip a leading flag or assignment so `--key=<secret>` is
	// judged on the secret rather than the whole argument.
	if _, after, found := strings.Cut(arg, "="); found {
		arg = after
	}
	// Require a length real credentials reach and ordinary
	// identifiers rarely do. GitHub and AWS tokens are 40 characters,
	// Stripe keys 32 or more; a Go test name or a flag value that
	// long is uncommon enough that the occasional vague memory line
	// is a fair trade.
	if len(arg) < 32 {
		return false
	}
	// Paths and URLs are long and mixed but are not secrets. A
	// credential embedded in a URL is caught by hasInlineCredentials.
	if strings.ContainsAny(arg, "/\\ ") {
		return false
	}

	var upper, lower, digit int
	for _, r := range arg {
		switch {
		case r >= 'A' && r <= 'Z':
			upper++
		case r >= 'a' && r <= 'z':
			lower++
		case r >= '0' && r <= '9':
			digit++
		}
	}
	// Require all three classes present, which rules out prose,
	// flags, hostnames, and hex-only strings like git SHAs.
	return upper > 0 && lower > 0 && digit > 0
}

// hasInlineCredentials reports whether the command contains a URL with
// embedded user:password credentials.
func hasInlineCredentials(cmd string) bool {
	for _, scheme := range []string{"://"} {
		idx := 0
		for {
			i := strings.Index(cmd[idx:], scheme)
			if i < 0 {
				break
			}
			start := idx + i + len(scheme)
			// Look for user:pass@ before the next path separator or
			// whitespace.
			end := len(cmd)
			for j := start; j < len(cmd); j++ {
				if cmd[j] == '/' || cmd[j] == ' ' || cmd[j] == '\t' {
					end = j
					break
				}
			}
			authority := cmd[start:end]
			if at := strings.Index(authority, "@"); at > 0 {
				if strings.Contains(authority[:at], ":") {
					return true
				}
			}
			idx = start
		}
	}
	return false
}

// stringParam returns the first present string parameter among keys.
func stringParam(params map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := params[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

// pathParam returns a file path parameter under any of its common
// names, shortened to something readable.
func pathParam(params map[string]any) string {
	p := stringParam(params, "file_path", "path", "filePath", "filename")
	if p == "" {
		return ""
	}
	return shortenPath(p)
}

// shortenPath trims an absolute path to its last few segments, which
// is enough to identify the file without recording the machine's
// directory layout.
func shortenPath(p string) string {
	p = filepath.ToSlash(p)
	parts := strings.Split(strings.Trim(p, "/"), "/")
	if len(parts) <= 3 {
		return p
	}
	return ".../" + strings.Join(parts[len(parts)-3:], "/")
}

// firstLine returns the first non-empty line of s.
func firstLine(s string) string {
	for line := range strings.SplitSeq(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

// clampSummary bounds a summary line.
func clampSummary(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return clamp(s, maxSummaryLen)
}
