package provider

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"charm.land/fantasy"
)

const ClaudeCodeToolName = "claude_code"

//go:embed claude_code.md
var claudeCodeDescription string

// claudeCommand is the Claude Code binary. Tests point it at a fake script.
var claudeCommand = "claude"

// claudeStrategy adapts Claude Code's headless interface: one JSON result
// object whose is_error flag and session_id carry the outcome.
type claudeStrategy struct{}

func (claudeStrategy) name() string     { return ClaudeCodeToolName }
func (claudeStrategy) describe() string { return claudeCodeDescription }
func (claudeStrategy) binary() string   { return claudeCommand }

// args assembles the headless invocation. The tool call itself is the
// permission gate: the user approves each delegation, so the nested agent
// must run without prompts it could not ask anyone to answer.
func (claudeStrategy) args(prompt, resume string) []string {
	args := []string{
		"-p", prompt,
		"--output-format", "json",
		"--dangerously-skip-permissions",
	}
	if resume != "" {
		args = append(args, "--resume", resume)
	}
	return args
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
// headlessly in the working directory.
func NewClaudeCodeTool(workingDir string) fantasy.AgentTool {
	return newDelegationTool(workingDir, claudeStrategy{})
}
