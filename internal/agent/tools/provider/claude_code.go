package provider

import (
	"cmp"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"charm.land/fantasy"
)

const ClaudeCodeToolName = "claude_code"

// Biến env cấu hình tool claude_code. Phải có trong môi trường của tiến
// trình agent-cli (export ở shell); biến chỉ khai trong crushrc/tệp env
// không tới được đây vì crushrc chạy trong trình thông dịch riêng.
const (
	claudeBinEnv  = "AGENT_CLI_CLAUDE_BIN"
	claudeModeEnv = "AGENT_CLI_CLAUDE_PERMISSION_MODE"
)

// defaultClaudePermissionMode giữ mặc định đã chốt của tool: Claude Code
// dùng mọi tool không hỏi lại, sau khi người dùng duyệt lời uỷ quyền.
const defaultClaudePermissionMode = "bypassPermissions"

//go:embed claude_code.md
var claudeCodeDescription string

// claudeCommand is the Claude Code binary. Tests point it at a fake script.
var claudeCommand = "claude"

// claudeStrategy adapts Claude Code's headless interface: one JSON result
// object whose is_error flag and session_id carry the outcome. Giá trị
// rỗng của bin/mode nghĩa là dùng mặc định.
type claudeStrategy struct {
	bin  string
	mode string
}

// newClaudeStrategy đọc cấu hình từ env lúc dựng tool.
func newClaudeStrategy() claudeStrategy {
	return claudeStrategy{bin: os.Getenv(claudeBinEnv), mode: os.Getenv(claudeModeEnv)}
}

func (claudeStrategy) name() string       { return ClaudeCodeToolName }
func (claudeStrategy) describe() string   { return claudeCodeDescription }
func (claudeStrategy) label() string      { return "Claude Code" }
func (claudeStrategy) binaryEnv() string  { return claudeBinEnv }
func (s claudeStrategy) binary() string   { return cmp.Or(s.bin, claudeCommand) }
func (s claudeStrategy) permMode() string { return cmp.Or(s.mode, defaultClaudePermissionMode) }
func (s claudeStrategy) access() string   { return strings.Join(s.permissionArgs(), " ") }

// permissionArgs giữ nguyên cờ đã chạy thật cho mặc định, và dùng
// --permission-mode cho mọi mức khác (acceptEdits, plan, ...).
func (s claudeStrategy) permissionArgs() []string {
	if s.permMode() == defaultClaudePermissionMode {
		return []string{"--dangerously-skip-permissions"}
	}
	return []string{"--permission-mode", s.permMode()}
}

// args assembles the headless invocation. The permission prompt of the
// tool call is the gate: the user approves each delegation, so the nested
// agent runs without prompts it could not ask anyone to answer.
func (s claudeStrategy) args(prompt, resume string) []string {
	args := append([]string{"-p", "--output-format", "json"}, s.permissionArgs()...)
	if resume != "" {
		// Một token "--resume=<id>": id không thể bị tách thành cờ riêng.
		args = append(args, "--resume="+resume)
	}
	// "--" để prompt bắt đầu bằng "-" (vd gạch đầu dòng) không bị hiểu
	// thành cờ: claude báo "unknown option" và thoát.
	return append(args, "--", prompt)
}

// claudeCodeResult mirrors the summary object printed by
// "claude -p --output-format json". Only the fields this tool surfaces are
// decoded; the payload also carries usage and cost details we ignore.
type claudeCodeResult struct {
	Result    string `json:"result"`
	SessionID string `json:"session_id"`
	IsError   bool   `json:"is_error"`
}

func (claudeStrategy) parse(stdout []byte) (delegationOutput, error) {
	var res claudeCodeResult
	if err := json.Unmarshal(stdout, &res); err != nil {
		return delegationOutput{}, fmt.Errorf("parsing Claude Code output: %w", err)
	}
	if res.IsError {
		return delegationOutput{}, &delegationError{msg: fmt.Sprintf("Claude Code reported a failure: %s", res.Result)}
	}
	return delegationOutput{text: res.Result, sessionID: res.SessionID}, nil
}

// NewClaudeCodeTool delegates a self-contained task to Claude Code running
// headlessly in the working directory, after the user approves the call.
func NewClaudeCodeTool(workingDir string, perms permissionRequester) fantasy.AgentTool {
	return newDelegationTool(workingDir, perms, newClaudeStrategy())
}
