package tools

import (
	"encoding/json"
	"regexp"
)

// WriteToolNames mutate files; a successful result supersedes earlier
// reads of the same path. The set is also the verifyingTool wrap set and
// the gate's metadata-scan set — lsp_rename/lsp_replace_symbol mutate
// via workspace edits, the canonical caller-breaker.
var WriteToolNames = map[string]bool{
	"edit": true, "write": true, "multiedit": true,
	"lsp_rename": true, "lsp_replace_symbol": true,
}

// ReadToolNames capture file content; their results go stale on writes.
var ReadToolNames = map[string]bool{"view": true, "read": true}

// CommandToolNames emit re-derivable output: once a result is old
// enough to leave the recency guard, or a re-run makes it redundant,
// a labeled stub suffices.
var CommandToolNames = map[string]bool{"bash": true, "grep": true, "glob": true, "ls": true}

// mutatingBashRe matches shell commands that mutate files or git state
// — the "large, destructive, hard to reverse" calls that must not
// bypass the write boundary just because they arrive through bash
// instead of a write tool. Deliberately conservative in both
// directions: mutations hidden inside scripts or build targets (make,
// go generate) pass un-gated, and read-ish commands that merely touch
// state (git config --get) stay exploration. A false positive costs
// one confirmation question; a false negative skips the checkpoint.
//
// The scan runs on the raw command text — command names inside quoted
// spans still match ("bash -c 'rm -rf /'" gates) — while the redirect
// check below masks quoted spans so "echo 'a > b'" stays exploration.
var mutatingBashRe = regexp.MustCompile(`\b(rm|rmdir|mv|cp|dd|truncate|shred|chmod|chown|chgrp|ln|tee|patch|install|touch|mkdir|rsync|scp)\b|` +
	`\b(sed|perl)\s+(-\S+\s+)*(-\S*i|-i\S*|--in-place)\b|` +
	`\bgit\s+(commit|push|reset|checkout|switch|restore|clean|rebase|merge|am|apply|stash|tag|revert|cherry-pick|mv|rm|init|clone|pull|bisect|submodule|update-ref|notes|branch\s+-[dDmM])\b|` +
	`\bapt(-get)?\s+(install|remove|purge|upgrade|update|dist-upgrade)\b|` +
	`\bkubectl\s+(delete|apply|create|patch|edit|replace|scale|drain|cordon|uncordon)\b`)

// redirectTargetRe finds shell redirects and their targets; writing to
// a real file mutates it, while fd duplication and /dev/null do not.
var redirectTargetRe = regexp.MustCompile(`>>?\s*(\S+)`)

// fdDupTargetRe matches the fd-duplication redirect targets that are
// not file writes — `>&1`, `>&-` — as opposed to `>&out`, which is
// bash's stdout+stderr-to-file form and does mutate.
var fdDupTargetRe = regexp.MustCompile(`^&[-\d]`)

// quotedSpanRe masks single- and double-quoted spans before the
// redirect scan: a `>` inside a string literal must not gate, while a
// quoted *target* (`> 'out'`) still counts — masking to a placeholder
// keeps the target position occupied.
var quotedSpanRe = regexp.MustCompile(`'[^']*'|"[^"]*"`)

// IsMutatingCall classifies a call as a write for boundary purposes:
// a write-tool name, a file-writing download, or a bash command whose
// text matches a mutating pattern or a file-writing redirect. It is
// the single vocabulary shared by the scope gate's intercept path and
// the notebook checkpoint's write-boundary trigger — one boundary,
// one definition.
func IsMutatingCall(name, input string) bool {
	if WriteToolNames[name] || name == DownloadToolName {
		return true
	}
	if name != "bash" {
		return false
	}
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(input), &params); err != nil || params.Command == "" {
		return false
	}
	if mutatingBashRe.MatchString(params.Command) {
		return true
	}
	for _, m := range redirectTargetRe.FindAllStringSubmatch(quotedSpanRe.ReplaceAllString(params.Command, "f"), -1) {
		if m[1] != "/dev/null" && !fdDupTargetRe.MatchString(m[1]) {
			return true
		}
	}
	return false
}

// ToolCallFilePath extracts the file path from a tool call's JSON
// input, trying the conventional keys.
func ToolCallFilePath(input string) string {
	var fields map[string]any
	if err := json.Unmarshal([]byte(input), &fields); err != nil {
		return ""
	}
	for _, key := range []string{"file_path", "path", "file"} {
		if s, ok := fields[key].(string); ok && s != "" {
			return s
		}
	}
	return ""
}
