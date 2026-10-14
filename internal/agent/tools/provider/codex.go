package provider

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"charm.land/fantasy"
)

const CodexToolName = "codex"

//go:embed codex.md
var codexDescription string

// codexCommand is the Codex CLI binary. Tests point it at a fake script.
var codexCommand = "codex"

// codexStrategy adapts Codex's headless interface: a JSONL event stream
// whose error, turn.failed, and agent_message events carry the outcome.
type codexStrategy struct{}

func (codexStrategy) name() string     { return CodexToolName }
func (codexStrategy) describe() string { return codexDescription }
func (codexStrategy) binary() string   { return codexCommand }

// args assembles the headless invocation. The tool call itself is the
// permission gate; danger-full-access matches the claude_code trust model.
func (codexStrategy) args(prompt, resume string) []string {
	args := []string{"exec"}
	if resume != "" {
		args = append(args, "resume")
	}
	args = append(args,
		"--json",
		"-s", "danger-full-access",
		"--skip-git-repo-check",
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
// in the working directory.
func NewCodexTool(workingDir string) fantasy.AgentTool {
	return newDelegationTool(workingDir, codexStrategy{})
}
