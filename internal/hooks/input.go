package hooks

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/tidwall/gjson"
)

// SupportedOutputVersion is the highest envelope version this build
// understands. Hooks may omit `version` entirely (treated as 1) or pin
// an older version. Unknown higher versions are still parsed but logged.
const SupportedOutputVersion = 1

// Payload is the JSON structure piped to hook commands via stdin. Field
// names follow Claude Code where an equivalent exists so scripts written
// for it keep working; `event` is kept alongside `hook_event_name`
// because earlier Crush releases shipped it.
//
// ToolInput is emitted as a parsed JSON object rather than a string.
type Payload struct {
	Event                string          `json:"event"`
	HookEventName        string          `json:"hook_event_name"`
	SessionID            string          `json:"session_id"`
	CWD                  string          `json:"cwd"`
	ToolName             string          `json:"tool_name,omitempty"`
	ToolInput            json.RawMessage `json:"tool_input,omitempty"`
	ToolUseID            string          `json:"tool_use_id,omitempty"`
	ToolResponse         *string         `json:"tool_response,omitempty"`
	Error                string          `json:"error,omitempty"`
	LastAssistantMessage *string         `json:"last_assistant_message,omitempty"`
	Decision             string          `json:"decision,omitempty"`
	Source               string          `json:"source,omitempty"`
	Reason               string          `json:"reason,omitempty"`
	Trigger              string          `json:"trigger,omitempty"`
}

// BuildPayload constructs the JSON stdin payload for an event.
func BuildPayload(ev Event, cwd string) []byte {
	p := Payload{
		Event:                ev.Name,
		HookEventName:        ev.Name,
		SessionID:            ev.SessionID,
		CWD:                  cwd,
		ToolResponse:         ev.ToolResponse,
		Error:                ev.Error,
		LastAssistantMessage: ev.LastAssistantMessage,
		Decision:             ev.Decision,
		Source:               ev.Source,
		Reason:               ev.Reason,
		Trigger:              ev.Trigger,
	}
	if ev.Tool != nil {
		p.ToolName = ev.Tool.Name
		p.ToolUseID = ev.Tool.ID
		p.ToolInput = json.RawMessage(ev.Tool.Input)
		if !json.Valid(p.ToolInput) {
			p.ToolInput = json.RawMessage("{}")
		}
	}
	data, err := json.Marshal(p)
	if err != nil {
		return []byte("{}")
	}
	return data
}

// BuildEnv constructs the environment variable slice for a hook command.
// It includes all current process env vars plus hook-specific ones. The
// tool variables are only set for events that carry a tool call.
// CLAUDE_PROJECT_DIR mirrors CRUSH_PROJECT_DIR so hook scripts written
// for Claude Code can locate themselves.
func BuildEnv(ev Event, cwd, projectDir string) []string {
	env := os.Environ()
	env = append(env, shell.CrushEnvMarkers()...)
	env = append(
		env,
		fmt.Sprintf("CRUSH_EVENT=%s", ev.Name),
		fmt.Sprintf("CRUSH_SESSION_ID=%s", ev.SessionID),
		fmt.Sprintf("CRUSH_CWD=%s", cwd),
		fmt.Sprintf("CRUSH_PROJECT_DIR=%s", projectDir),
		fmt.Sprintf("CLAUDE_PROJECT_DIR=%s", projectDir),
	)
	if ev.Tool == nil {
		return env
	}
	env = append(env, fmt.Sprintf("CRUSH_TOOL_NAME=%s", ev.Tool.Name))
	if cmd := gjson.Get(ev.Tool.Input, "command"); cmd.Exists() {
		env = append(env, fmt.Sprintf("CRUSH_TOOL_INPUT_COMMAND=%s", cmd.String()))
	}
	if fp := gjson.Get(ev.Tool.Input, "file_path"); fp.Exists() {
		env = append(env, fmt.Sprintf("CRUSH_TOOL_INPUT_FILE_PATH=%s", fp.String()))
	}
	return env
}

