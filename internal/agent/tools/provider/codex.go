package provider

import (
	"cmp"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/fantasy"
)

const CodexToolName = "codex"

// Biến env cấu hình tool codex; như tool claude_code, phải có trong môi
// trường của tiến trình agent-cli (export ở shell).
const (
	codexBinEnv     = "AGENT_CLI_CODEX_BIN"
	codexSandboxEnv = "AGENT_CLI_CODEX_SANDBOX"
)

// defaultCodexSandbox giữ mặc định đã chốt của tool, cùng mức tin cậy với
// claude_code: Codex chạy lệnh không giới hạn sau khi lời uỷ quyền được duyệt.
const defaultCodexSandbox = "danger-full-access"

//go:embed codex.md
var codexDescription string

// codexCommand is the Codex CLI binary. Tests point it at a fake script.
var codexCommand = "codex"

// codexStrategy adapts Codex's headless interface: a JSONL event stream
// whose error, turn.failed, and agent_message events carry the outcome.
// Giá trị rỗng của bin/sandbox nghĩa là dùng mặc định.
type codexStrategy struct {
	bin     string
	sandbox string
}

// newCodexStrategy đọc cấu hình từ env lúc dựng tool.
func newCodexStrategy() codexStrategy {
	return codexStrategy{bin: os.Getenv(codexBinEnv), sandbox: os.Getenv(codexSandboxEnv)}
}

func (codexStrategy) name() string          { return CodexToolName }
func (codexStrategy) describe() string      { return codexDescription }
func (codexStrategy) label() string         { return "Codex" }
func (codexStrategy) binaryEnv() string     { return codexBinEnv }
func (s codexStrategy) binary() string      { return cmp.Or(s.bin, codexCommand) }
func (s codexStrategy) sandboxMode() string { return cmp.Or(s.sandbox, defaultCodexSandbox) }
func (s codexStrategy) access() string      { return "sandbox " + s.sandboxMode() }

// args assembles the headless invocation. The permission prompt of the
// tool call is the gate, as for claude_code.
func (s codexStrategy) args(prompt, resume string) []string {
	args := []string{"exec"}
	if resume != "" {
		args = append(args, "resume")
	}
	// "exec resume" không nhận -s/--sandbox (codex-cli 0.159.2 báo
	// "unexpected argument '-s'"), nên mức sandbox đi qua -c, dùng được cho
	// cả lượt mới lẫn lượt tiếp tục. "--" để prompt bắt đầu bằng "-" không
	// bị hiểu thành cờ.
	args = append(args,
		"--json",
		"-c", fmt.Sprintf("sandbox_mode=%q", s.sandboxMode()),
		"--skip-git-repo-check",
		"--",
	)
	if resume != "" {
		args = append(args, resume)
	}
	return append(args, prompt)
}

// codexEvent is one line of the JSONL stream printed by "codex exec --json".
// Only the fields this tool surfaces are decoded.
type codexEvent struct {
	Type     string `json:"type"`
	ThreadID string `json:"thread_id"`
	Message  string `json:"message"`
	Error    *struct {
		Message string `json:"message"`
	} `json:"error"`
	Item *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"item"`
}

func (codexStrategy) parse(stdout []byte) (delegationOutput, error) {
	var (
		out    delegationOutput
		cliErr string
	)
	for _, line := range strings.Split(string(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev codexEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			return delegationOutput{}, fmt.Errorf("parsing Codex output: %w", err)
		}
		switch {
		case ev.Type == "thread.started" && ev.ThreadID != "":
			out.sessionID = ev.ThreadID
		case ev.Type == "error" && ev.Message != "":
			cliErr = ev.Message
		case ev.Type == "turn.failed" && ev.Error != nil && ev.Error.Message != "":
			cliErr = ev.Error.Message
		case ev.Type == "item.completed" && ev.Item != nil && ev.Item.Type == "agent_message" && ev.Item.Text != "":
			// Later agent messages win: the last one is the answer to
			// the prompt.
			out.text = ev.Item.Text
		}
	}
	// Codex exits nonzero on some failures and zero on others (quota
	// limits), so the structured error events decide the outcome.
	if cliErr != "" {
		return delegationOutput{}, &delegationError{msg: cliErr}
	}
	if out.text == "" {
		return delegationOutput{}, errors.New("no agent message found in Codex output")
	}
	return out, nil
}

// NewCodexTool delegates a self-contained task to Codex running headlessly
// in the working directory, after the user approves the call.
func NewCodexTool(workingDir string, perms permissionRequester) fantasy.AgentTool {
	return newDelegationTool(workingDir, perms, newCodexStrategy())
}