// parseStdout parses the JSON output from a hook command's stdout. It
// accepts the Crush envelope and the Claude Code forms: a top-level
// `decision: "block"` with `reason`, `continue: false` with
// `stopReason`, and `hookSpecificOutput`. The second result is false
// when stdout is not a JSON object, so callers can tell "no output"
// from "unparseable output".
func parseStdout(stdout string) (HookResult, bool) {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return HookResult{Decision: DecisionNone}, false
	}

	var parsed struct {
		Version            int             `json:"version"`
		Decision           string          `json:"decision"`
		Halt               bool            `json:"halt"`
		Reason             string          `json:"reason"`
		Context            json.RawMessage `json:"context"`
		UpdatedInput       json.RawMessage `json:"updated_input"`
		UpdatedOutput      json.RawMessage `json:"updated_output"`
		Continue           *bool           `json:"continue"`
		StopReason         string          `json:"stopReason"`
		HookSpecificOutput json.RawMessage `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		return HookResult{Decision: DecisionNone}, false
	}

	if parsed.Version > SupportedOutputVersion {
		slog.Debug(
			"Hook output declared a newer envelope version than this build supports",
			"version", parsed.Version,
			"supported", SupportedOutputVersion,
		)
	}

	result := HookResult{
		Decision:      parseDecision(parsed.Decision),
		Halt:          parsed.Halt,
		Reason:        parsed.Reason,
		Context:       parseContext(parsed.Context),
		UpdatedInput:  rawToString(parsed.UpdatedInput),
		UpdatedOutput: rawToString(parsed.UpdatedOutput),
	}
	if parsed.Continue != nil && !*parsed.Continue {
		result.Halt = true
		if parsed.StopReason != "" {
			result.Reason = parsed.StopReason
		}
	}
	if len(parsed.HookSpecificOutput) > 0 {
		applyClaudeCodeOutput(&result, parsed.HookSpecificOutput)
	}
	return result, true
}

// parseContext accepts either a single string or an array of strings and
// returns a newline-joined value with empty entries dropped.
func parseContext(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	// String form.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
		return ""
	}
	// Array form.
	if raw[0] == '[' {
		var items []string
		if err := json.Unmarshal(raw, &items); err != nil {
			return ""
		}
		out := items[:0]
		for _, s := range items {
			if s != "" {
				out = append(out, s)
			}
		}
		return strings.Join(out, "\n")
	}
	return ""
}

// applyClaudeCodeOutput folds a Claude Code hookSpecificOutput object
// into result: permissionDecision maps onto decision, updatedInput onto
// UpdatedInput, and updatedToolOutput onto UpdatedOutput.
func applyClaudeCodeOutput(result *HookResult, data json.RawMessage) {
	var hso struct {
		PermissionDecision       string          `json:"permissionDecision"`
		PermissionDecisionReason string          `json:"permissionDecisionReason"`
		UpdatedInput             json.RawMessage `json:"updatedInput"`
		UpdatedToolOutput        json.RawMessage `json:"updatedToolOutput"`
		AdditionalContext        string          `json:"additionalContext"`
	}
	if err := json.Unmarshal(data, &hso); err != nil {
		return
	}
	if d := parseDecision(hso.PermissionDecision); d != DecisionNone {
		result.Decision = d
	}
	if hso.PermissionDecisionReason != "" {
		result.Reason = hso.PermissionDecisionReason
	}
	if hso.AdditionalContext != "" {
		result.Context = strings.TrimSpace(result.Context + "\n" + hso.AdditionalContext)
	}
	if s := rawToString(hso.UpdatedInput); s != "" {
		result.UpdatedInput = s
	}
	if s := rawToString(hso.UpdatedToolOutput); s != "" {
		result.UpdatedOutput = s
	}
}

// rawToString converts a json.RawMessage to a string suitable for use
// as opaque tool input. It accepts both a JSON object (nested) and a
// JSON string (stringified, for backward compatibility).
func rawToString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	// If it's a JSON string, unwrap it.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	// Otherwise it's an object/array — use as-is.
	return string(raw)
}

// parseDecision maps both Crush ("allow"/"deny") and Claude Code
// ("block", plus the deprecated "approve") spellings onto a Decision.
func parseDecision(s string) Decision {
	switch strings.ToLower(s) {
	case "allow", "approve":
		return DecisionAllow
	case "deny", "block":
		return DecisionDeny
	default:
		return DecisionNone
	}
}
